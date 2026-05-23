package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
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
func (h *handlers) getComments(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "missing work item id")
		return
	}

	workItemType := r.URL.Query().Get("type")
	if workItemType == "" {
		workItemType = string(models.WorkItemTypeTask)
	}

	if !validWorkItemType(workItemType) {
		respondError(w, http.StatusBadRequest, "type must be 'story' or 'task'")
		return
	}

	resolvedID, err := h.resolveWorkItemID(r.Context(), id, workItemType)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "work item not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid work item id: "+err.Error())
		return
	}
	id = resolvedID

	comments, err := h.comments.GetByWorkItem(r.Context(), id, models.WorkItemType(workItemType))
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
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "missing work item id")
		return
	}

	workItemType := r.URL.Query().Get("type")
	if workItemType == "" {
		workItemType = string(models.WorkItemTypeTask)
	}

	if !validWorkItemType(workItemType) {
		respondError(w, http.StatusBadRequest, "type must be 'story' or 'task'")
		return
	}

	resolvedID, err := h.resolveWorkItemID(r.Context(), id, workItemType)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "work item not found")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid work item id: "+err.Error())
		return
	}
	id = resolvedID

	var req createCommentRequest
	if err := decodeJSON(r, w, &req); err != nil {
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
		WorkItemType: models.WorkItemType(workItemType),
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
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "missing comment id")
		return
	}

	existing, err := h.comments.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "comment not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get comment: "+err.Error())
		return
	}

	var req updateCommentRequest
	if err := decodeJSON(r, w, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Body == "" {
		respondError(w, http.StatusBadRequest, "body is required")
		return
	}

	sessionID := GetSessionID(r)
	authorID := sessionID
	if authorID == "" {
		authorID = r.URL.Query().Get("author_id")
	}

	if authorID == "" {
		respondError(w, http.StatusBadRequest, "author identity required (X-Session-ID header or author_id query param)")
		return
	}

	existing.Body = req.Body
	if existing.AuthorID != authorID {
		respondError(w, http.StatusForbidden, store.ErrUnauthorizedAuthor.Error())
		return
	}

	if err := h.comments.Update(r.Context(), existing); err != nil {
		if errors.Is(err, store.ErrUnauthorizedAuthor) {
			respondError(w, http.StatusForbidden, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update comment: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, existing)
}

// deleteComment handles DELETE /api/comments/{id}
func (h *handlers) deleteComment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "missing comment id")
		return
	}

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
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "comment not found")
			return
		}
		if errors.Is(err, store.ErrUnauthorizedAuthor) {
			respondError(w, http.StatusForbidden, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to delete comment: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
