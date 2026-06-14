// Package acp provides the opencode ACP (Agent Communication Protocol) types
// and a subprocess + JSON-RPC client for communicating with an opencode server via ACP.
package acp

import "encoding/json"

// SessionState represents the lifecycle state of an agent session.
type SessionState string

const (
	SessionStateCreating SessionState = "creating"
	SessionStateActive   SessionState = "active"
	SessionStateIdle     SessionState = "idle"
	SessionStateBusy     SessionState = "busy"
	SessionStateError    SessionState = "error"
)

// SessionMessage is sent during session lifecycle operations.
type SessionMessage struct {
	Type      string       `json:"type"`
	SessionID string       `json:"session_id,omitempty"`
	ProjectID string       `json:"project_id,omitempty"`
	AgentType string       `json:"agent_type,omitempty"`
	TaskID    string       `json:"task_id,omitempty"`
	Result    string       `json:"result,omitempty"`
	Status    SessionState `json:"status,omitempty"`
}

// TaskMessage is sent for task operation flows.
type TaskMessage struct {
	Type         string `json:"type"`
	TaskID       string `json:"task_id,omitempty"`
	SessionID    string `json:"session_id,omitempty"`
	Title        string `json:"title,omitempty"`
	Description  string `json:"description,omitempty"`
	Instructions string `json:"instructions,omitempty"`
	Status       string `json:"status,omitempty"`
	Result       string `json:"result,omitempty"`
}

// ACPResponse is a generic response wrapper returned for any ACP request.
type ACPResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	TaskID    string `json:"task_id,omitempty"`
	Error     string `json:"error,omitempty"`
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
