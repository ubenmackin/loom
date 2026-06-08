package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// readLimit sets the maximum message size from the server (64KB).
	readLimit = 64 * 1024

	// pongWait is how long to wait for a pong response before considering
	// the connection dead.
	pongWait = 45 * time.Second

	// writeWait is how long to wait for a write to complete.
	writeWait = 10 * time.Second

	// receiveChanSize is the buffer size for the incoming message channel.
	receiveChanSize = 256
)

// Client is a WebSocket client for the opencode ACP protocol.
type Client struct {
	URL       string
	mu        sync.RWMutex
	conn      *websocket.Conn
	done      chan struct{}
	connected bool

	// receiveCh is the channel that delivers raw JSON messages.
	receiveCh chan []byte

	// wg tracks the read pump goroutine.
	wg sync.WaitGroup
}

// NewClient creates a new ACP WebSocket client for the given endpoint URL.
func NewClient(url string) *Client {
	return &Client{
		URL:       url,
		done:      make(chan struct{}),
		receiveCh: make(chan []byte, receiveChanSize),
	}
}

// Connect dials the WebSocket endpoint and starts the read pump in a
// background goroutine. It sets up ping/pong handling with appropriate
// deadlines and configures a 64KB read limit.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return fmt.Errorf("acp: already connected")
	}

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.URL, nil)
	if err != nil {
		return fmt.Errorf("acp: dial %s: %w", c.URL, err)
	}

	c.conn = conn
	c.done = make(chan struct{})
	c.connected = true

	slog.Info("acp connected", "url", c.URL)

	// Start the read pump.
	c.wg.Add(1)
	go c.readPump()

	return nil
}

// readPump reads messages from the WebSocket connection in a loop and
// pushes raw JSON payloads onto the receive channel. It handles
// ping/pong with deadline enforcement.
func (c *Client) readPump() {
	defer c.wg.Done()

	c.conn.SetReadLimit(readLimit)

	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		slog.Warn("acp: setting initial read deadline", "error", err)
	}

	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Warn("acp: unexpected close", "error", err)
			}
			// Signal that the read pump has exited.
			close(c.receiveCh)
			// Mark disconnected.
			c.mu.Lock()
			c.connected = false
			if c.conn != nil {
				_ = c.conn.Close()
				c.conn = nil
			}
			c.mu.Unlock()
			slog.Info("acp disconnected", "url", c.URL, "error", err)
			return
		}

		// Make a copy of the message so the caller can't mutate the
		// internal buffer.
		msg := make([]byte, len(message))
		copy(msg, message)

		select {
		case c.receiveCh <- msg:
		default:
			slog.Warn("acp: receive channel full, dropping message")
		}
	}
}

// Send JSON-encodes the provided value and writes it as a text message
// to the server.
func (c *Client) Send(msg interface{}) error {
	c.mu.RLock()
	conn := c.conn
	connected := c.connected
	c.mu.RUnlock()

	if !connected || conn == nil {
		return fmt.Errorf("acp: not connected")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("acp: marshal: %w", err)
	}

	if err := conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return fmt.Errorf("acp: set write deadline: %w", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("acp: write: %w", err)
	}

	return nil
}

// Receive returns a channel that delivers raw JSON messages received
// from the server. The channel is closed when the connection is lost or
// the client is closed. Each call returns the same channel.
func (c *Client) Receive() (<-chan []byte, error) {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()

	if !connected {
		return nil, fmt.Errorf("acp: not connected")
	}

	return c.receiveCh, nil
}

// IsConnected returns whether the client is currently connected.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Close cleanly closes the WebSocket connection. It sends a close frame
// to the server and waits for the read pump to finish.
func (c *Client) Close() error {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return fmt.Errorf("acp: not connected")
	}

	c.connected = false

	// Signal the read pump to stop (already handles this on disconnect).
	close(c.done)

	conn := c.conn
	c.conn = nil
	c.mu.Unlock()

	// Send a close frame.
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "client shutting down")
	_ = conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(writeWait))
	_ = conn.Close()

	// Wait for the read pump to finish.
	c.wg.Wait()

	slog.Info("acp: closed", "url", c.URL)
	return nil
}
