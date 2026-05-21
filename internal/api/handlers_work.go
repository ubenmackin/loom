package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/dispatcher"
	"github.com/ubenmackin/loom/internal/models"
)

// --- Request/Response types ---

type workRequestReq struct {
	SessionID string `json:"session_id"`
}

type workStartReq struct {
	SessionID string `json:"session_id"`
	TaskID    string `json:"task_id"`
}

type workCompleteReq struct {
	SessionID string `json:"session_id"`
	TaskID    string `json:"task_id"`
	Result    string `json:"result"`
}

type workBlockReq struct {
	SessionID string `json:"session_id"`
	TaskID    string `json:"task_id"`
	Reason    string `json:"reason"`
}

type workKeepaliveReq struct {
	SessionID string `json:"session_id"`
}

type workAssignmentResponse struct {
	Task         *models.Task `json:"task"`
	Instructions string       `json:"instructions,omitempty"`
}

// --- Route registration ---

func (h *handlers) registerWorkRoutes(r chi.Router) {
	r.Post("/request", h.workRequest)
	r.Post("/start", h.workStart)
	r.Post("/complete", h.workComplete)
	r.Post("/block", h.workBlock)
	r.Post("/keepalive", h.workKeepalive)
}

// --- Handlers ---

// workRequest handles POST /api/work/request
// An agent requests the next available work. The dispatcher assigns a task.
func (h *handlers) workRequest(w http.ResponseWriter, r *http.Request) {
	var req workRequestReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.SessionID == "" {
		respondError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	// Verify session exists and is active.
	session, err := h.sessions.GetByID(r.Context(), req.SessionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "session not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get session: "+err.Error())
		return
	}
	if session.Status != models.SessionStatusActive {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("session %q is not active (status=%q)", req.SessionID, session.Status))
		return
	}

	// Use the dispatcher to find and assign work.
	task, err := h.dispatch.AssignWork(r.Context(), req.SessionID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to assign work: "+err.Error())
		return
	}
	if task == nil {
		respondJSON(w, http.StatusOK, map[string]string{"message": "no work available"})
		return
	}

	// Assemble instructions from the task's instructions field and the
	// prompt template for the task type.
	instructions := task.Instructions
	template, terr := h.templates.GetByTaskType(r.Context(), task.TaskType)
	if terr == nil && template != nil {
		// If a template exists, use it as the base and append task-specific instructions.
		instructions = template.Template
		if task.Instructions != "" {
			instructions += "\n\n" + task.Instructions
		}
	}

	respondJSON(w, http.StatusOK, workAssignmentResponse{
		Task:         task,
		Instructions: instructions,
	})
}

// workStart handles POST /api/work/start
// An agent confirms it is starting work on an assigned task.
func (h *handlers) workStart(w http.ResponseWriter, r *http.Request) {
	var req workStartReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.SessionID == "" {
		respondError(w, http.StatusBadRequest, "session_id is required")
		return
	}
	if req.TaskID == "" {
		respondError(w, http.StatusBadRequest, "task_id is required")
		return
	}

	// Verify the task exists and is assigned to this session.
	task, err := h.tasks.GetByID(r.Context(), req.TaskID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get task: "+err.Error())
		return
	}

	if task.AssignedTo != req.SessionID {
		respondError(w, http.StatusForbidden, fmt.Sprintf("task %q is not assigned to session %q", req.TaskID, req.SessionID))
		return
	}

	// Transition status to in_progress.
	if err := h.tasks.UpdateStatus(r.Context(), req.TaskID, models.StatusInProgress); err != nil {
		respondError(w, http.StatusBadRequest, "failed to start task: "+err.Error())
		return
	}

	// Log activity.
	if err := h.activity.Log(r.Context(), &models.ActivityLogEntry{
		WorkItemID:   req.TaskID,
		WorkItemType: models.WorkItemTypeTask,
		Action:       "work_started",
		Details:      func() string { d, _ := json.Marshal(map[string]string{"session_id": req.SessionID}); return string(d) }(),
	}); err != nil {
		slog.Error("failed to log activity", "action", "work_started", "error", err)
	}

	// Update session last_seen.
	if err := h.sessions.UpdateLastSeen(r.Context(), req.SessionID); err != nil {
		slog.Error("failed to update session last_seen", "session_id", req.SessionID, "error", err)
	}

	// Return updated task.
	updated, err := h.tasks.GetByID(r.Context(), req.TaskID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch updated task")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"task": updated})
}

// workComplete handles POST /api/work/complete
// An agent reports that work is done. Marks task Done, adds result as a comment,
// and triggers dispatcher for dependency resolution.
func (h *handlers) workComplete(w http.ResponseWriter, r *http.Request) {
	var req workCompleteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.SessionID == "" {
		respondError(w, http.StatusBadRequest, "session_id is required")
		return
	}
	if req.TaskID == "" {
		respondError(w, http.StatusBadRequest, "task_id is required")
		return
	}

	// Verify the task exists and is assigned to this session.
	task, err := h.tasks.GetByID(r.Context(), req.TaskID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get task: "+err.Error())
		return
	}

	if task.AssignedTo != req.SessionID {
		respondError(w, http.StatusForbidden, fmt.Sprintf("task %q is not assigned to session %q", req.TaskID, req.SessionID))
		return
	}

	// Transition status to done.
	if err := h.tasks.UpdateStatus(r.Context(), req.TaskID, models.StatusDone); err != nil {
		respondError(w, http.StatusBadRequest, "failed to complete task: "+err.Error())
		return
	}

	// Add result as a comment.
	if req.Result != "" {
		comment := &models.Comment{
			WorkItemID:   req.TaskID,
			WorkItemType: models.WorkItemTypeTask,
			AuthorID:     req.SessionID,
			AuthorType:   models.AssigneeTypeSession,
			Body:         req.Result,
		}
		if err := h.comments.Create(r.Context(), comment); err != nil {
			slog.Error("failed to create comment on work complete", "error", err)
		}
	}

	// Log activity.
	if err := h.activity.Log(r.Context(), &models.ActivityLogEntry{
		WorkItemID:   req.TaskID,
		WorkItemType: models.WorkItemTypeTask,
		Action:       "work_completed",
		Details:      func() string { d, _ := json.Marshal(map[string]string{"session_id": req.SessionID}); return string(d) }(),
	}); err != nil {
		slog.Error("failed to log activity", "action", "work_completed", "error", err)
	}

	// Update session last_seen.
	if err := h.sessions.UpdateLastSeen(r.Context(), req.SessionID); err != nil {
		slog.Error("failed to update session last_seen", "session_id", req.SessionID, "error", err)
	}

	// Submit dispatcher event for dependency resolution.
	h.dispatch.Submit(dispatcher.Event{
		Type:   dispatcher.EventTaskCompleted,
		TaskID: req.TaskID,
	})

	// Return updated task.
	updated, err := h.tasks.GetByID(r.Context(), req.TaskID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch updated task")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"task": updated})
}

// workBlock handles POST /api/work/block
// An agent reports it is blocked. Updates status, adds reason as comment,
// and triggers the dispatcher.
func (h *handlers) workBlock(w http.ResponseWriter, r *http.Request) {
	var req workBlockReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.SessionID == "" {
		respondError(w, http.StatusBadRequest, "session_id is required")
		return
	}
	if req.TaskID == "" {
		respondError(w, http.StatusBadRequest, "task_id is required")
		return
	}

	// Verify the task exists and is assigned to this session.
	task, err := h.tasks.GetByID(r.Context(), req.TaskID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get task: "+err.Error())
		return
	}

	if task.AssignedTo != req.SessionID {
		respondError(w, http.StatusForbidden, fmt.Sprintf("task %q is not assigned to session %q", req.TaskID, req.SessionID))
		return
	}

	// Transition status to blocked.
	if err := h.tasks.UpdateStatus(r.Context(), req.TaskID, models.StatusBlocked); err != nil {
		respondError(w, http.StatusBadRequest, "failed to block task: "+err.Error())
		return
	}

	// Add blocking reason as a comment.
	if req.Reason != "" {
		comment := &models.Comment{
			WorkItemID:   req.TaskID,
			WorkItemType: models.WorkItemTypeTask,
			AuthorID:     req.SessionID,
			AuthorType:   models.AssigneeTypeSession,
			Body:         "Blocked: " + req.Reason,
		}
		if err := h.comments.Create(r.Context(), comment); err != nil {
			slog.Error("failed to create comment on work block", "error", err)
		}
	}

	// Log activity.
	if err := h.activity.Log(r.Context(), &models.ActivityLogEntry{
		WorkItemID:   req.TaskID,
		WorkItemType: models.WorkItemTypeTask,
		Action:       "work_blocked",
		Details: func() string {
			d, _ := json.Marshal(map[string]string{"session_id": req.SessionID, "reason": req.Reason})
			return string(d)
		}(),
	}); err != nil {
		slog.Error("failed to log activity", "action", "work_blocked", "error", err)
	}

	// Update session last_seen.
	if err := h.sessions.UpdateLastSeen(r.Context(), req.SessionID); err != nil {
		slog.Error("failed to update session last_seen", "session_id", req.SessionID, "error", err)
	}

	// Submit dispatcher event.
	h.dispatch.Submit(dispatcher.Event{
		Type:   dispatcher.EventTaskBlocked,
		TaskID: req.TaskID,
	})

	// Return updated task.
	updated, err := h.tasks.GetByID(r.Context(), req.TaskID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch updated task")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"task": updated})
}

// workKeepalive handles POST /api/work/keepalive
// An opportunistic keepalive to update the session's last_seen_at.
func (h *handlers) workKeepalive(w http.ResponseWriter, r *http.Request) {
	var req workKeepaliveReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.SessionID == "" {
		respondError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	if err := h.sessions.UpdateLastSeen(r.Context(), req.SessionID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "session not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update last seen: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
