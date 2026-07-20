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

	// Split command into executable name and any inline arguments.
	// Build final argument list from any inline args plus explicit Args slice.
	cmdParts := strings.Fields(c.Command)
	if len(cmdParts) == 0 {
		c.mu.Unlock()
		return fmt.Errorf("acp: empty command")
	}
	cmdName := cmdParts[0]
	var finalArgs []string
	finalArgs = append(finalArgs, cmdParts[1:]...)
	finalArgs = append(finalArgs, c.Args...)

	cmd := exec.Command(cmdName, finalArgs...)
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
//
// It is a thin wrapper around NewSessionWithModes that preserves the
// historical (string-returning) signature for existing callers; new callers
// that need the available modes/config options should use
// NewSessionWithModes directly.
func (c *Client) NewSession(ctx context.Context, req NewSessionRequest) (string, error) {
	resp, err := c.NewSessionWithModes(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.SessionID, nil
}

// NewSessionWithModes sends a session/new request and returns the full
// NewSessionResponse, including the optional Modes and ConfigOptions that
// the gateway uses to drive session-mode routing (TASK-004).
func (c *Client) NewSessionWithModes(ctx context.Context, req NewSessionRequest) (*NewSessionResponse, error) {
	raw, err := c.sendRequest(ctx, "session/new", req)
	if err != nil {
		return nil, err
	}
	var resp NewSessionResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("acp: unmarshal new session response: %w", err)
	}
	return &resp, nil
}

// ExtractAvailableModes returns the list of selectable mode IDs advertised by
// the server in a session/new response. The preference order mirrors the
// ACP v1 evolution:
//
//  1. If ConfigOptions is non-empty, find the option whose Category == "mode"
//     (falling back to ID == "mode" when Category is not populated) and return
//     its Options[].Value. This is the modern session/set_config_option surface.
//  2. Else if Modes is non-nil and advertises AvailableModes, return each
//     AvailableModes[].ID. This is the legacy session/set_mode surface.
//  3. Else return nil (the server did not advertise any modes).
func (c *Client) ExtractAvailableModes(resp *NewSessionResponse) []string {
	return extractAvailableModes(resp)
}

// extractAvailableModes is the package-level form of the helper; it has no
// receiver state and is the canonical implementation used by the bound method
// above.
func extractAvailableModes(resp *NewSessionResponse) []string {
	if resp == nil {
		return nil
	}

	// 1. Prefer the modern ConfigOptions surface.
	if len(resp.ConfigOptions) > 0 {
		for _, opt := range resp.ConfigOptions {
			if opt.Category == "mode" || (opt.Category == "" && opt.ID == "mode") {
				if len(opt.Options) == 0 {
					return nil
				}
				modes := make([]string, 0, len(opt.Options))
				for _, o := range opt.Options {
					modes = append(modes, o.Value)
				}
				return modes
			}
		}
	}

	// 2. Fall back to the legacy Modes surface.
	if resp.Modes != nil && len(resp.Modes.AvailableModes) > 0 {
		modes := make([]string, 0, len(resp.Modes.AvailableModes))
		for _, m := range resp.Modes.AvailableModes {
			modes = append(modes, m.ID)
		}
		return modes
	}

	// 3. Nothing advertised.
	return nil
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

// SetSessionMode sends a session/set_mode request to change the currently
// active mode for an existing ACP session. The server returns only the
// reserved `_meta` field (no useful payload); on success the error is nil.
func (c *Client) SetSessionMode(ctx context.Context, sessionID, modeID string) error {
	req := SetSessionModeRequest{
		SessionID: sessionID,
		ModeID:    modeID,
	}
	raw, err := c.sendRequest(ctx, "session/set_mode", req)
	if err != nil {
		return err
	}
	var resp SetSessionModeResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("acp: unmarshal set mode response: %w", err)
	}
	return nil
}

// SetSessionConfigOption sends a session/set_config_option request to change
// the current value of a session configuration option. For v1 the `value`
// argument is a string (Loom only uses the select-type mode selector). The
// response carries the full refreshed config-options list, which is returned
// to the caller.
func (c *Client) SetSessionConfigOption(ctx context.Context, sessionID, configID, value string) (*SetSessionConfigOptionResponse, error) {
	req := SetSessionConfigOptionRequest{
		SessionID: sessionID,
		ConfigID:  configID,
		Value:     value,
	}
	raw, err := c.sendRequest(ctx, "session/set_config_option", req)
	if err != nil {
		return nil, err
	}
	var resp SetSessionConfigOptionResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("acp: unmarshal set config option response: %w", err)
	}
	return &resp, nil
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
