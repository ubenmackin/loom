package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// configStatusResponse is the JSON response for
// GET /api/projects/{id}/config-status.
//
// It surfaces the per-project opencode configuration-mismatch warnings the
// gateway accumulated via Gateway.recordConfigMismatch (Decisions 4 /
// TASK-010). The slice is sorted and deduplicated by
// Gateway.MissingOpencodeBlocks. It may be empty (the gateway always
// returns a non-nil slice).
type configStatusResponse struct {
	// MissingOpencodeBlocks lists the opencode agent block names the
	// gateway recorded as missing for this project (shaped
	// "missing_block:<agentType>"). v1 is read-only: there is no
	// PATCH/PUT endpoint to clear entries manually — they auto-clear when
	// the agent subsequently advertises the missing mode.
	MissingOpencodeBlocks []string `json:"missing_opencode_blocks"`
}

// handleProjectConfigStatus handles GET /api/projects/{id}/config-status
// (admin-only, registered in router.go alongside the other project/{id}
// admin endpoints).
//
// It returns a 200 OK with the missing-blocks surface even when the slice
// is empty, so the UI can distinguish "no warning" (200, empty array) from
// "endpoint not initialized" (404, only when the gateway is absent). The
// 503 path is reserved for when the gateway itself is not wired into the
// API handlers — mirroring handleGatewayStatus/handleGatewayQueue.
func (h *handlers) handleProjectConfigStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "missing project id")
		return
	}

	if h.gateway == nil {
		respondError(w, http.StatusServiceUnavailable, "gateway not initialized")
		return
	}

	blocks := h.gateway.MissingOpencodeBlocks(id)
	// MissingOpencodeBlocks always returns a non-nil slice, but be
	// defensive in case a future alternate implementation returns nil.
	if blocks == nil {
		blocks = []string{}
	}

	respondJSON(w, http.StatusOK, configStatusResponse{
		MissingOpencodeBlocks: blocks,
	})
}
