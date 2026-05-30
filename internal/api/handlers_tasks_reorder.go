package api

import (
	"errors"
	"net/http"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
)

// --- Request types ---

type reorderTaskRequest struct {
	StoryID   *string `json:"story_id,omitempty"`
	Status    *string `json:"status,omitempty"`
	SortOrder *int    `json:"sort_order,omitempty"`
}

type batchReorderItem struct {
	ID        string  `json:"id"`
	SortOrder int     `json:"sort_order"`
	Status    *string `json:"status,omitempty"`
	StoryID   *string `json:"story_id,omitempty"`
}

type batchReorderRequest struct {
	Tasks []batchReorderItem `json:"tasks"`
}

// --- Handlers ---

// updateTaskReorder handles PATCH /api/tasks/{id}/reorder
func (h *handlers) updateTaskReorder(w http.ResponseWriter, r *http.Request) {
	id, ok := h.resolveAndRespond(w, r, "id", string(models.WorkItemTypeTask), "task")
	if !ok {
		return
	}

	var req reorderTaskRequest
	if err := decodeJSON(r, w, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Get current task
	task, err := h.tasks.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get task: "+err.Error())
		return
	}

	// Apply partial updates
	if req.StoryID != nil {
		task.StoryID = *req.StoryID
	}
	if req.Status != nil {
		if !validStatus(*req.Status) {
			respondError(w, http.StatusBadRequest, "invalid status value")
			return
		}
		task.Status = models.Status(*req.Status)
	}
	if req.SortOrder != nil {
		task.SortOrder = *req.SortOrder
	}

	if err := h.tasks.Update(r.Context(), task); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusConflict, "task was modified or deleted concurrently")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to reorder task: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, task)
}

// batchReorderTasks handles PATCH /api/tasks/reorder
func (h *handlers) batchReorderTasks(w http.ResponseWriter, r *http.Request) {
	var req batchReorderRequest
	if err := decodeJSON(r, w, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if len(req.Tasks) == 0 {
		respondError(w, http.StatusBadRequest, "tasks array is required and must not be empty")
		return
	}

	// Fetch all tasks in the batch
	tasks := make([]*models.Task, len(req.Tasks))
	for i, item := range req.Tasks {
		task, err := h.tasks.GetByID(r.Context(), item.ID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				respondError(w, http.StatusNotFound, "task "+item.ID+" not found")
				return
			}
			respondError(w, http.StatusInternalServerError, "failed to get task: "+err.Error())
			return
		}
		tasks[i] = task
	}

	// Apply updates to each task
	for i, item := range req.Tasks {
		task := tasks[i]
		task.SortOrder = item.SortOrder
		if item.Status != nil {
			if !validStatus(*item.Status) {
				respondError(w, http.StatusBadRequest, "invalid status for task "+item.ID)
				return
			}
			task.Status = models.Status(*item.Status)
		}
		if item.StoryID != nil {
			task.StoryID = *item.StoryID
		}
	}

	// Apply all updates in a transaction
	if err := h.tasks.BatchUpdate(r.Context(), tasks); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to batch update tasks: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"updated": len(tasks)})
}
