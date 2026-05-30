package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
)

// resolveIDParam extracts and resolves an ID URL parameter, supporting both
// numeric IDs (which are resolved to UUID-style IDs via resolveWorkItemID)
// and direct string IDs.
func (h *handlers) resolveIDParam(r *http.Request, param string, itemType string) (string, error) {
	id := chi.URLParam(r, param)
	if id == "" {
		return "", fmt.Errorf("missing %s parameter", param)
	}
	resolved, err := h.resolveWorkItemID(r.Context(), id, itemType)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return "", err
		}
		return "", fmt.Errorf("resolve %s: %w", itemType, err)
	}
	return resolved, nil
}

// Pagination holds parsed limit/offset query parameters.
type Pagination struct {
	Limit  int
	Offset int
}

// parsePagination extracts and clamps limit/offset from query parameters.
func parsePagination(r *http.Request, defaultLimit, maxLimit int) Pagination {
	limit := defaultLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return Pagination{Limit: limit, Offset: offset}
}

// resolveAndRespond resolves an ID URL parameter and sends an error response
// on failure, returning the resolved ID and true on success.
func (h *handlers) resolveAndRespond(w http.ResponseWriter, r *http.Request, param, itemType, label string) (string, bool) {
	id, err := h.resolveIDParam(r, param, itemType)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, label+" not found")
		} else {
			respondError(w, http.StatusBadRequest, "invalid "+label+" id: "+err.Error())
		}
		return "", false
	}
	return id, true
}

// getWorkItemActivity handles GET /api/{type}/{id}/activity for any work item type.
func (h *handlers) getWorkItemActivity(w http.ResponseWriter, r *http.Request, itemType models.WorkItemType, label string) {
	id, ok := h.resolveAndRespond(w, r, "id", string(itemType), label)
	if !ok {
		return
	}

	p := parsePagination(r, 50, 500)

	entries, err := h.activity.GetByWorkItem(r.Context(), id, itemType, p.Limit, p.Offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get activity: "+err.Error())
		return
	}

	if entries == nil {
		entries = []*models.ActivityLogEntry{}
	}
	respondJSON(w, http.StatusOK, entries)
}

// logActivity safely logs an activity entry, logging errors via slog.
func (h *handlers) logActivity(ctx context.Context, entry *models.ActivityLogEntry) {
	if err := h.activity.Log(ctx, entry); err != nil {
		slog.Error("activity log failed", "error", err)
	}
}

// decodeJSON reads the request body with a 1MB limit and decodes JSON.
func decodeJSON(r *http.Request, w http.ResponseWriter, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	return json.NewDecoder(r.Body).Decode(v)
}

// validStatus returns true if the status is a known value.
func validStatus(s string) bool {
	switch models.Status(s) {
	case models.StatusNew, models.StatusReady, models.StatusInProgress,
		models.StatusBlocked, models.StatusDone, models.StatusCancelled,
		models.StatusArchived:
		return true
	}
	return false
}

// validTaskType returns true if the task type is a known value.
func validTaskType(s string) bool {
	switch models.TaskType(s) {
	case models.TaskTypeCode, models.TaskTypeBuild, models.TaskTypeReview:
		return true
	}
	return false
}

// validAssigneeType returns true if the assignee type is a known value.
func validAssigneeType(s string) bool {
	switch models.AssigneeType(s) {
	case models.AssigneeTypeHuman, models.AssigneeTypeSession:
		return true
	}
	return false
}

// validWorkItemType returns true if the work item type is a known value.
func validWorkItemType(s string) bool {
	switch models.WorkItemType(s) {
	case models.WorkItemTypeStory, models.WorkItemTypeTask:
		return true
	}
	return false
}
