// Package mcp implements a Model Context Protocol server for Loom,
// exposing agent operations as MCP tools over stdio JSON-RPC 2.0.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/ubenmackin/loom/internal/dispatcher"
	"github.com/ubenmackin/loom/internal/store"
)

// Request represents a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

// Error represents a JSON-RPC 2.0 error object.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ToolResult represents the result of an MCP tool call.
type ToolResult struct {
	Content []ContentBlock `json:"content"`
}

// ContentBlock represents a single content block in an MCP tool result.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ToolHandler is the function signature for MCP tool handlers.
type ToolHandler func(ctx context.Context, params map[string]any) (*ToolResult, error)

// ToolDef describes an MCP tool for the tools/list response.
type ToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

// Server implements an MCP server that communicates over stdio using
// JSON-RPC 2.0.
type Server struct {
	stories    *store.StoryStore
	tasks      *store.TaskStore
	sessions   *store.SessionStore
	comments   *store.CommentStore
	templates  *store.TemplateStore
	activities *store.ActivityStore
	dispatcher *dispatcher.Dispatcher

	encoder   *json.Encoder
	decoder   *json.Decoder
	tools     map[string]ToolHandler
	toolDefs  []ToolDef
	sessionID string // auto-registered on first tool call if not provided
}

// NewServer creates a new MCP server with the given stores and dispatcher,
// and registers all 14 MCP tools.
func NewServer(
	stories *store.StoryStore,
	tasks *store.TaskStore,
	sessions *store.SessionStore,
	comments *store.CommentStore,
	templates *store.TemplateStore,
	activities *store.ActivityStore,
	disp *dispatcher.Dispatcher,
) *Server {
	s := &Server{
		stories:    stories,
		tasks:      tasks,
		sessions:   sessions,
		comments:   comments,
		templates:  templates,
		activities: activities,
		dispatcher: disp,
		tools:      make(map[string]ToolHandler),
	}
	s.registerTools()
	return s
}

// Run starts the MCP server main loop. It reads JSON-RPC requests from
// stdin and writes responses to stdout.
func (s *Server) Run(ctx context.Context) error {
	return s.RunWith(ctx, os.Stdin, os.Stdout)
}

// RunWith is the testable version of Run that accepts explicit io.Reader
// and io.Writer for input and output streams.
func (s *Server) RunWith(ctx context.Context, in io.Reader, out io.Writer) error {
	s.encoder = json.NewEncoder(out)
	s.decoder = json.NewDecoder(in)

	slog.Info("MCP server starting, reading from stdin")

	for {
		select {
		case <-ctx.Done():
			slog.Info("MCP server shutting down", "reason", ctx.Err())
			return nil
		default:
		}

		var req Request
		if err := s.decoder.Decode(&req); err != nil {
			if err == io.EOF {
				slog.Info("MCP server: stdin closed")
				return nil
			}
			slog.Error("MCP server: decode error", "error", err)
			s.writeResponse(Response{
				JSONRPC: "2.0",
				ID:      nil,
				Error:   &Error{Code: -32700, Message: "Parse error"},
			})
			continue
		}

		s.handleRequest(ctx, req)
	}
}

// handleRequest routes a JSON-RPC request to the appropriate handler.
func (s *Server) handleRequest(ctx context.Context, req Request) {
	slog.Debug("MCP request", "method", req.Method, "id", req.ID)

	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "notifications/initialized":
		// Client acknowledges initialization — no response needed for notifications.
		slog.Debug("MCP client initialized")
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolsCall(ctx, req)
	default:
		s.writeResponse(Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)},
		})
	}
}

// handleInitialize handles the MCP initialize handshake.
func (s *Server) handleInitialize(req Request) {
	result := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "loom-mcp",
			"version": "0.1.0",
		},
	}
	s.writeResponse(Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	})
}

// handleToolsList returns the list of available MCP tools.
func (s *Server) handleToolsList(req Request) {
	s.writeResponse(Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"tools": s.toolDefs,
		},
	})
}

// handleToolsCall routes a tools/call request to the registered tool handler.
func (s *Server) handleToolsCall(ctx context.Context, req Request) {
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		s.writeResponse(Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: -32602, Message: "Invalid params"},
		})
		return
	}

	var callParams struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(paramsBytes, &callParams); err != nil {
		s.writeResponse(Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: -32602, Message: "Invalid params"},
		})
		return
	}

	handler, ok := s.tools[callParams.Name]
	if !ok {
		s.writeResponse(Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: -32602, Message: fmt.Sprintf("Unknown tool: %s", callParams.Name)},
		})
		return
	}

	result, err := handler(ctx, callParams.Arguments)
	if err != nil {
		s.writeResponse(Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &Error{Code: -32603, Message: err.Error()},
		})
		return
	}

	s.writeResponse(Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	})
}

// writeResponse writes a JSON-RPC response to the output stream.
func (s *Server) writeResponse(resp Response) {
	if err := s.encoder.Encode(resp); err != nil {
		slog.Error("MCP server: failed to write response", "error", err)
	}
}

// submitEvent is a helper that submits an event to the dispatcher if one
// is configured. Errors are logged but not propagated — dispatcher events
// are advisory and non-blocking.
func (s *Server) submitEvent(event dispatcher.Event) {
	if s.dispatcher == nil {
		return
	}
	s.dispatcher.Submit(event)
}

// textResult creates a ToolResult with a single text content block.
func textResult(text string) *ToolResult {
	return &ToolResult{
		Content: []ContentBlock{
			{Type: "text", Text: text},
		},
	}
}

// jsonTextResult marshals the value to JSON and returns it as a text content block.
func jsonTextResult(v any) (*ToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}
	return textResult(string(data)), nil
}

// getRequiredString extracts a required string parameter from the params map.
func getRequiredString(params map[string]any, key string) (string, error) {
	v, ok := params[key]
	if !ok {
		return "", fmt.Errorf("missing required parameter: %s", key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("parameter %s must be a string", key)
	}
	if s == "" {
		return "", fmt.Errorf("parameter %s must not be empty", key)
	}
	return s, nil
}

// getOptionalString extracts an optional string parameter from the params map.
func getOptionalString(params map[string]any, key string) string {
	v, ok := params[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// getOptionalInt extracts an optional int parameter from the params map.
func getOptionalInt(params map[string]any, key string) int {
	v, ok := params[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}
