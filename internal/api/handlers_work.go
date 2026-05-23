package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/dispatcher"
	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
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

// --- Helper functions ---

// verifyTaskAssignment retrieves a task and verifies it is assigned to the given session.
func (h *handlers) verifyTaskAssignment(ctx context.Context, taskID, sessionID string) (*models.Task, error) {
	task, err := h.tasks.GetByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("task not found")
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}
	if task.AssignedTo != sessionID {
		return nil, fmt.Errorf("task %q is not assigned to session %q", taskID, sessionID)
	}
	return task, nil
}

// logWorkActivity safely logs an activity entry for a work action.
func (h *handlers) logWorkActivity(ctx context.Context, taskID, action string, details string) {
	if err := h.activity.Log(ctx, &models.ActivityLogEntry{
		WorkItemID:   taskID,
		WorkItemType: models.WorkItemTypeTask,
		Action:       action,
		Details:      details,
	}); err != nil {
		slog.Error("failed to log activity", "action", action, "error", err)
	}
}

// updateSessionSeen safely updates the last_seen timestamp for a session.
func (h *handlers) updateSessionSeen(ctx context.Context, sessionID string) {
	if err := h.sessions.UpdateLastSeen(ctx, sessionID); err != nil {
		slog.Error("failed to update session last_seen", "session_id", sessionID, "error", err)
	}
}

// --- Handlers ---

// workRequest handles POST /api/work/request
func (h *handlers) workRequest(w http.ResponseWriter, r *http.Request) {
	var req workRequestReq
	if err := decodeJSON(r, w, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.SessionID == "" {
		respondError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	session, err := h.sessions.GetByID(r.Context(), req.SessionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "session not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get session: "+err.Error())
		return
	}
	if session.Status != models.SessionStatusActive {
		respondError(w, http.StatusForbidden, fmt.Sprintf("session %q is not active (status=%q)", req.SessionID, session.Status))
		return
	}

	task, err := h.dispatch.AssignWork(r.Context(), req.SessionID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to assign work: "+err.Error())
		return
	}
	if task == nil {
		respondJSON(w, http.StatusOK, map[string]string{"message": "no work available"})
		return
	}

	h.dispatch.Submit(dispatcher.Event{
		Type:      dispatcher.EventWorkRequested,
		SessionID: req.SessionID,
	})

	instructions := task.Instructions
	template, terr := h.templates.GetByTaskType(r.Context(), string(task.TaskType))
	if terr == nil && template != nil {
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
func (h *handlers) workStart(w http.ResponseWriter, r *http.Request) {
	var req workStartReq
	if err := decodeJSON(r, w, &req); err != nil {
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

	task, err := h.verifyTaskAssignment(r.Context(), req.TaskID, req.SessionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, err.Error())
		} else {
			respondError(w, http.StatusForbidden, err.Error())
		}
		return
	}
	_ = task // task is verified; proceed with status update

	if err := h.tasks.UpdateStatus(r.Context(), req.TaskID, models.StatusInProgress); err != nil {
		respondError(w, http.StatusBadRequest, "failed to start task: "+err.Error())
		return
	}

	details, _ := json.Marshal(map[string]string{"session_id": req.SessionID})
	h.logWorkActivity(r.Context(), req.TaskID, "work_started", string(details))
	h.updateSessionSeen(r.Context(), req.SessionID)

	updated, err := h.tasks.GetByID(r.Context(), req.TaskID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch updated task")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"task": updated})
}

// workComplete handles POST /api/work/complete
func (h *handlers) workComplete(w http.ResponseWriter, r *http.Request) {
	var req workCompleteReq
	if err := decodeJSON(r, w, &req); err != nil {
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

	task, err := h.verifyTaskAssignment(r.Context(), req.TaskID, req.SessionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, err.Error())
		} else {
			respondError(w, http.StatusForbidden, err.Error())
		}
		return
	}
	_ = task // task is verified; proceed with status update

	if err := h.tasks.UpdateStatus(r.Context(), req.TaskID, models.StatusDone); err != nil {
		respondError(w, http.StatusBadRequest, "failed to complete task: "+err.Error())
		return
	}

	if req.Result != "" {
		comment := &models.Comment{
			WorkItemID:   req.TaskID,
			WorkItemType: models.WorkItemTypeTask,
			AuthorID:     req.SessionID,
			AuthorType:   string(models.AssigneeTypeSession),
			Body:         req.Result,
		}
		if err := h.comments.Create(r.Context(), comment); err != nil {
			slog.Error("failed to create comment on work complete", "error", err)
		}
	}

	details, _ := json.Marshal(map[string]string{"session_id": req.SessionID})
	h.logWorkActivity(r.Context(), req.TaskID, "work_completed", string(details))
	h.updateSessionSeen(r.Context(), req.SessionID)

	h.dispatch.Submit(dispatcher.Event{
		Type:   dispatcher.EventTaskCompleted,
		TaskID: req.TaskID,
	})

	updated, err := h.tasks.GetByID(r.Context(), req.TaskID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch updated task")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"task": updated})
}

// workBlock handles POST /api/work/block
func (h *handlers) workBlock(w http.ResponseWriter, r *http.Request) {
	var req workBlockReq
	if err := decodeJSON(r, w, &req); err != nil {
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

	task, err := h.verifyTaskAssignment(r.Context(), req.TaskID, req.SessionID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			respondError(w, http.StatusNotFound, err.Error())
		} else {
			respondError(w, http.StatusForbidden, err.Error())
		}
		return
	}
	_ = task // task is verified; proceed with status update

	if err := h.tasks.UpdateStatus(r.Context(), req.TaskID, models.StatusBlocked); err != nil {
		respondError(w, http.StatusBadRequest, "failed to block task: "+err.Error())
		return
	}

	if req.Reason != "" {
		comment := &models.Comment{
			WorkItemID:   req.TaskID,
			WorkItemType: models.WorkItemTypeTask,
			AuthorID:     req.SessionID,
			AuthorType:   string(models.AssigneeTypeSession),
			Body:         "Blocked: " + req.Reason,
		}
		if err := h.comments.Create(r.Context(), comment); err != nil {
			slog.Error("failed to create comment on work block", "error", err)
		}
	}

	details, _ := json.Marshal(map[string]string{"session_id": req.SessionID, "reason": req.Reason})
	h.logWorkActivity(r.Context(), req.TaskID, "work_blocked", string(details))
	h.updateSessionSeen(r.Context(), req.SessionID)

	h.dispatch.Submit(dispatcher.Event{
		Type:   dispatcher.EventTaskBlocked,
		TaskID: req.TaskID,
	})

	updated, err := h.tasks.GetByID(r.Context(), req.TaskID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch updated task")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"task": updated})
}

// workKeepalive handles POST /api/work/keepalive
func (h *handlers) workKeepalive(w http.ResponseWriter, r *http.Request) {
	var req workKeepaliveReq
	if err := decodeJSON(r, w, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.SessionID == "" {
		respondError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	if err := h.sessions.UpdateLastSeen(r.Context(), req.SessionID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "session not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update last seen: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
