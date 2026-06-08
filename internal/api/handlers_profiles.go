package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
)

// listProfiles handles GET /api/profiles
func (h *handlers) listProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := h.profiles.List(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list profiles: "+err.Error())
		return
	}
	if profiles == nil {
		profiles = []*models.AgentProfile{}
	}
	respondJSON(w, http.StatusOK, profiles)
}

// createProfile handles POST /api/profiles
func (h *handlers) createProfile(w http.ResponseWriter, r *http.Request) {
	var profile models.AgentProfile
	if err := decodeJSON(r, w, &profile); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if profile.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := h.profiles.Create(r.Context(), &profile); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create profile: "+err.Error())
		return
	}
	// Reload gateway profiles to pick up new concurrency limits.
	if h.gateway != nil {
		if err := h.gateway.ReloadProfiles(r.Context()); err != nil {
			slog.Error("failed to reload gateway profiles after create", "error", err)
		}
	}
	respondJSON(w, http.StatusCreated, profile)
}

// getProfile handles GET /api/profiles/{id}
func (h *handlers) getProfile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	profile, err := h.profiles.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "profile not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get profile: "+err.Error())
		return
	}
	respondJSON(w, http.StatusOK, profile)
}

// updateProfile handles PUT /api/profiles/{id}
func (h *handlers) updateProfile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var profile models.AgentProfile
	if err := decodeJSON(r, w, &profile); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	profile.ID = id

	if err := h.profiles.Update(r.Context(), &profile); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "profile not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update profile: "+err.Error())
		return
	}
	// Reload gateway profiles to pick up updated concurrency limits.
	if h.gateway != nil {
		if err := h.gateway.ReloadProfiles(r.Context()); err != nil {
			slog.Error("failed to reload gateway profiles after update", "error", err)
		}
	}
	respondJSON(w, http.StatusOK, profile)
}

// deleteProfile handles DELETE /api/profiles/{id}
func (h *handlers) deleteProfile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.profiles.Delete(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "profile not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to delete profile: "+err.Error())
		return
	}
	// Reload gateway profiles to remove deleted concurrency limits.
	if h.gateway != nil {
		if err := h.gateway.ReloadProfiles(r.Context()); err != nil {
			slog.Error("failed to reload gateway profiles after delete", "error", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}
