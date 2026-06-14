package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ubenmackin/loom/internal/store"
)

// MaxTotalSetter is an interface for updating the global concurrency cap
// on a running gateway's job queue. Decoupling via an interface keeps the
// handler independent of the concrete *JobQueue type.
type MaxTotalSetter interface {
	SetMaxTotal(n int)
}

// SettingsHandler provides HTTP handlers for reading and writing application
// settings stored in the settings key-value table.
type SettingsHandler struct {
	settingStore   SettingStore
	maxTotalSetter MaxTotalSetter
}

// NewSettingsHandler creates a new SettingsHandler. The maxTotalSetter may be
// nil; when non-nil it is invoked after a successful DB write so the running
// gateway picks up the new cap without requiring a restart.
func NewSettingsHandler(settingStore SettingStore, maxTotalSetter MaxTotalSetter) *SettingsHandler {
	return &SettingsHandler{settingStore: settingStore, maxTotalSetter: maxTotalSetter}
}

// GetGlobalMaxConcurrency returns the global max concurrency setting.
// GET /api/settings/global_max_concurrency
func (h *SettingsHandler) GetGlobalMaxConcurrency(w http.ResponseWriter, r *http.Request) {
	v, err := h.settingStore.Get(r.Context(), store.SettingKeyGlobalMaxConcurrency)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Key not set — default to 0 (unlimited)
			respondJSON(w, http.StatusOK, map[string]int{"value": 0})
			return
		}
		// Genuine DB error
		slog.Error("settings: failed to read global_max_concurrency", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to read global_max_concurrency")
		return
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		// Corrupt value in the DB — surface the problem to the operator
		// rather than silently masking it as the legitimate "not set" default.
		slog.Error("settings: global_max_concurrency has corrupt value", "value", v, "error", err)
		truncated := v
		if len(truncated) > 64 {
			truncated = truncated[:64] + "..."
		}
		respondError(w, http.StatusInternalServerError, "global_max_concurrency has a corrupt value: "+truncated)
		return
	}
	respondJSON(w, http.StatusOK, map[string]int{"value": n})
}

// SetGlobalMaxConcurrency updates the global max concurrency setting.
// PUT /api/settings/global_max_concurrency
func (h *SettingsHandler) SetGlobalMaxConcurrency(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Value int `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if body.Value < 0 {
		respondError(w, http.StatusBadRequest, "value must be >= 0")
		return
	}
	if body.Value > 10000 {
		respondError(w, http.StatusBadRequest, "value must be <= 10000")
		return
	}
	if err := h.settingStore.Set(r.Context(), store.SettingKeyGlobalMaxConcurrency, strconv.Itoa(body.Value)); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save setting")
		return
	}

	// Propagate to the running gateway's job queue so the new cap takes
	// effect immediately rather than on the next server restart.
	if h.maxTotalSetter != nil {
		h.maxTotalSetter.SetMaxTotal(body.Value)
	}

	respondJSON(w, http.StatusOK, map[string]int{"value": body.Value})
}
