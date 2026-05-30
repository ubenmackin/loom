package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

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
	Task              *models.Task `json:"task"`
	Instructions      string       `json:"instructions,omitempty"`
	HasUnreadComments bool         `json:"has_unread_comments"`
	UnreadComments    []string     `json:"unread_comments,omitempty"`
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
func (h *handlers) verifyTaskAssignment(ctx context.Context, taskID, sessionID string) error {
	task, err := h.tasks.GetByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("task not found: %w", store.ErrNotFound)
		}
		return fmt.Errorf("failed to get task: %w", err)
	}
	if task.AssignedTo != sessionID {
		return fmt.Errorf("task %q is not assigned to session %q", taskID, sessionID)
	}
	return nil
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

	h.dispatch.Submit(r.Context(), dispatcher.Event{
		Type:      dispatcher.EventWorkRequested,
		SessionID: req.SessionID,
	})

	instructions := task.Instructions
	template, terr := h.templates.GetByTaskType(r.Context(), task.TaskType)
	if terr == nil && template != nil {
		instructions = template.Template
		if task.Description != "" {
			instructions += "\n\n## Task Description\n\n" + task.Description
		}
		if task.Instructions != "" {
			instructions += "\n\n## Additional Instructions\n\n" + task.Instructions
		}
	}

	// Fetch unread comments and filter to human-authored only.
	var unreadBodies []string

	// 1. Get unread comments on tasks assigned to this session.
	unreads, uErr := h.comments.GetUnreadForSession(r.Context(), req.SessionID)
	if uErr == nil {
		for _, c := range unreads {
			if c.AuthorType == models.AuthorTypeHuman {
				unreadBodies = append(unreadBodies, c.Body)
			}
		}
	} else {
		slog.Error("failed to get unread comments for session", "session_id", req.SessionID, "error", uErr)
	}

	// 2. Get unread comments on the parent story as well.
	if task != nil && task.StoryID != "" {
		storyUnreads, sErr := h.comments.GetUnreadForSessionByWorkItem(r.Context(), req.SessionID, task.StoryID, models.WorkItemTypeStory)
		if sErr == nil {
			for _, c := range storyUnreads {
				if c.AuthorType == models.AuthorTypeHuman {
					unreadBodies = append(unreadBodies, c.Body)
				}
			}
		} else {
			slog.Error("failed to get unread story comments for session", "session_id", req.SessionID, "story_id", task.StoryID, "error", sErr)
		}
	}

	hasUnread := len(unreadBodies) > 0

	respondJSON(w, http.StatusOK, workAssignmentResponse{
		Task:              task,
		Instructions:      instructions,
		HasUnreadComments: hasUnread,
		UnreadComments:    unreadBodies,
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

	if err := h.verifyTaskAssignment(r.Context(), req.TaskID, req.SessionID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, err.Error())
		} else {
			respondError(w, http.StatusForbidden, err.Error())
		}
		return
	}

	if err := h.tasks.UpdateStatus(r.Context(), req.TaskID, models.StatusInProgress); err != nil {
		respondError(w, http.StatusBadRequest, "failed to start task: "+err.Error())
		return
	}

	details, err := json.Marshal(map[string]string{"session_id": req.SessionID})
	if err != nil {
		slog.Error("failed to marshal activity details", "error", err)
		details = []byte(`{"session_id":"` + req.SessionID + `"}`)
	}
	h.logActivity(r.Context(), &models.ActivityLogEntry{
		WorkItemID:   req.TaskID,
		WorkItemType: models.WorkItemTypeTask,
		Action:       "work_started",
		Details:      string(details),
	})
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

	if err := h.verifyTaskAssignment(r.Context(), req.TaskID, req.SessionID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, err.Error())
		} else {
			respondError(w, http.StatusForbidden, err.Error())
		}
		return
	}

	if err := h.tasks.UpdateStatus(r.Context(), req.TaskID, models.StatusDone); err != nil {
		respondError(w, http.StatusBadRequest, "failed to complete task: "+err.Error())
		return
	}

	if req.Result != "" {
		comment := &models.Comment{
			WorkItemID:   req.TaskID,
			WorkItemType: models.WorkItemTypeTask,
			AuthorID:     req.SessionID,
			AuthorType:   models.AuthorType(models.AssigneeTypeSession),
			Body:         req.Result,
		}
		if err := h.comments.Create(r.Context(), comment); err != nil {
			slog.Error("failed to create comment on work complete", "error", err)
		}
	}

	details, err := json.Marshal(map[string]string{"session_id": req.SessionID})
	if err != nil {
		slog.Error("failed to marshal activity details", "error", err)
		details = []byte(`{"session_id":"` + req.SessionID + `"}`)
	}
	h.logActivity(r.Context(), &models.ActivityLogEntry{
		WorkItemID:   req.TaskID,
		WorkItemType: models.WorkItemTypeTask,
		Action:       "work_completed",
		Details:      string(details),
	})
	h.updateSessionSeen(r.Context(), req.SessionID)

	h.dispatch.Submit(r.Context(), dispatcher.Event{
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

	if err := h.verifyTaskAssignment(r.Context(), req.TaskID, req.SessionID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, err.Error())
		} else {
			respondError(w, http.StatusForbidden, err.Error())
		}
		return
	}

	if err := h.tasks.UpdateStatus(r.Context(), req.TaskID, models.StatusBlocked); err != nil {
		respondError(w, http.StatusBadRequest, "failed to block task: "+err.Error())
		return
	}

	if req.Reason != "" {
		comment := &models.Comment{
			WorkItemID:   req.TaskID,
			WorkItemType: models.WorkItemTypeTask,
			AuthorID:     req.SessionID,
			AuthorType:   models.AuthorType(models.AssigneeTypeSession),
			Body:         "Blocked: " + req.Reason,
		}
		if err := h.comments.Create(r.Context(), comment); err != nil {
			slog.Error("failed to create comment on work block", "error", err)
		}
	}

	details, err := json.Marshal(map[string]string{"session_id": req.SessionID, "reason": req.Reason})
	if err != nil {
		slog.Error("failed to marshal activity details", "error", err)
		details = []byte(`{"session_id":"` + req.SessionID + `","reason":"` + req.Reason + `"}`)
	}
	h.logActivity(r.Context(), &models.ActivityLogEntry{
		WorkItemID:   req.TaskID,
		WorkItemType: models.WorkItemTypeTask,
		Action:       "work_blocked",
		Details:      string(details),
	})
	h.updateSessionSeen(r.Context(), req.SessionID)

	h.dispatch.Submit(r.Context(), dispatcher.Event{
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
