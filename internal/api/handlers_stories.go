package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/dispatcher"
	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
)

// --- Request/Response types ---

type createStoryRequest struct {
	Title            string `json:"title"`
	Description      string `json:"description,omitempty"`
	ProjectID        string `json:"project_id,omitempty"`
	RequiresBuild    bool   `json:"requires_build,omitempty"`
	RequiresReview   bool   `json:"requires_review,omitempty"`
	RequiresSecurity *bool  `json:"requires_security,omitempty"`
	AssignedTo       string `json:"assigned_to,omitempty"`
	AssigneeType     string `json:"assignee_type,omitempty"`
	SortOrder        int    `json:"sort_order,omitempty"`
}

type updateStoryRequest struct {
	Title            *string `json:"title,omitempty"`
	Description      *string `json:"description,omitempty"`
	ProjectID        *string `json:"project_id,omitempty"`
	RequiresBuild    *bool   `json:"requires_build,omitempty"`
	RequiresReview   *bool   `json:"requires_review,omitempty"`
	RequiresSecurity *bool   `json:"requires_security,omitempty"`
	BranchName       *string `json:"branch_name,omitempty"`
	AssignedTo       *string `json:"assigned_to,omitempty"`
	AssigneeType     *string `json:"assignee_type,omitempty"`
	SortOrder        *int    `json:"sort_order,omitempty"`
	Status           *string `json:"status,omitempty"`
}

type updateStatusRequest struct {
	Status string `json:"status"`
}

type storyWithTasksResponse struct {
	Story *models.Story  `json:"story"`
	Tasks []*models.Task `json:"tasks"`
}

// --- Route registration ---

func (h *handlers) registerStoryRoutes(r chi.Router) {
	r.Get("/", h.listStories)
	r.Post("/", h.createStory)
	r.Patch("/reorder", h.batchReorderStories)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.getStory)
		r.Put("/", h.updateStory)
		r.Patch("/status", h.setStoryStatus)
		r.Delete("/", h.deleteStory)
		r.Get("/activity", h.getStoryActivity)
		r.Post("/generate-tasks", h.generateTasks)
	})
}

// --- Handlers ---

// listStories handles GET /api/stories
func (h *handlers) listStories(w http.ResponseWriter, r *http.Request) {
	filter := store.StoryFilter{
		Status:     models.Status(r.URL.Query().Get("status")),
		AssignedTo: r.URL.Query().Get("assigned_to"),
		ProjectID:  r.URL.Query().Get("project_id"),
	}

	stories, err := h.stories.List(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list stories: "+err.Error())
		return
	}

	if stories == nil {
		stories = []*models.Story{}
	}
	respondJSON(w, http.StatusOK, stories)
}

// createStory handles POST /api/stories
func (h *handlers) createStory(w http.ResponseWriter, r *http.Request) {
	var req createStoryRequest
	if err := decodeJSON(r, w, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if strings.TrimSpace(req.Title) == "" {
		respondError(w, http.StatusBadRequest, "title is required")
		return
	}

	if req.AssigneeType != "" && !validAssigneeType(req.AssigneeType) {
		respondError(w, http.StatusBadRequest, "invalid assignee_type")
		return
	}

	story := &models.Story{
		Title:          strings.TrimSpace(req.Title),
		Description:    req.Description,
		ProjectID:      req.ProjectID,
		RequiresBuild:  req.RequiresBuild,
		RequiresReview: req.RequiresReview,
		AssignedTo:     req.AssignedTo,
		AssigneeType:   models.AssigneeType(req.AssigneeType),
		SortOrder:      req.SortOrder,
	}

	// Default requires_security to true when not explicitly specified by
	// the client so the Security gate remains opt-out rather than opt-in.
	if req.RequiresSecurity != nil {
		story.RequiresSecurity = *req.RequiresSecurity
	} else {
		story.RequiresSecurity = true
	}

	if err := h.stories.Create(r.Context(), story); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create story: "+err.Error())
		return
	}

	// Log activity automatically.
	currentUser := GetUser(r)
	details := "Created by user"
	if currentUser != nil {
		details = "Created by user " + currentUser.Username
	}
	h.logActivity(r.Context(), &models.ActivityLogEntry{
		WorkItemID:   story.ID,
		WorkItemType: models.WorkItemTypeStory,
		Action:       "story_created",
		Details:      details,
	})

	respondJSON(w, http.StatusCreated, story)
}

// getStory handles GET /api/stories/{id}
func (h *handlers) getStory(w http.ResponseWriter, r *http.Request) {
	id, ok := h.resolveAndRespond(w, r, "id", string(models.WorkItemTypeStory), "story")
	if !ok {
		return
	}

	story, tasks, err := h.stories.GetWithTasks(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "story not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get story: "+err.Error())
		return
	}

	if tasks == nil {
		tasks = []*models.Task{}
	}

	respondJSON(w, http.StatusOK, storyWithTasksResponse{
		Story: story,
		Tasks: tasks,
	})
}

// updateStory handles PUT /api/stories/{id}
func (h *handlers) updateStory(w http.ResponseWriter, r *http.Request) {
	id, ok := h.resolveAndRespond(w, r, "id", string(models.WorkItemTypeStory), "story")
	if !ok {
		return
	}

	story, err := h.stories.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "story not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get story: "+err.Error())
		return
	}

	// Capture old values before applying updates.
	oldTitle := story.Title
	oldDescription := story.Description
	oldProjectID := story.ProjectID
	oldRequiresBuild := story.RequiresBuild
	oldRequiresReview := story.RequiresReview
	oldRequiresSecurity := story.RequiresSecurity
	oldBranchName := story.BranchName
	oldAssignedTo := story.AssignedTo
	oldAssigneeType := string(story.AssigneeType)
	oldSortOrder := story.SortOrder
	oldStatus := string(story.Status)

	var req updateStoryRequest
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
		story.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		story.Description = *req.Description
	}
	if req.ProjectID != nil {
		story.ProjectID = *req.ProjectID
	}
	if req.RequiresBuild != nil {
		story.RequiresBuild = *req.RequiresBuild
	}
	if req.RequiresReview != nil {
		story.RequiresReview = *req.RequiresReview
	}
	if req.RequiresSecurity != nil {
		story.RequiresSecurity = *req.RequiresSecurity
	}
	if req.BranchName != nil {
		story.BranchName = *req.BranchName
	}
	if req.AssignedTo != nil {
		story.AssignedTo = *req.AssignedTo
	}
	if req.AssigneeType != nil {
		if !validAssigneeType(*req.AssigneeType) {
			respondError(w, http.StatusBadRequest, "invalid assignee_type")
			return
		}
		story.AssigneeType = models.AssigneeType(*req.AssigneeType)
	}
	if req.SortOrder != nil {
		story.SortOrder = *req.SortOrder
	}
	if req.Status != nil {
		if !validStatus(*req.Status) {
			respondError(w, http.StatusBadRequest, "invalid status value")
			return
		}
		story.Status = models.Status(*req.Status)
	}

	if err := h.stories.Update(r.Context(), story); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusConflict, "story was modified or deleted concurrently")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update story: "+err.Error())
		return
	}

	// Build a list of changed fields.
	var changed []string
	if story.Title != oldTitle {
		changed = append(changed, "title")
	}
	if story.Description != oldDescription {
		changed = append(changed, "description")
	}
	if story.ProjectID != oldProjectID {
		changed = append(changed, "project_id")
	}
	if story.RequiresBuild != oldRequiresBuild {
		changed = append(changed, "requires_build")
	}
	if story.RequiresReview != oldRequiresReview {
		changed = append(changed, "requires_review")
	}
	if story.RequiresSecurity != oldRequiresSecurity {
		changed = append(changed, "requires_security")
	}
	if story.BranchName != oldBranchName {
		changed = append(changed, "branch_name")
	}
	if story.AssignedTo != oldAssignedTo {
		changed = append(changed, "assigned_to")
	}
	if string(story.AssigneeType) != oldAssigneeType {
		changed = append(changed, "assignee_type")
	}
	if story.SortOrder != oldSortOrder {
		changed = append(changed, "sort_order")
	}
	if string(story.Status) != oldStatus {
		changed = append(changed, "status")
	}
	currentUser := GetUser(r)
	details := "Changed: " + strings.Join(changed, ", ")
	if currentUser != nil {
		details = "Updated by user " + currentUser.Username + ": " + details
	}
	h.logActivity(r.Context(), &models.ActivityLogEntry{
		WorkItemID:   story.ID,
		WorkItemType: models.WorkItemTypeStory,
		Action:       "story_updated",
		Details:      details,
	})

	respondJSON(w, http.StatusOK, story)
}

// deleteStory handles DELETE /api/stories/{id}
func (h *handlers) deleteStory(w http.ResponseWriter, r *http.Request) {
	id, ok := h.resolveAndRespond(w, r, "id", string(models.WorkItemTypeStory), "story")
	if !ok {
		return
	}

	if err := h.stories.Delete(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "story not found")
			return
		}
		if errors.Is(err, store.ErrInvalidTransition) {
			respondError(w, http.StatusBadRequest, "only stories in 'new' status can be deleted")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to delete story: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// getStoryActivity handles GET /api/stories/{id}/activity
func (h *handlers) getStoryActivity(w http.ResponseWriter, r *http.Request) {
	h.getWorkItemActivity(w, r, models.WorkItemTypeStory, "story")
}

// planStory handles POST /api/stories/{id}/plan
func (h *handlers) planStory(w http.ResponseWriter, r *http.Request) {
	id, ok := h.resolveAndRespond(w, r, "id", string(models.WorkItemTypeStory), "story")
	if !ok {
		return
	}

	story, err := h.stories.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "story not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get story: "+err.Error())
		return
	}

	if story.Status != models.StatusDraft {
		respondError(w, http.StatusBadRequest, "only draft stories can be planned; current status: "+string(story.Status))
		return
	}

	// Transition story to "planning" first so the database state is
	// consistent before any side effects occur.
	if err := h.stories.UpdateStatus(r.Context(), story.ID, models.StatusPlanning); err != nil {
		if errors.Is(err, store.ErrInvalidTransition) {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "story not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update story status: "+err.Error())
		return
	}

	// Trigger a planner ACP session via the gateway so the planning agent
	// can decompose the story into tasks.
	if h.gateway != nil {
		h.gateway.SubmitEvent(dispatcher.Event{
			Type: dispatcher.EventWorkRequested,
			Payload: map[string]interface{}{
				"project_id": story.ProjectID,
				"agent_type": "planner",
				"story_id":   story.ID,
			},
		})
	}

	story, err = h.stories.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get updated story: "+err.Error())
		return
	}

	currentUser := GetUser(r)
	details := "Planning started"
	if currentUser != nil {
		details = "Planning started by user " + currentUser.Username
	}
	h.logActivity(r.Context(), &models.ActivityLogEntry{
		WorkItemID:   story.ID,
		WorkItemType: models.WorkItemTypeStory,
		Action:       "story_planning_started",
		Details:      details,
	})

	respondJSON(w, http.StatusOK, story)
}

// setStoryStatus handles PATCH /api/stories/{id}/status
func (h *handlers) setStoryStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := h.resolveAndRespond(w, r, "id", string(models.WorkItemTypeStory), "story")
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

	story, err := h.stories.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "story not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get story: "+err.Error())
		return
	}

	oldStatus := story.Status
	newStatus := models.Status(req.Status)

	// Validate the transition against the shared model-level rules.
	if !models.IsValidTransition(oldStatus, newStatus) {
		respondError(w, http.StatusBadRequest,
			fmt.Sprintf("invalid story status transition: %q → %q", oldStatus, newStatus))
		return
	}

	if err := h.stories.UpdateStatus(r.Context(), story.ID, newStatus); err != nil {
		if errors.Is(err, store.ErrInvalidTransition) {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "story not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update story status: "+err.Error())
		return
	}

	// When transitioning to "ready", push all dependency-free tasks to ready
	// and submit work requests so the dispatcher can assign them.
	if newStatus == models.StatusReady {
		tasks, err := h.tasks.GetByStory(r.Context(), story.ID)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to list story tasks: "+err.Error())
			return
		}

		depsForTasks, err := h.tasks.GetBlockersForTasks(r.Context(), taskIDs(tasks))
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to get task blockers: "+err.Error())
			return
		}

		for _, task := range tasks {
			if task.Status != models.StatusNew && task.Status != models.StatusReady {
				continue
			}
			if blockers, hasBlockers := depsForTasks[task.ID]; hasBlockers && len(blockers) > 0 {
				continue
			}
			if task.Status != models.StatusReady {
				if err := h.tasks.UpdateStatus(r.Context(), task.ID, models.StatusReady); err != nil {
					respondError(w, http.StatusInternalServerError, "failed to update task status: "+err.Error())
					return
				}
			}
			if h.dispatch != nil {
				h.dispatch.Submit(r.Context(), dispatcher.Event{
					Type:    dispatcher.EventWorkRequested,
					TaskID:  task.ID,
					Payload: map[string]string{"story_id": story.ID},
				})
			}
		}
	}

	story, err = h.stories.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get updated story: "+err.Error())
		return
	}

	currentUser := GetUser(r)
	details := string(oldStatus) + " → " + string(newStatus)
	if currentUser != nil {
		details = "Status changed by user " + currentUser.Username + ": " + details
	}
	h.logActivity(r.Context(), &models.ActivityLogEntry{
		WorkItemID:   story.ID,
		WorkItemType: models.WorkItemTypeStory,
		Action:       "status_changed",
		Details:      details,
	})

	respondJSON(w, http.StatusOK, story)
}

// resetStoryFailures handles POST /api/stories/{id}/reset-failures
// Resets the story's failure_count to 0 and transitions it to "ready" status
// so it can re-enter the pipeline after a circuit breaker trip.
func (h *handlers) resetStoryFailures(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	story, err := h.stories.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "story not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get story: "+err.Error())
		return
	}

	// Guard: only allow reset when the story has actually tripped the circuit
	// breaker (status is "failed"). Without this check, an admin resetting a
	// story that is still in_progress (failure_count below the breaker
	// threshold) would have failure_count committed to 0 by Update() below,
	// and then UpdateStatus() would return ErrInvalidTransition — leaving the
	// DB in a partially-updated state while returning an error to the client.
	if story.Status != models.StatusFailed {
		respondError(w, http.StatusConflict, "story is not in failed status (current="+string(story.Status)+")")
		return
	}

	story.FailureCount = 0
	if err := h.stories.Update(r.Context(), story); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to reset story failures: "+err.Error())
		return
	}

	// Transition to ready using state machine validation.
	if err := h.stories.UpdateStatus(r.Context(), id, models.StatusReady); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "story not found")
			return
		}
		if errors.Is(err, store.ErrInvalidTransition) {
			respondError(w, http.StatusBadRequest, "invalid status transition: "+err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to transition story: "+err.Error())
		return
	}

	// Re-fetch the story to return the post-transition state (status="ready",
	// failure_count=0) rather than the pre-transition in-memory copy.
	updatedStory, err := h.stories.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch updated story: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, updatedStory)
}

// taskIDs extracts the string IDs from a slice of tasks for bulk queries.
func taskIDs(tasks []*models.Task) []string {
	ids := make([]string, 0, len(tasks))
	for _, t := range tasks {
		ids = append(ids, t.ID)
	}
	return ids
}
