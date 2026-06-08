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
