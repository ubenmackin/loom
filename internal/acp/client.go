package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	receiveChanSize = 256
)

// Client is a JSON-RPC 2.0 client that communicates with an opencode ACP server
// by spawning it as a subprocess and communicating over stdin/stdout.
type Client struct {
	Command    string
	Args       []string
	ExtraFiles []*os.File
	cmd        *exec.Cmd
	stdin      io.WriteCloser

	mu        sync.RWMutex
	writeMu   sync.Mutex
	connected bool
	receiveCh chan []byte
	wg        sync.WaitGroup
	reqID     atomic.Int64

	pending   map[int64]chan []byte
	pendingMu sync.Mutex

	initialized bool
}

// NewClient creates a new ACP subprocess client.
func NewClient(command string) *Client {
	return &Client{
		Command:   command,
		receiveCh: make(chan []byte, receiveChanSize),
		pending:   make(map[int64]chan []byte),
	}
}

// Connect spawns the subprocess and starts the read and log goroutines.
// After the subprocess starts, it auto-runs Initialize().
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		return fmt.Errorf("acp: already connected")
	}

	if err := ctx.Err(); err != nil {
		c.mu.Unlock()
		return err
	}

	parts := strings.Fields(c.Command)
	if len(parts) == 0 {
		c.mu.Unlock()
		return fmt.Errorf("acp: empty command")
	}
	parts = append(parts, c.Args...)

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.ExtraFiles = c.ExtraFiles

	stdin, err := cmd.StdinPipe()
	if err != nil {
		c.mu.Unlock()
		return fmt.Errorf("acp: stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.mu.Unlock()
		return fmt.Errorf("acp: stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		c.mu.Unlock()
		return fmt.Errorf("acp: stderr pipe: %w", err)
	}

	if err := ctx.Err(); err != nil {
		c.mu.Unlock()
		return err
	}

	if err := cmd.Start(); err != nil {
		c.mu.Unlock()
		return fmt.Errorf("acp: start: %w", err)
	}

	c.cmd = cmd
	c.stdin = stdin
	c.connected = true
	c.mu.Unlock()

	slog.Info("acp: subprocess started", "command", c.Command, "pid", cmd.Process.Pid)

	c.wg.Add(1)
	go c.readPump(stdout)

	c.wg.Add(1)
	go c.logStderr(stderr)

	// Auto-initialize after subprocess start.
	initCtx, initCancel := context.WithTimeout(ctx, 10*time.Second)
	defer initCancel()
	initInfo := &ClientInfo{Name: "loom", Version: "1.0.0"}
	if _, err := c.Initialize(initCtx, initInfo); err != nil {
		slog.Warn("acp: auto-initialize failed, continuing", "error", err)
	}

	return nil
}

// readPump reads JSON-RPC 2.0 lines from stdout and dispatches responses
// by request ID to the matching pending channel. Notifications and
// unmatched responses fall through to the receive channel.
func (c *Client) readPump(stdout io.Reader) {
	defer c.wg.Done()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		raw := make([]byte, len(line))
		copy(raw, line)

		var resp JSONRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			slog.Warn("acp: failed to unmarshal JSON-RPC response", "error", err)
			continue
		}

		if resp.ID != nil {
			c.pendingMu.Lock()
			ch, ok := c.pending[*resp.ID]
			c.pendingMu.Unlock()
			if ok {
				select {
				case ch <- raw:
				default:
					slog.Warn("acp: pending channel full, dropping response", "id", *resp.ID)
				}
				continue
			}
		}

		// No matching pending request — treat as notification and push to receive channel.
		select {
		case c.receiveCh <- raw:
		default:
			slog.Warn("acp: receive channel full, dropping message")
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Warn("acp: read pump scanner error", "error", err)
	}

	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()

	// Drain pending map so any waiting goroutines unblock.
	c.pendingMu.Lock()
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()

	close(c.receiveCh)
	slog.Info("acp: subprocess stdout closed")
}

// logStderr reads stderr from the subprocess line by line and logs each line.
func (c *Client) logStderr(stderr io.Reader) {
	defer c.wg.Done()

	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	for scanner.Scan() {
		slog.Warn("acp: subprocess stderr", "line", scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		slog.Warn("acp: stderr scanner error", "error", err)
	}
}

// sendRequest sends a JSON-RPC request with the given method and params,
// waits for the matching response, and returns the raw result bytes.
func (c *Client) sendRequest(ctx context.Context, method string, params any) ([]byte, error) {
	if method != "initialize" {
		c.mu.RLock()
		init := c.initialized
		c.mu.RUnlock()
		if !init {
			return nil, fmt.Errorf("acp: client not initialized, call Initialize first")
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	ch := make(chan []byte, 1)
	id := c.reqID.Add(1)

	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("acp: marshal request: %w", err)
	}
	data = append(data, '\n')

	c.mu.RLock()
	if !c.connected || c.stdin == nil {
		c.mu.RUnlock()
		return nil, fmt.Errorf("acp: not connected")
	}
	stdin := c.stdin
	c.mu.RUnlock()

	c.writeMu.Lock()
	_, err = stdin.Write(data)
	c.writeMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("acp: write request: %w", err)
	}

	select {
	case raw := <-ch:
		var resp JSONRPCResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, fmt.Errorf("acp: unmarshal response: %w", err)
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("acp: %s (code %d)", resp.Error.Message, resp.Error.Code)
		}
		return resp.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Initialize sends an initialize request and negotiates capabilities.
// It sets the initialized flag on success.
func (c *Client) Initialize(ctx context.Context, info *ClientInfo) (*InitializeResponse, error) {
	req := InitializeRequest{
		ProtocolVersion: 1,
		ClientInfo:      info,
	}
	raw, err := c.sendRequest(ctx, "initialize", req)
	if err != nil {
		return nil, err
	}
	var resp InitializeResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("acp: unmarshal initialize response: %w", err)
	}
	c.mu.Lock()
	c.initialized = true
	c.mu.Unlock()
	return &resp, nil
}

// NewSession sends a session/new request and returns the created session ID.
func (c *Client) NewSession(ctx context.Context, req NewSessionRequest) (string, error) {
	raw, err := c.sendRequest(ctx, "session/new", req)
	if err != nil {
		return "", err
	}
	var resp NewSessionResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("acp: unmarshal new session response: %w", err)
	}
	return resp.SessionID, nil
}

// SendPrompt sends a prompt to an existing ACP session and returns the stop reason.
func (c *Client) SendPrompt(ctx context.Context, sessionID string, promptText string) (string, error) {
	req := PromptRequest{
		SessionID: sessionID,
		Prompt: []ContentBlock{
			{Type: "text", Text: promptText},
		},
	}
	raw, err := c.sendRequest(ctx, "session/prompt", req)
	if err != nil {
		return "", err
	}
	var resp PromptResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("acp: unmarshal prompt response: %w", err)
	}
	return resp.StopReason, nil
}

// Receive returns the channel that delivers raw JSON messages received
// from the subprocess (notifications and unmatched responses).
func (c *Client) Receive() (<-chan []byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.connected {
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

// Close shuts down the subprocess and cleans up pending requests.
func (c *Client) Close() error {
	c.mu.Lock()
	if !c.connected {
		c.mu.Unlock()
		return fmt.Errorf("acp: not connected")
	}
	c.connected = false
	stdin := c.stdin
	cmd := c.cmd
	c.mu.Unlock()

	// Close stdin to signal EOF to the subprocess.
	if stdin != nil {
		if err := stdin.Close(); err != nil {
			slog.Warn("acp: close stdin", "error", err)
		}
	}

	// Wait for the subprocess to exit with a 5-second timeout.
	waitDone := make(chan struct{})
	go func() {
		if cmd != nil {
			_ = cmd.Wait()
		}
		close(waitDone)
	}()

	select {
	case <-waitDone:
	case <-time.After(5 * time.Second):
		if cmd != nil && cmd.Process != nil {
			slog.Warn("acp: subprocess did not exit in time, killing", "pid", cmd.Process.Pid)
			if err := cmd.Process.Kill(); err != nil {
				slog.Warn("acp: kill subprocess", "error", err)
			}
			<-waitDone
		}
	}

	// Close all pending channels with a "client closed" error so that
	// any goroutines waiting in sendRequest unblock.
	c.pendingMu.Lock()
	for id, ch := range c.pending {
		select {
		case ch <- nil:
		default:
		}
		close(ch)
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()

	// Wait for readPump and logStderr to finish.
	c.wg.Wait()

	slog.Info("acp: subprocess closed", "command", c.Command)
	return nil
}
