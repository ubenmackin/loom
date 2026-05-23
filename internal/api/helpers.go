package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

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
