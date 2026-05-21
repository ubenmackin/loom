package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/models"
)

// --- Request/Response types ---

type createCommentRequest struct {
	AuthorID   string `json:"author_id"`
	AuthorType string `json:"author_type"`
	Body       string `json:"body"`
}

type updateCommentRequest struct {
	Body string `json:"body"`
}

// --- Route registration ---

func (h *handlers) registerCommentRoutes(r chi.Router) {
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/comments", h.getComments)
		r.Post("/comments", h.createComment)
	})
	r.Route("/comments/{id}", func(r chi.Router) {
		r.Put("/", h.updateComment)
		r.Delete("/", h.deleteComment)
	})
}

// --- Handlers ---

// getComments handles GET /api/work-items/{id}/comments
// The "type" query parameter specifies the work item type ("story" or "task").
func (h *handlers) getComments(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	workItemType := r.URL.Query().Get("type")
	if workItemType == "" {
		workItemType = models.WorkItemTypeTask // default to task
	}

	if workItemType != models.WorkItemTypeStory && workItemType != models.WorkItemTypeTask {
		respondError(w, http.StatusBadRequest, "type must be 'story' or 'task'")
		return
	}

	resolvedID, err := h.resolveWorkItemID(r.Context(), id, workItemType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "work item not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid work item id: "+err.Error())
		return
	}
	id = resolvedID

	comments, err := h.comments.GetByWorkItem(r.Context(), id, workItemType)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get comments: "+err.Error())
		return
	}

	if comments == nil {
		comments = []*models.Comment{}
	}
	respondJSON(w, http.StatusOK, comments)
}

// createComment handles POST /api/work-items/{id}/comments
func (h *handlers) createComment(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	workItemType := r.URL.Query().Get("type")
	if workItemType == "" {
		workItemType = models.WorkItemTypeTask
	}

	if workItemType != models.WorkItemTypeStory && workItemType != models.WorkItemTypeTask {
		respondError(w, http.StatusBadRequest, "type must be 'story' or 'task'")
		return
	}

	resolvedID, err := h.resolveWorkItemID(r.Context(), id, workItemType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "work item not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid work item id: "+err.Error())
		return
	}
	id = resolvedID

	var req createCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.AuthorID == "" {
		respondError(w, http.StatusBadRequest, "author_id is required")
		return
	}
	if req.AuthorType == "" {
		respondError(w, http.StatusBadRequest, "author_type is required")
		return
	}
	if req.Body == "" {
		respondError(w, http.StatusBadRequest, "body is required")
		return
	}

	comment := &models.Comment{
		WorkItemID:   id,
		WorkItemType: workItemType,
		AuthorID:     req.AuthorID,
		AuthorType:   req.AuthorType,
		Body:         req.Body,
	}

	if err := h.comments.Create(r.Context(), comment); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create comment: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, comment)
}

// updateComment handles PUT /api/comments/{id}
func (h *handlers) updateComment(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "missing comment id")
		return
	}

	existing, err := h.comments.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "comment not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get comment: "+err.Error())
		return
	}

	var req updateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Body == "" {
		respondError(w, http.StatusBadRequest, "body is required")
		return
	}

	// Extract author identity from context or request.
	// Use X-Session-ID header or a query param for agent authors.
	// For human authors, the author_id should be provided in the request.
	sessionID := GetSessionID(r)
	authorID := sessionID
	if authorID == "" {
		// Fallback: check for an author_id query parameter.
		authorID = r.URL.Query().Get("author_id")
	}

	if authorID == "" {
		respondError(w, http.StatusBadRequest, "author identity required (X-Session-ID header or author_id query param)")
		return
	}

	existing.Body = req.Body
	existing.AuthorID = authorID // Set for the store layer authorization check.

	if err := h.comments.Update(r.Context(), existing); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "only the author") {
			respondError(w, http.StatusForbidden, errMsg)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update comment: "+errMsg)
		return
	}

	respondJSON(w, http.StatusOK, existing)
}

// deleteComment handles DELETE /api/comments/{id}
func (h *handlers) deleteComment(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "missing comment id")
		return
	}

	// Extract author identity from context or request.
	sessionID := GetSessionID(r)
	authorID := sessionID
	if authorID == "" {
		authorID = r.URL.Query().Get("author_id")
	}

	if authorID == "" {
		respondError(w, http.StatusBadRequest, "author identity required (X-Session-ID header or author_id query param)")
		return
	}

	if err := h.comments.Delete(r.Context(), id, authorID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "comment not found")
			return
		}
		errMsg := err.Error()
		if strings.Contains(errMsg, "only the author") {
			respondError(w, http.StatusForbidden, errMsg)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to delete comment: "+errMsg)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
