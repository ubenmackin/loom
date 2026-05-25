package api

import (
	"net/http"
	"time"

	"github.com/ubenmackin/loom/internal/dispatcher"
)

// dispatcherStatusResponse is the JSON response for GET /api/dispatcher/status.
// It converts the internal dispatcher.DispatcherStatus into frontend-friendly
// field names and types (e.g., time.Duration → seconds as a float64).
type dispatcherStatusResponse struct {
	Running         bool             `json:"running"`
	StartedAt       string           `json:"started_at"`
	UptimeSeconds   float64          `json:"uptime_seconds"`
	EventQueueDepth int              `json:"event_queue_depth"`
	EventsProcessed map[string]int64 `json:"events_processed"`
}

// handleDispatcherStatus handles GET /api/dispatcher/status.
// Returns a snapshot of the dispatcher's current runtime state.
func (h *handlers) handleDispatcherStatus(w http.ResponseWriter, r *http.Request) {
	s := h.dispatch.Status()
	resp := toDispatcherStatusResponse(s)
	respondJSON(w, http.StatusOK, resp)
}

// toDispatcherStatusResponse converts a dispatcher.DispatcherStatus to the
// JSON-friendly response format, converting time.Duration to seconds.
func toDispatcherStatusResponse(s dispatcher.DispatcherStatus) dispatcherStatusResponse {
	return dispatcherStatusResponse{
		Running:         s.Running,
		StartedAt:       s.StartedAt.Format(time.RFC3339),
		UptimeSeconds:   s.Uptime.Seconds(),
		EventQueueDepth: s.EventQueueDepth,
		EventsProcessed: s.EventsProcessed,
	}
}
