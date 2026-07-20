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
//
// Modes and ConfigOptions are optional: a server that returns neither (older
// opencode builds) still parses cleanly because Modes is a pointer and
// ConfigOptions is a nilable slice, both with omitempty JSON tags. The gateway
// prefers ConfigOptions (the modern session/set_config_option surface) and
// falls back to Modes (the legacy session/set_mode surface).
type NewSessionResponse struct {
	SessionID     string                `json:"sessionId"`
	Modes         *SessionModeState     `json:"modes,omitempty"`
	ConfigOptions []SessionConfigOption `json:"configOptions,omitempty"`
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

// SessionModeState describes the set of modes an Agent can operate in and the
// one currently active. See ACP v1 session-modes.
type SessionModeState struct {
	CurrentModeID  string        `json:"currentModeId"`
	AvailableModes []SessionMode `json:"availableModes"`
}

// SessionMode is a single mode the Agent can operate in.
type SessionMode struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// SessionConfigOption is a session configuration option selector and its
// current state. The ACP v1 wire shape is a discriminated union on `type`
// ("select" or "boolean"); this struct flattens both variants by carrying
// CurrentValue as `any` and treating Options as optional (only populated for
// the "select" variant).
type SessionConfigOption struct {
	ID           string                      `json:"id"`
	Name         string                      `json:"name"`
	Description  string                      `json:"description,omitempty"`
	Category     string                      `json:"category,omitempty"`
	Type         string                      `json:"type"`
	CurrentValue any                         `json:"currentValue"`
	Options      []SessionConfigOptionOption `json:"options,omitempty"`
}

// SessionConfigOptionOption is a single selectable value for a "select"-type
// SessionConfigOption. Per the ACP v1 schema this is the SessionConfigSelectOption
// shape (name + value, with an optional description).
type SessionConfigOptionOption struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

// SetSessionModeRequest is sent to set the current mode for a session.
type SetSessionModeRequest struct {
	SessionID string `json:"sessionId"`
	ModeID    string `json:"modeId"`
}

// SetSessionModeResponse is the response to session/set_mode. Per spec it
// carries only the reserved `_meta` field (no useful return payload).
type SetSessionModeResponse struct {
	Meta map[string]any `json:"_meta,omitempty"`
}

// SetSessionConfigOptionRequest is sent to set the current value of a session
// configuration option. Value is marshaled as `any` (a string for the
// type="select" variant, a bool for type="boolean"). Type is the optional
// discriminator and is omitted for the default value_id variant.
type SetSessionConfigOptionRequest struct {
	SessionID string `json:"sessionId"`
	ConfigID  string `json:"configId"`
	Value     any    `json:"value"`
	Type      string `json:"type,omitempty"`
}

// SetSessionConfigOptionResponse is the response to session/set_config_option.
// Per the spec it MUST contain the full updated set of config options.
type SetSessionConfigOptionResponse struct {
	ConfigOptions []SessionConfigOption `json:"configOptions"`
}
