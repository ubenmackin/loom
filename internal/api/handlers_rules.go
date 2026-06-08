package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
)

// listRulesByProfile handles GET /api/profiles/{id}/rules
func (h *handlers) listRulesByProfile(w http.ResponseWriter, r *http.Request) {
	profileID := chi.URLParam(r, "id")
	rules, err := h.rules.ListByProfile(r.Context(), profileID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list rules: "+err.Error())
		return
	}
	if rules == nil {
		rules = []*models.TriggerRule{}
	}
	respondJSON(w, http.StatusOK, rules)
}

// createRule handles POST /api/profiles/{id}/rules
func (h *handlers) createRule(w http.ResponseWriter, r *http.Request) {
	profileID := chi.URLParam(r, "id")

	var rule models.TriggerRule
	if err := decodeJSON(r, w, &rule); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	rule.AgentProfileID = profileID
	if rule.EventType == "" || rule.Action == "" {
		respondError(w, http.StatusBadRequest, "event_type and action are required")
		return
	}
	if err := h.rules.Create(r.Context(), &rule); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create rule: "+err.Error())
		return
	}
	// Reload gateway rules to pick up new trigger rules.
	if h.gateway != nil {
		if err := h.gateway.ReloadRules(r.Context()); err != nil {
			slog.Error("failed to reload gateway rules after create", "error", err)
		}
	}
	respondJSON(w, http.StatusCreated, rule)
}

// updateRule handles PUT /api/profiles/{id}/rules/{ruleID}
func (h *handlers) updateRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "ruleID")

	var rule models.TriggerRule
	if err := decodeJSON(r, w, &rule); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	rule.ID = ruleID

	if rule.EventType == "" || rule.Action == "" {
		respondError(w, http.StatusBadRequest, "event_type and action are required")
		return
	}

	if err := h.rules.Update(r.Context(), &rule); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "rule not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update rule: "+err.Error())
		return
	}
	// Reload gateway rules to pick up updated trigger rules.
	if h.gateway != nil {
		if err := h.gateway.ReloadRules(r.Context()); err != nil {
			slog.Error("failed to reload gateway rules after update", "error", err)
		}
	}
	respondJSON(w, http.StatusOK, rule)
}

// deleteRule handles DELETE /api/profiles/{id}/rules/{ruleID}
func (h *handlers) deleteRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "ruleID")
	if err := h.rules.Delete(r.Context(), ruleID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "rule not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to delete rule: "+err.Error())
		return
	}
	// Reload gateway rules to pick up rule deletion.
	if h.gateway != nil {
		if err := h.gateway.ReloadRules(r.Context()); err != nil {
			slog.Error("failed to reload gateway rules after delete", "error", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
