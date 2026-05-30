package api

import (
	"errors"
	"net/http"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
)

// --- Request types ---

type batchStoryReorderItem struct {
	ID        string `json:"id"`
	SortOrder int    `json:"sort_order"`
}

type batchStoryReorderRequest struct {
	Stories []batchStoryReorderItem `json:"stories"`
}

// --- Handlers ---

// batchReorderStories handles PATCH /api/stories/reorder
func (h *handlers) batchReorderStories(w http.ResponseWriter, r *http.Request) {
	var req batchStoryReorderRequest
	if err := decodeJSON(r, w, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if len(req.Stories) == 0 {
		respondError(w, http.StatusBadRequest, "stories array is required and must not be empty")
		return
	}

	// Fetch all stories in the batch
	stories := make([]*models.Story, len(req.Stories))
	for i, item := range req.Stories {
		story, err := h.stories.GetByID(r.Context(), item.ID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				respondError(w, http.StatusNotFound, "story "+item.ID+" not found")
				return
			}
			respondError(w, http.StatusInternalServerError, "failed to get story: "+err.Error())
			return
		}
		stories[i] = story
	}

	// Apply sort_order updates
	for i, item := range req.Stories {
		stories[i].SortOrder = item.SortOrder
	}

	// Apply all updates in a transaction
	if err := h.stories.BatchUpdate(r.Context(), stories); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to batch update stories: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"updated": len(stories)})
}
