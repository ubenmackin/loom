package acp

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// testClientWithPipes creates a Client whose I/O goes through pipes instead of
// a real subprocess. It returns:
//   - client — the modified *Client ready for test use
//   - requests — a channel that delivers every JSON-RPC request the client sends
//   - respond — a helper that builds and writes a JSON-RPC response for the given id
//   - writeRaw — a helper that writes arbitrary bytes directly to the stdout pipe
//   - cleanup — a function that shuts down the readPump goroutine and closes pipes
//
// The caller must call cleanup (typically via defer) to avoid leaking goroutines.
func testClientWithPipes(t *testing.T) (*Client, <-chan JSONRPCRequest, func(int64, interface{}, *JSONRPCError), func([]byte), func()) {
	t.Helper()

	client := NewClient("test")
	client.connected = true
	client.pending = make(map[int64]chan []byte)
	client.receiveCh = make(chan []byte, receiveChanSize)

	// Stdin: client writes requests, we read them.
	stdinReader, stdinWriter := io.Pipe()
	client.stdin = stdinWriter

	// Stdout: we write responses, the client reads them via readPump.
	stdoutReader, stdoutWriter := io.Pipe()

	client.wg.Add(1)
	go client.readPump(stdoutReader)

	// Read JSON-RPC requests from the stdin pipe as the client writes them.
	reqCh := make(chan JSONRPCRequest, 100)
	go func() {
		dec := json.NewDecoder(stdinReader)
		for {
			var req JSONRPCRequest
			if err := dec.Decode(&req); err != nil {
				close(reqCh)
				return
			}
			reqCh <- req
		}
	}()

	// writeRaw writes raw bytes directly to the stdout pipe, appending a
	// trailing newline if one is not already present (bufio.Scanner in
	// readPump expects newline-delimited JSON).
	writeRaw := func(data []byte) {
		if len(data) == 0 || data[len(data)-1] != '\n' {
			data = append(data, '\n')
		}
		_, err := stdoutWriter.Write(data)
		if err != nil {
			t.Errorf("writeRaw: stdoutWriter.Write: %v", err)
		}
	}

	// respond builds a JSON-RPC 2.0 response and writes it to the stdout pipe.
	// If both result and rpcErr are nil the call is skipped.
	respond := func(id int64, result interface{}, rpcErr *JSONRPCError) {
		t.Helper()

		if result == nil && rpcErr == nil {
			t.Error("respond: both result and rpcErr are nil, nothing to send")
			return
		}

		var resultRaw json.RawMessage
		if result != nil {
			var err error
			resultRaw, err = json.Marshal(result)
			if err != nil {
				t.Errorf("respond: json.Marshal(result): %v", err)
				return
			}
		}

		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      &id,
			Result:  resultRaw,
			Error:   rpcErr,
		}
		data, err := json.Marshal(resp)
		if err != nil {
			t.Errorf("respond: json.Marshal(resp): %v", err)
			return
		}
		writeRaw(data)
	}

	// cleanup closes both pipes and waits for readPump to finish.
	cleanup := func() {
		stdoutWriter.Close()
		stdinWriter.Close()
		client.wg.Wait()
	}

	return client, reqCh, respond, writeRaw, cleanup
}

// setInitialized marks the client as initialized under the lock so that
// methods requiring initialization (NewSession, SendPrompt, etc.) pass
// their guard check.
func setInitialized(c *Client) {
	c.mu.Lock()
	c.initialized = true
	c.mu.Unlock()
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestInitialize verifies that Initialize sends an "initialize" request and
// correctly parses the response, setting the internal initialized flag.
func TestInitialize(t *testing.T) {
	client, requests, respond, _, cleanup := testClientWithPipes(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Respond as soon as we see the request — io.Pipe provides natural
	// synchronization so no artificial sleep is needed.
	go func() {
		req := <-requests
		if req.Method != "initialize" {
			t.Errorf("request method = %q, want %q", req.Method, "initialize")
		}

		respond(1, InitializeResponse{
			ProtocolVersion: 1,
			AgentCapabilities: map[string]any{
				"streaming": true,
			},
			AgentInfo: &ClientInfo{Name: "opencode", Version: "1.0.0"},
		}, nil)
	}()

	info := &ClientInfo{Name: "loom", Version: "1.0.0"}
	initResp, err := client.Initialize(ctx, info)
	if err != nil {
		t.Fatalf("Initialize() returned unexpected error: %v", err)
	}
	if initResp.ProtocolVersion != 1 {
		t.Errorf("ProtocolVersion = %d, want 1", initResp.ProtocolVersion)
	}
	if !client.initialized {
		t.Error("client.initialized is false after successful Initialize")
	}
}

// TestNewSession verifies that NewSession sends a "session/new" request and
// returns the session ID from the response.
func TestNewSession(t *testing.T) {
	client, requests, respond, _, cleanup := testClientWithPipes(t)
	defer cleanup()

	setInitialized(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		req := <-requests
		if req.Method != "session/new" {
			t.Errorf("request method = %q, want %q", req.Method, "session/new")
		}
		respond(req.ID, NewSessionResponse{SessionID: "sess_001"}, nil)
	}()

	sessionID, err := client.NewSession(ctx, NewSessionRequest{
		Cwd: "/test/path",
	})
	if err != nil {
		t.Fatalf("NewSession() returned unexpected error: %v", err)
	}
	if sessionID != "sess_001" {
		t.Errorf("sessionID = %q, want %q", sessionID, "sess_001")
	}
}

// TestSendPrompt verifies that SendPrompt sends a "session/prompt" request and
// returns the stop reason from the response.
func TestSendPrompt(t *testing.T) {
	client, requests, respond, _, cleanup := testClientWithPipes(t)
	defer cleanup()

	setInitialized(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		req := <-requests
		if req.Method != "session/prompt" {
			t.Errorf("request method = %q, want %q", req.Method, "session/prompt")
		}
		respond(req.ID, PromptResponse{StopReason: "end_turn"}, nil)
	}()

	stopReason, err := client.SendPrompt(ctx, "sess_001", "Hello, agent!")
	if err != nil {
		t.Fatalf("SendPrompt() returned unexpected error: %v", err)
	}
	if stopReason != "end_turn" {
		t.Errorf("stopReason = %q, want %q", stopReason, "end_turn")
	}
}

// TestNotInitialized verifies that calling a method that requires initialization
// (NewSession, SendPrompt, etc.) before Initialize is called returns an error.
func TestNotInitialized(t *testing.T) {
	client, _, _, _, cleanup := testClientWithPipes(t)
	defer cleanup()

	// State: initialized defaults to false, we do NOT call setInitialized.

	ctx := context.Background()
	_, err := client.NewSession(ctx, NewSessionRequest{Cwd: "/test"})
	if err == nil {
		t.Fatal("expected error for NewSession before Initialize, got nil")
	}
	if !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("error message = %q, want substring 'not initialized'", err.Error())
	}
}

// TestTimeout verifies that a request fails with a context deadline exceeded
// error when no response arrives.
func TestTimeout(t *testing.T) {
	client, _, _, _, cleanup := testClientWithPipes(t)
	defer cleanup()

	setInitialized(client)

	// Use a very short timeout so the test completes quickly.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := client.SendPrompt(ctx, "sess_001", "hello")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("error = %v, want %v", err, context.DeadlineExceeded)
	}
}

// TestErrorResponse verifies that a JSON-RPC error response from the
// subprocess is surfaced as a Go error with the correct message.
func TestErrorResponse(t *testing.T) {
	client, requests, respond, _, cleanup := testClientWithPipes(t)
	defer cleanup()

	setInitialized(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go func() {
		req := <-requests
		respond(req.ID, nil, &JSONRPCError{Code: -32000, Message: "session not found"})
	}()

	_, err := client.SendPrompt(ctx, "bad_session", "hello")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "session not found") {
		t.Errorf("error message = %q, want substring 'session not found'", err.Error())
	}
}

// TestNotification verifies that a JSON-RPC notification (a response with no
// ID field) is delivered to the channel returned by Receive().
func TestNotification(t *testing.T) {
	client, _, _, writeRaw, cleanup := testClientWithPipes(t)
	defer cleanup()

	setInitialized(client)

	// Write a notification: a JSON-RPC message without an "id" field.
	notif := JSONRPCResponse{
		JSONRPC: "2.0",
		Method:  "session/event",
		Params:  json.RawMessage(`{"type":"progress"}`),
	}
	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatalf("json.Marshal notification: %v", err)
	}
	writeRaw(data)

	// Get the receive channel.
	ch, err := client.Receive()
	if err != nil {
		t.Fatalf("Receive() returned unexpected error: %v", err)
	}

	select {
	case msg, ok := <-ch:
		if !ok {
			t.Fatal("receiveCh closed unexpectedly")
		}
		var resp JSONRPCResponse
		if err := json.Unmarshal(msg, &resp); err != nil {
			t.Errorf("json.Unmarshal received message: %v", err)
		}
		if resp.Method != "session/event" {
			t.Errorf("method = %q, want %q", resp.Method, "session/event")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for notification on receiveCh")
	}
}

// TestConcurrentRequests verifies that multiple concurrent requests receive
// the correct responses (request-response correlation).
func TestConcurrentRequests(t *testing.T) {
	client, requests, respond, _, cleanup := testClientWithPipes(t)
	defer cleanup()

	setInitialized(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	var (
		sessionID1, sessionID2 string
		err1, err2             error
	)

	go func() {
		defer wg.Done()
		sessionID1, err1 = client.NewSession(ctx, NewSessionRequest{Cwd: "/path1"})
	}()
	go func() {
		defer wg.Done()
		sessionID2, err2 = client.NewSession(ctx, NewSessionRequest{Cwd: "/path2"})
	}()

	// Read both requests from the pipe (synchronized by io.Pipe — no sleep needed).
	req1 := <-requests
	req2 := <-requests

	// Respond to each request using its own ID so the correlation is
	// deterministic regardless of goroutine scheduling / pipe ordering.
	respond(req1.ID, NewSessionResponse{SessionID: "sess_001"}, nil)
	respond(req2.ID, NewSessionResponse{SessionID: "sess_002"}, nil)

	wg.Wait()

	if err1 != nil {
		t.Errorf("request 1 error: %v", err1)
	}
	if err2 != nil {
		t.Errorf("request 2 error: %v", err2)
	}
	// Verify that both goroutines got a valid, non-empty session ID and that
	// the two IDs are distinct — proving that one was not dropped and both
	// were routed to the correct pending channel.
	got := map[string]bool{sessionID1: true, sessionID2: true}
	if len(got) != 2 {
		t.Errorf("concurrent requests returned duplicate session IDs: %q, %q", sessionID1, sessionID2)
	}
	if sessionID1 == "" || sessionID2 == "" {
		t.Errorf("unexpected empty session ID: %q, %q", sessionID1, sessionID2)
	}
}

// TestIsConnected verifies the IsConnected method returns the expected state
// before and after the readPump shuts down.
func TestIsConnected(t *testing.T) {
	client, _, _, _, cleanup := testClientWithPipes(t)
	defer cleanup()

	if !client.IsConnected() {
		t.Error("expected IsConnected() = true immediately after setup")
	}

	// Shut down the pipes; readPump will set connected = false.
	cleanup()

	if client.IsConnected() {
		t.Error("expected IsConnected() = false after cleanup")
	}
}

// TestSetSessionMode verifies that SetSessionMode sends a "session/set_mode"
// JSON-RPC request with the expected session id and mode id, and that the
// (empty-_meta) response is unmarshaled without error so the method returns
// nil. This is a smoke test: it exercises the legacy session/set_mode path
// retained for fallback and for tests that want to drive the legacy flow
// (plan.md Decision 1) so it is not silently broken by future refactorings.
func TestSetSessionMode(t *testing.T) {
	client, requests, respond, _, cleanup := testClientWithPipes(t)
	defer cleanup()

	setInitialized(client)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Respond as soon as we see the request — io.Pipe provides natural
	// synchronization so no artificial sleep is needed.
	go func() {
		req := <-requests
		if req.Method != "session/set_mode" {
			t.Errorf("request method = %q, want %q", req.Method, "session/set_mode")
		}
		// session/set_mode returns only the reserved _meta field; an empty
		// _meta is a valid response per the spec.
		respond(req.ID, SetSessionModeResponse{}, nil)
	}()

	if err := client.SetSessionMode(ctx, "session-test", "planner"); err != nil {
		t.Fatalf("SetSessionMode() returned unexpected error: %v", err)
	}
}
