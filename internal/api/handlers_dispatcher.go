package api

import (
	"net/http"
	"time"

	"github.com/ubenmackin/loom/internal/dispatcher"
)

// dispatcherStatusResponse is the JSON response for GET /api/dispatcher/status.
// It converts the internal dispatcher.DispatcherStatus into frontend-friendly
// field names and types (e.g., time.Duration → seconds as a float64,
// *time.Time → RFC 3339 string or null).
type dispatcherStatusResponse struct {
	Running            bool             `json:"running"`
	StartedAt          string           `json:"started_at"`
	UptimeSeconds      float64          `json:"uptime_seconds"`
	EventQueueDepth    int              `json:"event_queue_depth"`
	EventsProcessed    map[string]int64 `json:"events_processed"`
	ReadyTasks         int              `json:"ready_tasks"`
	ActiveSessions     int              `json:"active_sessions"`
	PendingBuildGates  int              `json:"pending_build_gates"`
	PendingReviewGates int              `json:"pending_review_gates"`
	StaleSessions      int              `json:"stale_sessions"`
	LastAssignPass     *string          `json:"last_assign_pass"`
	LastStalenessCheck *string          `json:"last_staleness_check"`
}

// handleDispatcherStatus handles GET /api/dispatcher/status.
// Returns a snapshot of the dispatcher's current runtime state.
func (h *handlers) handleDispatcherStatus(w http.ResponseWriter, r *http.Request) {
	s := h.dispatch.Status()
	resp := toDispatcherStatusResponse(&s)
	respondJSON(w, http.StatusOK, resp)
}

// toDispatcherStatusResponse converts a dispatcher.DispatcherStatus to the
// JSON-friendly response format, converting time.Duration to seconds and
// optional *time.Time fields to nullable RFC 3339 strings.
func toDispatcherStatusResponse(s *dispatcher.DispatcherStatus) dispatcherStatusResponse {
	resp := dispatcherStatusResponse{
		Running:            s.Running,
		StartedAt:          s.StartedAt.Format(time.RFC3339),
		UptimeSeconds:      s.Uptime.Seconds(),
		EventQueueDepth:    s.EventQueueDepth,
		EventsProcessed:    s.EventsProcessed,
		ReadyTasks:         s.ReadyTasks,
		ActiveSessions:     s.ActiveSessions,
		PendingBuildGates:  s.PendingBuildGates,
		PendingReviewGates: s.PendingReviewGates,
		StaleSessions:      s.StaleSessions,
	}

	if s.LastAssignPass != nil {
		str := s.LastAssignPass.Format(time.RFC3339)
		resp.LastAssignPass = &str
	}
	if s.LastStalenessCheck != nil {
		str := s.LastStalenessCheck.Format(time.RFC3339)
		resp.LastStalenessCheck = &str
	}

	return resp
}
