// Package ws provides a WebSocket hub for broadcasting real-time board events
// to connected browser clients.
package ws

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/ubenmackin/loom/internal/config"
	"github.com/ubenmackin/loom/internal/dispatcher"
)

// Event types that can be broadcast to clients.
const (
	EventTaskStatusChanged = "task_status_changed"
	EventTaskAssigned      = "task_assigned"
	EventTaskCreated       = "task_created"
	EventCommentAdded      = "comment_added"
	EventTaskStale         = "task_stale"
	EventBoardRefresh      = "board_refresh"
)

// BroadcastEvent is the JSON structure sent to all connected clients.
type BroadcastEvent struct {
	Type    string `json:"type"`
	TaskID  string `json:"task_id,omitempty"`
	StoryID string `json:"story_id,omitempty"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// Hub manages WebSocket connections and broadcasts events to all clients.
// It implements the dispatcher.EventBroadcaster interface.
type Hub struct {
	clients        map[*Client]bool
	broadcast      chan BroadcastEvent
	register       chan *Client
	unregister     chan *Client
	done           chan struct{}
	mu             sync.RWMutex
	upgrader       websocket.Upgrader
	allowedOrigins []string
	closed         atomic.Bool
	started        atomic.Bool
}

// Compile-time interface guards.
var _ dispatcher.EventBroadcaster = (*Hub)(nil)

// Client represents a single WebSocket connection.
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// Timing constants for WebSocket health checks.
const (
	writeWait      = 10 * time.Second
	pongWait       = 45 * time.Second
	pingPeriod     = 30 * time.Second
	maxMessageSize = 1024
)

// NewHub creates a new Hub with buffered channels for client management.
// Allowed WebSocket origins are read from the LOOM_ALLOWED_ORIGINS env var.
func NewHub() *Hub {
	allowedOrigins := config.GetAllowedOrigins()

	return &Hub{
		clients:        make(map[*Client]bool),
		broadcast:      make(chan BroadcastEvent, 256),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		done:           make(chan struct{}),
		allowedOrigins: allowedOrigins,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true // Non-browser clients (e.g., curl) are allowed.
				}
				return config.IsOriginAllowed(origin, allowedOrigins)
			},
		},
	}
}

// Start runs the hub event loop. It handles client registration,
// unregistration, and event broadcasting. This should be called as a goroutine.
// Safe to call multiple times — subsequent calls are no-ops.
func (h *Hub) Start() {
	if h.started.Swap(true) {
		return
	}
	if h.closed.Load() {
		return
	}
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			slog.Info("websocket client connected", "count", h.ClientCount())

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			slog.Info("websocket client disconnected", "count", h.ClientCount())

		case event, ok := <-h.broadcast:
			if !ok {
				// Channel closed — hub is shutting down.
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				slog.Error("failed to marshal broadcast event", "error", err)
				continue
			}
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- data:
				default:
					// Client send buffer full — skip to avoid blocking.
					slog.Warn("websocket client send buffer full, skipping")
				}
			}
			h.mu.RUnlock()

		case <-h.done:
			return
		}
	}
}

// ServeHTTP implements http.Handler for chi to mount. It upgrades the HTTP
// connection to WebSocket, creates a client, and starts read/write loops.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.closed.Load() {
		http.Error(w, "server shutting down", http.StatusServiceUnavailable)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "error", err)
		return
	}

	client := &Client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.register <- client

	// Start read and write loops.
	go client.writeLoop()
	go client.readLoop()
}

// Broadcast implements the dispatcher.EventBroadcaster interface.
// It sends an event to the broadcast channel for all connected clients.
// If the hub is stopping, the event is dropped to avoid panicking on a
// closed channel.
func (h *Hub) Broadcast(eventType string, payload any) {
	if h.closed.Load() {
		return
	}

	event := BroadcastEvent{
		Type: eventType,
		Data: payload,
	}

	// Extract common fields from payload if it's a map.
	if data, ok := payload.(map[string]string); ok {
		if taskID, ok := data["task_id"]; ok {
			event.TaskID = taskID
		}
		if storyID, ok := data["story_id"]; ok {
			event.StoryID = storyID
		}
		if from, ok := data["from"]; ok {
			event.From = from
		}
		if to, ok := data["to"]; ok {
			event.To = to
		}
	}

	select {
	case <-h.done:
		// Hub stopped while waiting to send — drop the event.
	case h.broadcast <- event:
	default:
		// Broadcast channel full — drop event to avoid blocking.
		slog.Warn("broadcast channel full, dropping event", "type", eventType)
	}
}

// ClientCount returns the number of currently connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Stop gracefully shuts down the hub by closing the done channel
// and sending a close frame (1001 Going Away) to all connected clients.
func (h *Hub) Stop() {
	h.mu.Lock()
	if h.closed.Load() {
		h.mu.Unlock()
		return
	}
	h.closed.Store(true)
	close(h.done)

	// Snapshot the client map before releasing the write lock
	// to avoid needing a read lock later (which would deadlock
	// with a concurrent ServeHTTP -> Start() -> register -> mu.Lock).
	clients := make([]*Client, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.Unlock()

	// Close all client connections outside the lock.
	for _, client := range clients {
		closeMsg := websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutting down")
		_ = client.conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(writeWait))
		_ = client.conn.Close()
	}

	// Close the unregister channel and drain any remaining entries
	// so that readLoop goroutines still trying to unregister don't
	// block forever.
	close(h.unregister)
	go func() {
		for range h.unregister {
			// Drain remaining unregister entries.
		}
	}()
}

// readLoop handles incoming WebSocket messages and connection health.
func (c *Client) readLoop() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Warn("websocket unexpected close", "error", err)
			}
			break
		}
		// Clients don't send meaningful data in v1; messages are ignored.
	}
}

// writeLoop handles outgoing WebSocket messages and periodic pings.
func (c *Client) writeLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Channel closed — hub unregistered us.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if _, err := w.Write(message); err != nil {
				slog.Error("websocket write error", "error", err)
				return
			}

			// Send any pending messages in the channel batch.
			n := len(c.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				_, _ = w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
