package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
)

// --- Request types ---

type addDependencyRequest struct {
	DependsOnTaskID string `json:"depends_on_task_id"`
}

// --- Handlers ---

// getTaskBlockers handles GET /api/tasks/{id}/blockers
func (h *handlers) getTaskBlockers(w http.ResponseWriter, r *http.Request) {
	id, ok := h.resolveAndRespond(w, r, "id", string(models.WorkItemTypeTask), "task")
	if !ok {
		return
	}

	blockers, err := h.tasks.GetBlockers(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get blockers: "+err.Error())
		return
	}

	if blockers == nil {
		blockers = []*models.Task{}
	}
	respondJSON(w, http.StatusOK, blockers)
}

// addDependency handles POST /api/tasks/{id}/dependencies
func (h *handlers) addDependency(w http.ResponseWriter, r *http.Request) {
	id, ok := h.resolveAndRespond(w, r, "id", string(models.WorkItemTypeTask), "task")
	if !ok {
		return
	}

	var req addDependencyRequest
	if err := decodeJSON(r, w, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.DependsOnTaskID == "" {
		respondError(w, http.StatusBadRequest, "depends_on_task_id is required")
		return
	}

	if err := h.tasks.AddDependency(r.Context(), id, req.DependsOnTaskID); err != nil {
		if errors.Is(err, store.ErrSelfDependency) || errors.Is(err, store.ErrCycleDetected) {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to add dependency: "+err.Error())
		return
	}

	// Return updated dependencies list.
	deps, err := h.tasks.GetDependencies(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get dependencies: "+err.Error())
		return
	}
	if deps == nil {
		deps = []string{}
	}

	respondJSON(w, http.StatusOK, map[string]any{"task_id": id, "dependencies": deps})
}

// removeDependency handles DELETE /api/tasks/{id}/dependencies/{dependsOnId}
func (h *handlers) removeDependency(w http.ResponseWriter, r *http.Request) {
	id, ok := h.resolveAndRespond(w, r, "id", string(models.WorkItemTypeTask), "task")
	if !ok {
		return
	}

	dependsOnID := chi.URLParam(r, "dependsOnId")
	if dependsOnID == "" {
		respondError(w, http.StatusBadRequest, "missing depends_on id")
		return
	}

	if err := h.tasks.RemoveDependency(r.Context(), id, dependsOnID); err != nil {
		errMsg := err.Error()
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, errMsg)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to remove dependency: "+errMsg)
		return
	}

	// Return updated dependencies list.
	deps, err := h.tasks.GetDependencies(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get dependencies: "+err.Error())
		return
	}
	if deps == nil {
		deps = []string{}
	}

	respondJSON(w, http.StatusOK, map[string]any{"task_id": id, "dependencies": deps})
}
