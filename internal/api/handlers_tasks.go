package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
)

// --- Request/Response types ---

type createTaskRequest struct {
	Title        string `json:"title"`
	Description  string `json:"description,omitempty"`
	TaskType     string `json:"task_type,omitempty"`
	AssignedTo   string `json:"assigned_to,omitempty"`
	AssigneeType string `json:"assignee_type,omitempty"`
	SortOrder    int    `json:"sort_order,omitempty"`
	Instructions string `json:"instructions,omitempty"`
}

type updateTaskRequest struct {
	Title        *string `json:"title,omitempty"`
	Description  *string `json:"description,omitempty"`
	Status       *string `json:"status,omitempty"`
	TaskType     *string `json:"task_type,omitempty"`
	AssignedTo   *string `json:"assigned_to,omitempty"`
	AssigneeType *string `json:"assignee_type,omitempty"`
	SortOrder    *int    `json:"sort_order,omitempty"`
	Instructions *string `json:"instructions,omitempty"`
	IsStale      *bool   `json:"is_stale,omitempty"`
}

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

type addDependencyRequest struct {
	DependsOnTaskID string `json:"depends_on_task_id"`
}

type taskDetailResponse struct {
	Task         *models.Task   `json:"task"`
	Dependencies []string       `json:"dependencies"`
	Dependents   []*models.Task `json:"dependents"`
}

// --- Route registration ---

func (h *handlers) registerTaskRoutes(r chi.Router) {
	r.Get("/", h.listTasks)
	r.Patch("/reorder", h.batchReorderTasks)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.getTask)
		r.Put("/", h.updateTask)
		r.Patch("/status", h.updateTaskStatus)
		r.Patch("/reorder", h.updateTaskReorder)
		r.Get("/blockers", h.getTaskBlockers)
		r.Post("/dependencies", h.addDependency)
		r.Delete("/dependencies/{dependsOnId}", h.removeDependency)
		r.Get("/activity", h.getTaskActivity)
		r.Delete("/", h.deleteTask)
	})
}

// --- Handlers ---

// listTasks handles GET /api/tasks
func (h *handlers) listTasks(w http.ResponseWriter, r *http.Request) {
	filter := store.TaskFilter{
		StoryID:    r.URL.Query().Get("story_id"),
		Status:     models.Status(r.URL.Query().Get("status")),
		AssignedTo: r.URL.Query().Get("assigned_to"),
		TaskType:   models.TaskType(r.URL.Query().Get("task_type")),
	}

	tasks, err := h.tasks.List(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list tasks: "+err.Error())
		return
	}

	if tasks == nil {
		tasks = []*models.Task{}
	}
	respondJSON(w, http.StatusOK, tasks)
}

// createTaskUnderStory handles POST /api/stories/{id}/tasks
func (h *handlers) createTaskUnderStory(w http.ResponseWriter, r *http.Request) {
	storyID, err := h.resolveIDParam(r, "id", string(models.WorkItemTypeStory))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "story not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid story id: "+err.Error())
		return
	}

	// Verify story exists (redundant after resolveIDParam for non-numeric IDs, but kept for clarity).
	if _, err := h.stories.GetByID(r.Context(), storyID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "story not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get story: "+err.Error())
		return
	}

	var req createTaskRequest
	if err := decodeJSON(r, w, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		respondError(w, http.StatusBadRequest, "title is required")
		return
	}

	if req.TaskType == "" {
		req.TaskType = string(models.TaskTypeCode)
	} else if !validTaskType(req.TaskType) {
		respondError(w, http.StatusBadRequest, "invalid task_type")
		return
	}

	if req.AssigneeType != "" && !validAssigneeType(req.AssigneeType) {
		respondError(w, http.StatusBadRequest, "invalid assignee_type")
		return
	}

	task := &models.Task{
		StoryID:      storyID,
		Title:        strings.TrimSpace(req.Title),
		Description:  req.Description,
		TaskType:     models.TaskType(req.TaskType),
		AssignedTo:   req.AssignedTo,
		AssigneeType: models.AssigneeType(req.AssigneeType),
		SortOrder:    req.SortOrder,
		Instructions: req.Instructions,
	}

	if err := h.tasks.Create(r.Context(), task); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create task: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, task)
}

// getTask handles GET /api/tasks/{id}
func (h *handlers) getTask(w http.ResponseWriter, r *http.Request) {
	id, err := h.resolveIDParam(r, "id", string(models.WorkItemTypeTask))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid task id: "+err.Error())
		return
	}

	task, err := h.tasks.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get task: "+err.Error())
		return
	}

	deps, err := h.tasks.GetDependencies(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get dependencies: "+err.Error())
		return
	}
	if deps == nil {
		deps = []string{}
	}

	dependents, err := h.tasks.GetDependents(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get dependents: "+err.Error())
		return
	}
	if dependents == nil {
		dependents = []*models.Task{}
	}

	respondJSON(w, http.StatusOK, taskDetailResponse{
		Task:         task,
		Dependencies: deps,
		Dependents:   dependents,
	})
}

// updateTask handles PUT /api/tasks/{id}
func (h *handlers) updateTask(w http.ResponseWriter, r *http.Request) {
	id, err := h.resolveIDParam(r, "id", string(models.WorkItemTypeTask))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid task id: "+err.Error())
		return
	}

	task, err := h.tasks.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get task: "+err.Error())
		return
	}

	var req updateTaskRequest
	if err := decodeJSON(r, w, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Apply partial updates.
	if req.Title != nil {
		if strings.TrimSpace(*req.Title) == "" {
			respondError(w, http.StatusBadRequest, "title cannot be empty")
			return
		}
		task.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.TaskType != nil {
		if !validTaskType(*req.TaskType) {
			respondError(w, http.StatusBadRequest, "invalid task_type")
			return
		}
		task.TaskType = models.TaskType(*req.TaskType)
	}
	if req.Status != nil {
		if !validStatus(*req.Status) {
			respondError(w, http.StatusBadRequest, "invalid status value")
			return
		}
		task.Status = models.Status(*req.Status)
	}
	if req.AssignedTo != nil {
		task.AssignedTo = *req.AssignedTo
	}
	if req.AssigneeType != nil {
		if !validAssigneeType(*req.AssigneeType) {
			respondError(w, http.StatusBadRequest, "invalid assignee_type")
			return
		}
		task.AssigneeType = models.AssigneeType(*req.AssigneeType)
	}
	if req.SortOrder != nil {
		task.SortOrder = *req.SortOrder
	}
	if req.Instructions != nil {
		task.Instructions = *req.Instructions
	}
	if req.IsStale != nil {
		task.IsStale = *req.IsStale
	}

	if err := h.tasks.Update(r.Context(), task); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusConflict, "task was modified or deleted concurrently")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update task: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, task)
}

// updateTaskStatus handles PATCH /api/tasks/{id}/status
func (h *handlers) updateTaskStatus(w http.ResponseWriter, r *http.Request) {
	id, err := h.resolveIDParam(r, "id", string(models.WorkItemTypeTask))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid task id: "+err.Error())
		return
	}

	var req updateStatusRequest
	if err := decodeJSON(r, w, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Status == "" {
		respondError(w, http.StatusBadRequest, "status is required")
		return
	}

	if !validStatus(req.Status) {
		respondError(w, http.StatusBadRequest, "invalid status value")
		return
	}

	if err := h.tasks.UpdateStatus(r.Context(), id, models.Status(req.Status)); err != nil {
		if errors.Is(err, store.ErrInvalidTransition) {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update task status: "+err.Error())
		return
	}

	// Return updated task.
	task, err := h.tasks.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get updated task: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, task)
}

// updateTaskReorder handles PATCH /api/tasks/{id}/reorder
func (h *handlers) updateTaskReorder(w http.ResponseWriter, r *http.Request) {
	id, err := h.resolveIDParam(r, "id", string(models.WorkItemTypeTask))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid task id: "+err.Error())
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

	// Validate all task IDs exist first
	taskIDs := make([]string, len(req.Tasks))
	for i, item := range req.Tasks {
		taskIDs[i] = item.ID
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

// getTaskBlockers handles GET /api/tasks/{id}/blockers
func (h *handlers) getTaskBlockers(w http.ResponseWriter, r *http.Request) {
	id, err := h.resolveIDParam(r, "id", string(models.WorkItemTypeTask))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid task id: "+err.Error())
		return
	}

	blockers, err := h.tasks.GetBlockers(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get blockers: "+err.Error())
		return
	}

	if blockers == nil {
		blockers = []*models.Task{}
	}
	respondJSON(w, http.StatusOK, blockers)
}

// addDependency handles POST /api/tasks/{id}/dependencies
func (h *handlers) addDependency(w http.ResponseWriter, r *http.Request) {
	id, err := h.resolveIDParam(r, "id", string(models.WorkItemTypeTask))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid task id: "+err.Error())
		return
	}

	var req addDependencyRequest
	if err := decodeJSON(r, w, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.DependsOnTaskID == "" {
		respondError(w, http.StatusBadRequest, "depends_on_task_id is required")
		return
	}

	if err := h.tasks.AddDependency(r.Context(), id, req.DependsOnTaskID); err != nil {
		if errors.Is(err, store.ErrSelfDependency) || errors.Is(err, store.ErrCycleDetected) {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to add dependency: "+err.Error())
		return
	}

	// Return updated dependencies list.
	deps, err := h.tasks.GetDependencies(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get dependencies: "+err.Error())
		return
	}
	if deps == nil {
		deps = []string{}
	}

	respondJSON(w, http.StatusOK, map[string]any{"task_id": id, "dependencies": deps})
}

// removeDependency handles DELETE /api/tasks/{id}/dependencies/{dependsOnId}
func (h *handlers) removeDependency(w http.ResponseWriter, r *http.Request) {
	id, err := h.resolveIDParam(r, "id", string(models.WorkItemTypeTask))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid task id: "+err.Error())
		return
	}

	dependsOnID := chi.URLParam(r, "dependsOnId")
	if dependsOnID == "" {
		respondError(w, http.StatusBadRequest, "missing depends_on id")
		return
	}

	if err := h.tasks.RemoveDependency(r.Context(), id, dependsOnID); err != nil {
		errMsg := err.Error()
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, errMsg)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to remove dependency: "+errMsg)
		return
	}

	// Return updated dependencies list.
	deps, err := h.tasks.GetDependencies(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get dependencies: "+err.Error())
		return
	}
	if deps == nil {
		deps = []string{}
	}

	respondJSON(w, http.StatusOK, map[string]any{"task_id": id, "dependencies": deps})
}

// getTaskActivity handles GET /api/tasks/{id}/activity
func (h *handlers) getTaskActivity(w http.ResponseWriter, r *http.Request) {
	id, err := h.resolveIDParam(r, "id", string(models.WorkItemTypeTask))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid task id: "+err.Error())
		return
	}

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 500 {
		limit = 500
	}

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	entries, err := h.activity.GetByWorkItem(r.Context(), id, models.WorkItemTypeTask, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get activity: "+err.Error())
		return
	}

	if entries == nil {
		entries = []*models.ActivityLogEntry{}
	}
	respondJSON(w, http.StatusOK, entries)
}

// deleteTask handles DELETE /api/tasks/{id}
func (h *handlers) deleteTask(w http.ResponseWriter, r *http.Request) {
	id, err := h.resolveIDParam(r, "id", string(models.WorkItemTypeTask))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid task id: "+err.Error())
		return
	}

	task, err := h.tasks.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get task: "+err.Error())
		return
	}

	if task.Status != models.StatusNew {
		respondError(w, http.StatusBadRequest, "only tasks in 'new' status can be deleted")
		return
	}

	if err := h.tasks.Delete(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to delete task: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
