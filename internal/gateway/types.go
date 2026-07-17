// Package gateway implements the Loom Gateway Engine — the in-process
// component that manages agent session lifecycle, event dispatch, and
// gateway-level state tracking.
package gateway

import "time"

// GatewaySessionStatus represents the internal gateway-level state of an
// agent session. This is distinct from models.SessionStatus (which tracks
// the database-level connection state: active/stale/disconnected) and
// acp.SessionState (which tracks the ACP-protocol-level lifecycle:
// creating/active/idle/busy/error).
type GatewaySessionStatus string

const (
	SessionCreating GatewaySessionStatus = "creating"
	SessionActive   GatewaySessionStatus = "active"
	SessionIdle     GatewaySessionStatus = "idle"
	SessionBusy     GatewaySessionStatus = "busy"
	SessionError    GatewaySessionStatus = "error"
)

// GatewaySession represents a tracked agent session within the gateway.
// Sessions are keyed by (ProjectID, AgentType) and hold the running state
// of an agent's interaction with a specific project.
type GatewaySession struct {
	ProjectID      string
	AgentType      string
	SessionID      string // ACP session ID
	Status         GatewaySessionStatus
	AssignedTaskID string
	StoryID        string // Story being planned (for planner sessions)
	LastHeartbeat  time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// GatewayStatus is a snapshot of the overall gateway runtime state. It is
// intended for use by REST API status endpoints.
type GatewayStatus struct {
	Running           bool
	ActiveSessions    int
	QueueDepth        int
	EventsProcessed   int64
	UptimeSeconds     int64
	SessionsByProject map[string]int // project_id -> count
	SessionsByAgent   map[string]int // agent_type -> count
}

// GatewayEvent is a wrapper for events flowing through the gateway event
// processing loop. The Type field identifies the kind of event; the
// remaining fields provide routing and payload information.
type GatewayEvent struct {
	Type      string
	ProjectID string
	AgentType string
	TaskID    string
	SessionID string
	Payload   interface{}
}

// ---------------------------------------------------------------------------
// WebSocket broadcast payload types for gateway status events.
// ---------------------------------------------------------------------------

// SessionProjectEntry holds a session count for a project with its resolved
// name, replacing the raw map[string]int for WebSocket broadcasts.
type SessionProjectEntry struct {
	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name"`
	Count       int    `json:"count"`
}

// QueueJobEntry holds a queued job with the resolved project name.
type QueueJobEntry struct {
	ID          string `json:"id"`
	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name"`
	AgentType   string `json:"agent_type"`
	TaskID      string `json:"task_id"`
	EventRef    string `json:"event_ref"`
	CreatedAt   string `json:"created_at"`
}

// GatewayStatusBroadcast is the payload for periodic gateway_status
// WebSocket broadcasts. It carries resolved project names for both
// session counts and queued jobs.
type GatewayStatusBroadcast struct {
	Running           bool                  `json:"running"`
	ActiveSessions    int                   `json:"active_sessions"`
	QueueDepth        int                   `json:"queue_depth"`
	EventsProcessed   int64                 `json:"events_processed"`
	UptimeSeconds     int64                 `json:"uptime_seconds"`
	SessionsByProject []SessionProjectEntry `json:"sessions_by_project"`
	SessionsByAgent   map[string]int        `json:"sessions_by_agent"`
	QueueJobs         []QueueJobEntry       `json:"queue_jobs"`
}
