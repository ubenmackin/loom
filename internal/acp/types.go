// Package acp provides the opencode ACP (Agent Communication Protocol) types
// and a WebSocket client for communicating with an opencode server via ACP.
package acp

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
