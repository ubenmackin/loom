package dispatcher

// Centralized event type constants used throughout the dispatcher and
// consumed by external components (MCP server, WebSocket hub).
const (
	EventTaskCompleted     = "task_status_changed"
	EventTaskBlocked       = "task_blocked"
	EventWorkRequested     = "work_requested"
	EventSessionRegistered = "session_registered"
	EventDependencyAdded   = "dependency_added"
	EventPeriodicTick      = "periodic_tick"
	EventDispatcherAction  = "dispatcher_event"
	EventGateCheck         = "gate_check"
	EventGateTaskCreated   = "gate_task_created"
	EventStalenessCheck    = "staleness_check"
	EventSessionStale      = "session_stale"
	EventTaskStale         = "task_stale"
	EventTasksGenerated    = "tasks_generated"
)
