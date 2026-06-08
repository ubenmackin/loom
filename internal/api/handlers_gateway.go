package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ubenmackin/loom/internal/dispatcher"
)

// gatewayStatusResponse is the JSON response for GET /api/gateway/status.
// It maps the internal gateway.GatewayStatus into a frontend-friendly format.
type gatewayStatusResponse struct {
	Running           bool           `json:"running"`
	ActiveSessions    int            `json:"active_sessions"`
	QueueDepth        int            `json:"queue_depth"`
	EventsProcessed   int64          `json:"events_processed"`
	UptimeSeconds     int64          `json:"uptime_seconds"`
	SessionsByProject map[string]int `json:"sessions_by_project"`
	SessionsByAgent   map[string]int `json:"sessions_by_agent"`
}

// gatewayQueueResponse is the JSON response for GET /api/gateway/queue.
type gatewayQueueResponse struct {
	Total int          `json:"total"`
	Jobs  []gatewayJob `json:"jobs"`
}

// gatewayJob represents a single queued job in the gateway queue listing.
type gatewayJob struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	AgentType string `json:"agent_type"`
	TaskID    string `json:"task_id"`
	EventRef  string `json:"event_ref"`
	CreatedAt string `json:"created_at"`
}

// gatewayTriggerRequest is the JSON body for POST /api/gateway/trigger.
type gatewayTriggerRequest struct {
	EventType string `json:"event_type"`
	ProjectID string `json:"project_id"`
	AgentType string `json:"agent_type"`
	TaskID    string `json:"task_id"`
}

// handleGatewayStatus handles GET /api/gateway/status.
// Returns a snapshot of the gateway's current runtime state.
func (h *handlers) handleGatewayStatus(w http.ResponseWriter, r *http.Request) {
	if h.gateway == nil {
		respondError(w, http.StatusServiceUnavailable, "gateway not initialized")
		return
	}

	s := h.gateway.Status()
	resp := gatewayStatusResponse{
		Running:           s.Running,
		ActiveSessions:    s.ActiveSessions,
		QueueDepth:        s.QueueDepth,
		EventsProcessed:   s.EventsProcessed,
		UptimeSeconds:     s.UptimeSeconds,
		SessionsByProject: s.SessionsByProject,
		SessionsByAgent:   s.SessionsByAgent,
	}
	respondJSON(w, http.StatusOK, resp)
}

// handleGatewayQueue handles GET /api/gateway/queue.
// Returns a list of all queued jobs waiting for assignment.
func (h *handlers) handleGatewayQueue(w http.ResponseWriter, r *http.Request) {
	if h.gateway == nil {
		respondError(w, http.StatusServiceUnavailable, "gateway not initialized")
		return
	}

	jobs := h.gateway.Queue().ListAll()
	respJobs := make([]gatewayJob, 0, len(jobs))
	for _, j := range jobs {
		respJobs = append(respJobs, gatewayJob{
			ID:        j.ID,
			ProjectID: j.ProjectID,
			AgentType: j.AgentType,
			TaskID:    j.TaskID,
			EventRef:  j.EventRef,
			CreatedAt: j.CreatedAt.Format(time.RFC3339),
		})
	}

	resp := gatewayQueueResponse{
		Total: len(respJobs),
		Jobs:  respJobs,
	}
	respondJSON(w, http.StatusOK, resp)
}

// handleGatewayTrigger handles POST /api/gateway/trigger.
// Accepts a JSON body and submits a manual event to the gateway.
func (h *handlers) handleGatewayTrigger(w http.ResponseWriter, r *http.Request) {
	if h.gateway == nil {
		respondError(w, http.StatusServiceUnavailable, "gateway not initialized")
		return
	}

	var req gatewayTriggerRequest
	if err := decodeJSON(r, w, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	if req.EventType == "" {
		respondError(w, http.StatusBadRequest, "event_type is required")
		return
	}

	evt := dispatcher.Event{
		Type:   req.EventType,
		TaskID: req.TaskID,
		Payload: map[string]string{
			"event_type": req.EventType,
			"project_id": req.ProjectID,
			"agent_type": req.AgentType,
			"task_id":    req.TaskID,
		},
	}
	h.gateway.SubmitEvent(evt)

	respondJSON(w, http.StatusAccepted, map[string]string{
		"status":     "accepted",
		"event_type": req.EventType,
		"task_id":    req.TaskID,
		"message":    fmt.Sprintf("event %q submitted to gateway", req.EventType),
	})
}
