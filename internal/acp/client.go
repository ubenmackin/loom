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
	// receiveChanSize is the buffer size for the incoming message channel.
	receiveChanSize = 256
)

// Client is a JSON-RPC 2.0 client that communicates with an opencode ACP server
// by spawning it as a subprocess and communicating over stdin/stdout.
type Client struct {
	Command    string
	Args       []string
	ExtraFiles []*os.File // additional fds passed at fd 3, 4, … to the subprocess
	cmd        *exec.Cmd
	stdin      io.WriteCloser

	mu        sync.RWMutex
	writeMu   sync.Mutex
	connected bool
	receiveCh chan []byte
	wg        sync.WaitGroup
	reqID     atomic.Int64
}

// NewClient creates a new ACP subprocess client. Command is the subprocess to
// spawn (e.g., "opencode acp") and communicates via JSON-RPC 2.0 over
// stdin/stdout.
func NewClient(command string) *Client {
	return &Client{
		Command:   command,
		receiveCh: make(chan []byte, receiveChanSize),
	}
}

// Connect spawns the subprocess and starts the read and log goroutines.
// It returns an error if the command is empty, the client is already connected,
// the context is canceled before spawning, or the subprocess fails to start.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return fmt.Errorf("acp: already connected")
	}

	// Check context before proceeding.
	if err := ctx.Err(); err != nil {
		return err
	}

	parts := strings.Fields(c.Command)
	if len(parts) == 0 {
		return fmt.Errorf("acp: empty command")
	}
	parts = append(parts, c.Args...)

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.ExtraFiles = c.ExtraFiles

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("acp: stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("acp: stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("acp: stderr pipe: %w", err)
	}

	// Check context before starting the subprocess.
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("acp: start: %w", err)
	}

	c.cmd = cmd
	c.stdin = stdin
	c.connected = true

	slog.Info("acp: subprocess started", "command", c.Command, "pid", cmd.Process.Pid)

	c.wg.Add(1)
	go c.readPump(stdout)

	c.wg.Add(1)
	go c.logStderr(stderr)

	return nil
}

// readPump reads JSON-RPC 2.0 lines from the subprocess's stdout.
// Each line is parsed as a JSONRPCResponse and the relevant payload
// (Result, Params for notifications, or marshaled Error) is pushed
// onto the receive channel. When stdout is closed (subprocess exits),
// the channel is closed and connected is set to false.
func (c *Client) readPump(stdout io.Reader) {
	defer c.wg.Done()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Bytes()

		var resp JSONRPCResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			slog.Warn("acp: failed to unmarshal JSON-RPC response", "error", err)
			continue
		}

		var payload []byte
		switch {
		case resp.Result != nil:
			payload = resp.Result
		case resp.Params != nil:
			// Notification — push params directly.
			payload = resp.Params
		case resp.Error != nil:
			// Error response — wrap in ACPResponse.
			errResp := ACPResponse{Success: false, Error: resp.Error.Message}
			data, marshalErr := json.Marshal(errResp)
			if marshalErr != nil {
				slog.Warn("acp: failed to marshal error response", "error", marshalErr)
				continue
			}
			payload = data
		default:
			// Neither result, params, nor error — nothing to forward.
			continue
		}

		select {
		case c.receiveCh <- payload:
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

	close(c.receiveCh)

	slog.Info("acp: subprocess stdout closed")
}

// logStderr reads stderr from the subprocess line by line and logs each
// line via slog.Warn. This prevents stderr from filling up and blocking
// the subprocess.
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

// methodName derives the JSON-RPC method name from the message type.
// For SessionMessage it returns "acp." + v.Type.
// For TaskMessage it returns "acp." + v.Type.
func methodName(msg interface{}) (string, error) {
	switch v := msg.(type) {
	case SessionMessage:
		return "acp." + v.Type, nil
	case TaskMessage:
		return "acp." + v.Type, nil
	default:
		return "", fmt.Errorf("acp: unknown message type %T", msg)
	}
}

// Send JSON-encodes the provided value as a JSON-RPC 2.0 request and
// writes it to the subprocess's stdin. The method name is derived from
// the message type. An auto-incrementing request ID is assigned. If the
// client is not connected an error is returned.
func (c *Client) Send(msg interface{}) error {
	method, err := methodName(msg)
	if err != nil {
		return err
	}

	id := c.reqID.Add(1)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  msg,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("acp: marshal: %w", err)
	}

	data = append(data, '\n')

	c.mu.RLock()
	if !c.connected || c.stdin == nil {
		c.mu.RUnlock()
		return fmt.Errorf("acp: not connected")
	}
	stdin := c.stdin
	c.mu.RUnlock()

	c.writeMu.Lock()
	if _, err := stdin.Write(data); err != nil {
		c.writeMu.Unlock()
		return fmt.Errorf("acp: write: %w", err)
	}
	c.writeMu.Unlock()

	return nil
}

// Receive returns the channel that delivers raw JSON messages received
// from the subprocess. The channel is closed when the subprocess exits
// or the client is closed. If the client is not connected an error is
// returned.
func (c *Client) Receive() (<-chan []byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected {
		return nil, fmt.Errorf("acp: not connected")
	}

	return c.receiveCh, nil
}

// IsConnected returns whether the client is currently connected to a
// running subprocess.
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Close cleanly shuts down the subprocess. It closes stdin to signal
// EOF, waits up to 5 seconds for the subprocess to exit on its own,
// and kills it if it does not. It then waits for all background
// goroutines to finish.
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

	// Wait for readPump and logStderr to finish.
	c.wg.Wait()

	slog.Info("acp: subprocess closed", "command", c.Command)
	return nil
}
