package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/models"
)

// --- Request/Response types ---

type registerSessionRequest struct {
	ID           string `json:"id,omitempty"`
	HarnessType  string `json:"harness_type"`
	Capabilities string `json:"capabilities,omitempty"`
	Metadata     string `json:"metadata,omitempty"`
}

// --- Route registration ---

func (h *handlers) registerSessionRoutes(r chi.Router) {
	r.Post("/register", h.registerSession)
	r.Get("/", h.listSessions)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.getSession)
		r.Delete("/", h.disconnectSession)
		r.Get("/tasks", h.getSessionTasks)
		r.Get("/unread-comments", h.getUnreadComments)
	})
}

// --- Handlers ---

// registerSession handles POST /api/sessions/register
func (h *handlers) registerSession(w http.ResponseWriter, r *http.Request) {
	var req registerSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.HarnessType == "" {
		respondError(w, http.StatusBadRequest, "harness_type is required")
		return
	}

	session := &models.Session{
		ID:           req.ID,
		HarnessType:  req.HarnessType,
		Capabilities: req.Capabilities,
		Metadata:     req.Metadata,
	}

	if err := h.sessions.Register(r.Context(), session); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to register session: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, session)
}

// getSession handles GET /api/sessions/{id}
func (h *handlers) getSession(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "missing session id")
		return
	}

	session, err := h.sessions.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "session not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get session: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, session)
}

// disconnectSession handles DELETE /api/sessions/{id}
func (h *handlers) disconnectSession(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "missing session id")
		return
	}

	if err := h.sessions.Disconnect(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "session not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to disconnect session: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// getSessionTasks handles GET /api/sessions/{id}/tasks
func (h *handlers) getSessionTasks(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "missing session id")
		return
	}

	tasks, err := h.sessions.GetTasksForSession(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get session tasks: "+err.Error())
		return
	}

	if tasks == nil {
		tasks = []*models.Task{}
	}
	respondJSON(w, http.StatusOK, tasks)
}

// getUnreadComments handles GET /api/sessions/{id}/unread-comments
func (h *handlers) getUnreadComments(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "missing session id")
		return
	}

	comments, err := h.comments.GetUnreadForSession(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get unread comments: "+err.Error())
		return
	}

	if comments == nil {
		comments = []*models.Comment{}
	}
	respondJSON(w, http.StatusOK, comments)
}

// listSessions handles GET /api/sessions — returns all sessions.
func (h *handlers) listSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.sessions.ListAll(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list sessions: "+err.Error())
		return
	}

	if sessions == nil {
		sessions = []*models.Session{}
	}
	respondJSON(w, http.StatusOK, sessions)
}
