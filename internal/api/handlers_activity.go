package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ubenmackin/loom/internal/models"
)

// activityLogEntryResponse enriches an ActivityLogEntry with resolved
// work item title and project name for display in the UI.
type activityLogEntryResponse struct {
	models.ActivityLogEntry
	WorkItemTitle string `json:"work_item_title"`
	ProjectName   string `json:"project_name"`
}

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

	enriched := h.enrichActivityEntries(r.Context(), entries)
	respondJSON(w, http.StatusOK, enriched)
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

	enriched := h.enrichActivityEntries(r.Context(), entries)
	respondJSON(w, http.StatusOK, enriched)
}

// enrichActivityEntries batch-resolves work item titles and project names
// for a set of activity log entries. Lookup failures are logged but do not
// block the response — unresolved fields remain empty strings.
func (h *handlers) enrichActivityEntries(ctx context.Context, entries []*models.ActivityLogEntry) []activityLogEntryResponse {
	out := make([]activityLogEntryResponse, 0, len(entries))

	for _, entry := range entries {
		resp := activityLogEntryResponse{ActivityLogEntry: *entry}

		// Resolve work item title.
		switch entry.WorkItemType {
		case models.WorkItemTypeStory:
			story, err := h.stories.GetByID(ctx, entry.WorkItemID)
			if err != nil {
				slog.Debug("activity: failed to load story for enrichment",
					"entry_id", entry.ID, "work_item_id", entry.WorkItemID, "error", err)
			} else if story != nil {
				resp.WorkItemTitle = story.Title
			}
		case models.WorkItemTypeTask:
			task, err := h.tasks.GetByID(ctx, entry.WorkItemID)
			if err != nil {
				slog.Debug("activity: failed to load task for enrichment",
					"entry_id", entry.ID, "work_item_id", entry.WorkItemID, "error", err)
			} else if task != nil {
				resp.WorkItemTitle = task.Title
			}
		}

		// Resolve project name.
		if entry.ProjectID != "" {
			project, err := h.projects.GetByID(ctx, entry.ProjectID)
			if err != nil {
				slog.Debug("activity: failed to load project for enrichment",
					"entry_id", entry.ID, "project_id", entry.ProjectID, "error", err)
			} else if project != nil {
				resp.ProjectName = project.Name
			}
		}

		out = append(out, resp)
	}

	return out
}
