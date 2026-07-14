// Package acp provides the opencode ACP (Agent Communication Protocol) types
// and a subprocess + JSON-RPC client for communicating with an opencode server via ACP.
package acp

import "encoding/json"

// ClientInfo represents the client identification info.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeRequest is sent to initialize an ACP session.
type InitializeRequest struct {
	ProtocolVersion int         `json:"protocolVersion"`
	ClientInfo      *ClientInfo `json:"clientInfo,omitempty"`
}

// InitializeResponse is the response to an initialize request.
type InitializeResponse struct {
	ProtocolVersion   int            `json:"protocolVersion"`
	AgentCapabilities map[string]any `json:"agentCapabilities"`
	AgentInfo         *ClientInfo    `json:"agentInfo,omitempty"`
}

// MCPServer represents an MCP server configuration.
type MCPServer struct {
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Env     []EnvVar `json:"env"`
}

// EnvVar represents an environment variable.
type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// NewSessionRequest is sent to create a new ACP session.
type NewSessionRequest struct {
	Cwd                   string      `json:"cwd"`
	MCPServers            []MCPServer `json:"mcpServers"`
	AdditionalDirectories []string    `json:"additionalDirectories,omitempty"`
}

// NewSessionResponse is the response to a session/new request.
type NewSessionResponse struct {
	SessionID string `json:"sessionId"`
}

// ContentBlock represents a content block in a prompt.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// PromptRequest is sent to send a prompt to an existing session.
type PromptRequest struct {
	SessionID string         `json:"sessionId"`
	Prompt    []ContentBlock `json:"prompt"`
}

// PromptResponse is the response to a session/prompt request.
type PromptResponse struct {
	StopReason string `json:"stopReason"`
}

// JSONRPCRequest represents a JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error object.
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
