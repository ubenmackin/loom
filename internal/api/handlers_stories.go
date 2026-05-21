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

type createStoryRequest struct {
	Title          string `json:"title"`
	Description    string `json:"description,omitempty"`
	Priority       int    `json:"priority,omitempty"`
	RequiresBuild  bool   `json:"requires_build,omitempty"`
	RequiresReview bool   `json:"requires_review,omitempty"`
	AssignedTo     string `json:"assigned_to,omitempty"`
	AssigneeType   string `json:"assignee_type,omitempty"`
	SortOrder      int    `json:"sort_order,omitempty"`
}

type updateStoryRequest struct {
	Title          *string `json:"title,omitempty"`
	Description    *string `json:"description,omitempty"`
	Priority       *int    `json:"priority,omitempty"`
	RequiresBuild  *bool   `json:"requires_build,omitempty"`
	RequiresReview *bool   `json:"requires_review,omitempty"`
	AssignedTo     *string `json:"assigned_to,omitempty"`
	AssigneeType   *string `json:"assignee_type,omitempty"`
	SortOrder      *int    `json:"sort_order,omitempty"`
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
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.getStory)
		r.Put("/", h.updateStory)
		r.Patch("/status", h.updateStoryStatus)
		r.Delete("/", h.deleteStory)
	})
}

// --- Handlers ---

// listStories handles GET /api/stories
func (h *handlers) listStories(w http.ResponseWriter, r *http.Request) {
	filter := store.StoryFilter{
		Status:     r.URL.Query().Get("status"),
		AssignedTo: r.URL.Query().Get("assigned_to"),
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Title == "" {
		respondError(w, http.StatusBadRequest, "title is required")
		return
	}

	story := &models.Story{
		Title:          req.Title,
		Description:    req.Description,
		Priority:       req.Priority,
		RequiresBuild:  req.RequiresBuild,
		RequiresReview: req.RequiresReview,
		AssignedTo:     req.AssignedTo,
		AssigneeType:   req.AssigneeType,
		SortOrder:      req.SortOrder,
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
	_ = h.activity.Log(r.Context(), &models.ActivityLogEntry{
		WorkItemID:   story.ID,
		WorkItemType: models.WorkItemTypeStory,
		Action:       "story_created",
		Details:      details,
	})

	respondJSON(w, http.StatusCreated, story)
}

// getStory handles GET /api/stories/{id}
func (h *handlers) getStory(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	resolvedID, err := h.resolveWorkItemID(r.Context(), id, models.WorkItemTypeStory)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "story not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid story id: "+err.Error())
		return
	}
	id = resolvedID

	story, tasks, err := h.stories.GetWithTasks(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
	id := parseID(r, "id")
	resolvedID, err := h.resolveWorkItemID(r.Context(), id, models.WorkItemTypeStory)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "story not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid story id: "+err.Error())
		return
	}
	id = resolvedID

	story, err := h.stories.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "story not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get story: "+err.Error())
		return
	}

	var req updateStoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Apply partial updates.
	if req.Title != nil {
		story.Title = *req.Title
	}
	if req.Description != nil {
		story.Description = *req.Description
	}
	if req.Priority != nil {
		story.Priority = *req.Priority
	}
	if req.RequiresBuild != nil {
		story.RequiresBuild = *req.RequiresBuild
	}
	if req.RequiresReview != nil {
		story.RequiresReview = *req.RequiresReview
	}
	if req.AssignedTo != nil {
		story.AssignedTo = *req.AssignedTo
	}
	if req.AssigneeType != nil {
		story.AssigneeType = *req.AssigneeType
	}
	if req.SortOrder != nil {
		story.SortOrder = *req.SortOrder
	}

	if err := h.stories.Update(r.Context(), story); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update story: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, story)
}

// updateStoryStatus handles PATCH /api/stories/{id}/status
func (h *handlers) updateStoryStatus(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	resolvedID, err := h.resolveWorkItemID(r.Context(), id, models.WorkItemTypeStory)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "story not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid story id: "+err.Error())
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

	if err := h.stories.UpdateStatus(r.Context(), id, req.Status); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "invalid transition") {
			respondError(w, http.StatusBadRequest, errMsg)
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "story not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update story status: "+errMsg)
		return
	}

	// Return updated story.
	story, err := h.stories.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get updated story: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, story)
}

// deleteStory handles DELETE /api/stories/{id}
func (h *handlers) deleteStory(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	resolvedID, err := h.resolveWorkItemID(r.Context(), id, models.WorkItemTypeStory)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "story not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid story id: "+err.Error())
		return
	}
	id = resolvedID

	if err := h.stories.Delete(r.Context(), id); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "cannot delete") {
			respondError(w, http.StatusBadRequest, errMsg)
			return
		}
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "story not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to delete story: "+errMsg)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
