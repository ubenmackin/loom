package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/dispatcher"
	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
)

// --- Request/Response types ---

type createTaskRequest struct {
	Title          string `json:"title"`
	Description    string `json:"description,omitempty"`
	TaskType       string `json:"task_type,omitempty"`
	AssignedTo     string `json:"assigned_to,omitempty"`
	AssigneeType   string `json:"assignee_type,omitempty"`
	AgentSessionID string `json:"agent_session_id,omitempty"`
	AgentType      string `json:"agent_type,omitempty"`
	SortOrder      int    `json:"sort_order,omitempty"`
	Instructions   string `json:"instructions,omitempty"`
}

type updateTaskRequest struct {
	Title          *string `json:"title,omitempty"`
	Description    *string `json:"description,omitempty"`
	Status         *string `json:"status,omitempty"`
	TaskType       *string `json:"task_type,omitempty"`
	AssignedTo     *string `json:"assigned_to,omitempty"`
	AssigneeType   *string `json:"assignee_type,omitempty"`
	AgentSessionID *string `json:"agent_session_id,omitempty"`
	AgentType      *string `json:"agent_type,omitempty"`
	SortOrder      *int    `json:"sort_order,omitempty"`
	Instructions   *string `json:"instructions,omitempty"`
	IsStale        *bool   `json:"is_stale,omitempty"`
}

type generateTaskItem struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	TaskType    string   `json:"task_type,omitempty"`  // defaults to "code"
	DependsOn   []string `json:"depends_on,omitempty"` // positional refs like "#1", or real task IDs
}

type generateTasksRequest struct {
	Tasks []generateTaskItem `json:"tasks"`
}

type generateTasksResponse struct {
	Tasks []*models.Task `json:"tasks"`
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
	storyID, ok := h.resolveAndRespond(w, r, "id", string(models.WorkItemTypeStory), "story")
	if !ok {
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
		StoryID:        storyID,
		Title:          strings.TrimSpace(req.Title),
		Description:    req.Description,
		TaskType:       models.TaskType(req.TaskType),
		AssignedTo:     req.AssignedTo,
		AssigneeType:   models.AssigneeType(req.AssigneeType),
		AgentSessionID: req.AgentSessionID,
		AgentType:      req.AgentType,
		SortOrder:      req.SortOrder,
		Instructions:   req.Instructions,
	}

	if err := h.tasks.Create(r.Context(), task); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create task: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, task)
}

// getTask handles GET /api/tasks/{id}
func (h *handlers) getTask(w http.ResponseWriter, r *http.Request) {
	id, ok := h.resolveAndRespond(w, r, "id", string(models.WorkItemTypeTask), "task")
	if !ok {
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
	id, ok := h.resolveAndRespond(w, r, "id", string(models.WorkItemTypeTask), "task")
	if !ok {
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
	if req.AgentSessionID != nil {
		task.AgentSessionID = *req.AgentSessionID
	}
	if req.AgentType != nil {
		task.AgentType = *req.AgentType
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
	id, ok := h.resolveAndRespond(w, r, "id", string(models.WorkItemTypeTask), "task")
	if !ok {
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

// getTaskActivity handles GET /api/tasks/{id}/activity
func (h *handlers) getTaskActivity(w http.ResponseWriter, r *http.Request) {
	h.getWorkItemActivity(w, r, models.WorkItemTypeTask, "task")
}

// generateTasks handles POST /api/stories/{id}/generate-tasks
func (h *handlers) generateTasks(w http.ResponseWriter, r *http.Request) {
	storyID, ok := h.resolveAndRespond(w, r, "id", string(models.WorkItemTypeStory), "story")
	if !ok {
		return
	}

	var req generateTasksRequest
	if err := decodeJSON(r, w, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Validate: at least one task provided.
	if len(req.Tasks) == 0 {
		respondError(w, http.StatusBadRequest, "at least one task is required")
		return
	}

	// Validate all tasks have titles and valid task types.
	for i, item := range req.Tasks {
		if strings.TrimSpace(item.Title) == "" {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("task[%d]: title is required", i))
			return
		}
		tt := item.TaskType
		if tt != "" && !validTaskType(tt) {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("task[%d]: invalid task_type %q", i, tt))
			return
		}
	}

	// Phase 1 & 2 — wrapped in a database transaction for atomicity.
	var created []*models.Task
	err := h.tasks.Transact(r.Context(), func(txCtx context.Context) error {
		created = make([]*models.Task, 0, len(req.Tasks))
		posToID := make(map[int]string) // positional index → real task ID

		// Phase 1: Create all tasks in order.
		for i, item := range req.Tasks {
			taskType := item.TaskType
			if taskType == "" {
				taskType = string(models.TaskTypeCode)
			}

			task := &models.Task{
				StoryID:     storyID,
				Title:       strings.TrimSpace(item.Title),
				Description: item.Description,
				TaskType:    models.TaskType(taskType),
				Status:      models.StatusNew,
			}

			if err := h.tasks.Create(txCtx, task); err != nil {
				return fmt.Errorf("failed to create task[%d]: %w", i, err)
			}

			posToID[i] = task.ID
			created = append(created, task)
		}

		// Phase 2: Resolve and add dependencies.
		for i, item := range req.Tasks {
			if len(item.DependsOn) == 0 {
				continue
			}
			taskID := posToID[i]
			for _, ref := range item.DependsOn {
				var depID string
				if strings.HasPrefix(ref, "#") {
					// Positional reference: "#1" means index 1 (0-based).
					idxStr := strings.TrimPrefix(ref, "#")
					idx, err := strconv.Atoi(idxStr)
					if err != nil {
						return fmt.Errorf("task[%d]: invalid positional dependency %q", i, ref)
					}
					var found bool
					depID, found = posToID[idx]
					if !found {
						return fmt.Errorf("task[%d]: positional dependency %q references non-existent task index", i, ref)
					}
				} else {
					// Treat as a real task ID (may reference existing tasks).
					depID = ref
				}

				if err := h.tasks.AddDependency(txCtx, taskID, depID); err != nil {
					return fmt.Errorf("task[%d]: failed to add dependency on %q: %w", i, depID, err)
				}
			}
		}

		return nil
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate tasks: "+err.Error())
		return
	}

	// Activity logging (outside transaction — separate store).
	for _, task := range created {
		currentUser := GetUser(r)
		details := "Created via batch generation"
		if currentUser != nil {
			details = "Created by user " + currentUser.Username + " via batch generation"
		}
		h.logActivity(r.Context(), &models.ActivityLogEntry{
			WorkItemID:   task.ID,
			WorkItemType: models.WorkItemTypeTask,
			Action:       "task_created",
			Details:      details,
		})
	}

	// Broadcast "tasks_generated" WebSocket event via the dispatcher.
	h.dispatch.Submit(r.Context(), dispatcher.Event{
		Type: dispatcher.EventTasksGenerated,
		Payload: map[string]any{
			"story_id": storyID,
			"count":    len(created),
		},
	})

	// Log a batch activity entry on the story.
	currentUser := GetUser(r)
	batchDetails := fmt.Sprintf("%d tasks generated", len(created))
	if currentUser != nil {
		batchDetails = "Batch of " + batchDetails + " by user " + currentUser.Username
	}
	h.logActivity(r.Context(), &models.ActivityLogEntry{
		WorkItemID:   storyID,
		WorkItemType: models.WorkItemTypeStory,
		Action:       "tasks_generated",
		Details:      batchDetails,
	})

	respondJSON(w, http.StatusCreated, generateTasksResponse{Tasks: created})
}

// deleteTask handles DELETE /api/tasks/{id}
func (h *handlers) deleteTask(w http.ResponseWriter, r *http.Request) {
	id, ok := h.resolveAndRespond(w, r, "id", string(models.WorkItemTypeTask), "task")
	if !ok {
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
