package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
)

// --- Request/Response types ---

type createTaskRequest struct {
	Title        string `json:"title"`
	Description  string `json:"description,omitempty"`
	Priority     int    `json:"priority,omitempty"`
	TaskType     string `json:"task_type,omitempty"`
	Estimate     *int   `json:"estimate,omitempty"`
	AssignedTo   string `json:"assigned_to,omitempty"`
	AssigneeType string `json:"assignee_type,omitempty"`
	SortOrder    int    `json:"sort_order,omitempty"`
	Context      string `json:"context,omitempty"`
	Instructions string `json:"instructions,omitempty"`
}

type updateTaskRequest struct {
	Title        *string `json:"title,omitempty"`
	Description  *string `json:"description,omitempty"`
	Priority     *int    `json:"priority,omitempty"`
	TaskType     *string `json:"task_type,omitempty"`
	Estimate     *int    `json:"estimate,omitempty"`
	AssignedTo   *string `json:"assigned_to,omitempty"`
	AssigneeType *string `json:"assignee_type,omitempty"`
	SortOrder    *int    `json:"sort_order,omitempty"`
	Context      *string `json:"context,omitempty"`
	Instructions *string `json:"instructions,omitempty"`
	IsStale      *bool   `json:"is_stale,omitempty"`
}

type addDependencyRequest struct {
	DependsOnTaskID string `json:"depends_on_task_id"`
}

type taskDetailResponse struct {
	Task         *models.Task `json:"task"`
	Dependencies []string     `json:"dependencies"`
}

// --- Route registration ---

func (h *handlers) registerTaskRoutes(r chi.Router) {
	r.Get("/", h.listTasks)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.getTask)
		r.Put("/", h.updateTask)
		r.Patch("/status", h.updateTaskStatus)
		r.Get("/blockers", h.getTaskBlockers)
		r.Post("/dependencies", h.addDependency)
		r.Delete("/dependencies/{dependsOnId}", h.removeDependency)
		r.Get("/activity", h.getTaskActivity)
	})
}

// --- Handlers ---

// listTasks handles GET /api/tasks
func (h *handlers) listTasks(w http.ResponseWriter, r *http.Request) {
	filter := store.TaskFilter{
		StoryID:    r.URL.Query().Get("story_id"),
		Status:     r.URL.Query().Get("status"),
		AssignedTo: r.URL.Query().Get("assigned_to"),
		TaskType:   r.URL.Query().Get("task_type"),
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

// createTaskUnderStory handles POST /api/stories/{storyId}/tasks
func (h *handlers) createTaskUnderStory(w http.ResponseWriter, r *http.Request) {
	storyID := parseID(r, "storyId")
	if storyID == "" {
		respondError(w, http.StatusBadRequest, "missing story id")
		return
	}

	// Verify story exists.
	if _, err := h.stories.GetByID(r.Context(), storyID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "story not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get story: "+err.Error())
		return
	}

	var req createTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Title == "" {
		respondError(w, http.StatusBadRequest, "title is required")
		return
	}

	if req.TaskType == "" {
		req.TaskType = models.TaskTypeCode
	}

	task := &models.Task{
		StoryID:      storyID,
		Title:        req.Title,
		Description:  req.Description,
		Priority:     req.Priority,
		TaskType:     req.TaskType,
		Estimate:     req.Estimate,
		AssignedTo:   req.AssignedTo,
		AssigneeType: req.AssigneeType,
		SortOrder:    req.SortOrder,
		Context:      req.Context,
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
	id := parseID(r, "id")
	resolvedID, err := h.resolveWorkItemID(r.Context(), id, models.WorkItemTypeTask)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid task id: "+err.Error())
		return
	}
	id = resolvedID

	task, err := h.tasks.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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

	respondJSON(w, http.StatusOK, taskDetailResponse{
		Task:         task,
		Dependencies: deps,
	})
}

// updateTask handles PUT /api/tasks/{id}
func (h *handlers) updateTask(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	resolvedID, err := h.resolveWorkItemID(r.Context(), id, models.WorkItemTypeTask)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid task id: "+err.Error())
		return
	}
	id = resolvedID

	task, err := h.tasks.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get task: "+err.Error())
		return
	}

	var req updateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Apply partial updates.
	if req.Title != nil {
		task.Title = *req.Title
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.Priority != nil {
		task.Priority = *req.Priority
	}
	if req.TaskType != nil {
		task.TaskType = *req.TaskType
	}
	if req.Estimate != nil {
		task.Estimate = req.Estimate
	}
	if req.AssignedTo != nil {
		task.AssignedTo = *req.AssignedTo
	}
	if req.AssigneeType != nil {
		task.AssigneeType = *req.AssigneeType
	}
	if req.SortOrder != nil {
		task.SortOrder = *req.SortOrder
	}
	if req.Context != nil {
		task.Context = *req.Context
	}
	if req.Instructions != nil {
		task.Instructions = *req.Instructions
	}
	if req.IsStale != nil {
		task.IsStale = *req.IsStale
	}

	if err := h.tasks.Update(r.Context(), task); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update task: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, task)
}

// updateTaskStatus handles PATCH /api/tasks/{id}/status
func (h *handlers) updateTaskStatus(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	resolvedID, err := h.resolveWorkItemID(r.Context(), id, models.WorkItemTypeTask)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid task id: "+err.Error())
		return
	}
	id = resolvedID

	var req updateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Status == "" {
		respondError(w, http.StatusBadRequest, "status is required")
		return
	}

	if err := h.tasks.UpdateStatus(r.Context(), id, req.Status); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "invalid transition") {
			respondError(w, http.StatusBadRequest, errMsg)
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update task status: "+errMsg)
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

// getTaskBlockers handles GET /api/tasks/{id}/blockers
func (h *handlers) getTaskBlockers(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	resolvedID, err := h.resolveWorkItemID(r.Context(), id, models.WorkItemTypeTask)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid task id: "+err.Error())
		return
	}
	id = resolvedID

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
	id := parseID(r, "id")
	resolvedID, err := h.resolveWorkItemID(r.Context(), id, models.WorkItemTypeTask)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid task id: "+err.Error())
		return
	}
	id = resolvedID

	var req addDependencyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.DependsOnTaskID == "" {
		respondError(w, http.StatusBadRequest, "depends_on_task_id is required")
		return
	}

	if err := h.tasks.AddDependency(r.Context(), id, req.DependsOnTaskID); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "cannot depend on itself") || strings.Contains(errMsg, "cycle") {
			respondError(w, http.StatusBadRequest, errMsg)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to add dependency: "+errMsg)
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
	id := parseID(r, "id")
	resolvedID, err := h.resolveWorkItemID(r.Context(), id, models.WorkItemTypeTask)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid task id: "+err.Error())
		return
	}
	id = resolvedID

	dependsOnID := parseID(r, "dependsOnId")
	if dependsOnID == "" {
		respondError(w, http.StatusBadRequest, "missing depends_on id")
		return
	}

	if err := h.tasks.RemoveDependency(r.Context(), id, dependsOnID); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") {
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
	id := parseID(r, "id")
	resolvedID, err := h.resolveWorkItemID(r.Context(), id, models.WorkItemTypeTask)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "task not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid task id: "+err.Error())
		return
	}
	id = resolvedID

	entries, err := h.activity.GetByWorkItem(r.Context(), id, models.WorkItemTypeTask, 50, 0)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get activity: "+err.Error())
		return
	}

	if entries == nil {
		entries = []*models.ActivityLogEntry{}
	}
	respondJSON(w, http.StatusOK, entries)
}
