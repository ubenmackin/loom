package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/ubenmackin/loom/internal/models"
)

func (h *handlers) registerActivityRoutes(r chi.Router) {
	r.Get("/", h.listActivity)
}

// listActivity handles GET /api/activity
func (h *handlers) listActivity(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 500 {
		limit = 500
	}

	entries, err := h.activity.GetRecent(r.Context(), limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch activity: "+err.Error())
		return
	}
	if entries == nil {
		entries = []*models.ActivityLogEntry{}
	}

	respondJSON(w, http.StatusOK, entries)
}
