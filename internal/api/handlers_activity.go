package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ubenmackin/loom/internal/models"
)

func (h *handlers) registerActivityRoutes(r chi.Router) {
	r.Get("/", h.listActivity)
	r.Get("/dispatcher", h.listDispatcherActivity)
}

// listActivity handles GET /api/activity
func (h *handlers) listActivity(w http.ResponseWriter, r *http.Request) {
	p := parsePagination(r, 100, 500)

	entries, err := h.activity.GetRecent(r.Context(), p.Limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch activity: "+err.Error())
		return
	}
	if entries == nil {
		entries = []*models.ActivityLogEntry{}
	}

	respondJSON(w, http.StatusOK, entries)
}

// listDispatcherActivity handles GET /api/activity/dispatcher
// Returns only dispatcher-generated actions (assigned, gate_created, marked_stale, unblocked).
func (h *handlers) listDispatcherActivity(w http.ResponseWriter, r *http.Request) {
	p := parsePagination(r, 100, 500)

	entries, err := h.activity.GetByAction(r.Context(), p.Limit, p.Offset, "assigned", "gate_created", "marked_stale", "unblocked")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch dispatcher activity: "+err.Error())
		return
	}
	if entries == nil {
		entries = []*models.ActivityLogEntry{}
	}

	respondJSON(w, http.StatusOK, entries)
}
