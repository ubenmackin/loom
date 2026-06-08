package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
)

// validTaskTypes is the set of allowed task type values.
var validTaskTypes = map[models.TaskType]bool{
	models.TaskTypeCode:     true,
	models.TaskTypeBuild:    true,
	models.TaskTypeReview:   true,
	models.TaskTypePlanning: true,
}

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

	// Validate task_types if provided.
	for _, tt := range profile.TaskTypes {
		if !validTaskTypes[models.TaskType(tt)] {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid task_type: %q", tt))
			return
		}
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

	// Validate task_types if provided.
	for _, tt := range profile.TaskTypes {
		if !validTaskTypes[models.TaskType(tt)] {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid task_type: %q", tt))
			return
		}
	}

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

// stripJSONCComments removes // line comments and /* */ block comments from JSONC content.
// It correctly handles strings (including escaped quotes) to avoid stripping // inside strings.
func stripJSONCComments(input string) (string, error) {
	var out strings.Builder
	out.Grow(len(input))

	i := 0
	for i < len(input) {
		// Check if we're entering a string
		if input[i] == '"' {
			// Copy the string as-is
			out.WriteByte('"')
			i++
			for i < len(input) {
				out.WriteByte(input[i])
				if input[i] == '\\' && i+1 < len(input) {
					i++
					out.WriteByte(input[i]) // skip escaped char
				} else if input[i] == '"' {
					break // end of string
				}
				i++
			}
			i++
			continue
		}

		// Check for single-line comment
		if i+1 < len(input) && input[i] == '/' && input[i+1] == '/' {
			// Skip to end of line
			for i < len(input) && input[i] != '\n' {
				i++
			}
			continue
		}

		// Check for block comment
		if i+1 < len(input) && input[i] == '/' && input[i+1] == '*' {
			i += 2
			for i+1 < len(input) {
				if input[i] == '*' && input[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			continue
		}

		out.WriteByte(input[i])
		i++
	}

	return out.String(), nil
}

// importProfiles handles POST /api/profiles/import
// Reads opencode.json from a project's repo_path and creates/updates agent profiles.
func (h *handlers) importProfiles(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10 MB limit

	// Extract project_id from query param or request body.
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		var body struct {
			ProjectID string `json:"project_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			projectID = body.ProjectID
		}
	}

	if projectID == "" {
		respondError(w, http.StatusBadRequest, "project_id is required")
		return
	}

	// Fetch the project to get repo_path.
	project, err := h.projects.GetByID(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "project not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get project: "+err.Error())
		return
	}

	if project.RepoPath == "" {
		respondError(w, http.StatusNotFound, "project has no repo_path configured")
		return
	}

	// Validate the repo path to prevent path traversal.
	cleanPath := filepath.Clean(project.RepoPath)
	if !filepath.IsAbs(cleanPath) {
		respondError(w, http.StatusBadRequest, "project repo_path must be an absolute path")
		return
	}
	project.RepoPath = cleanPath

	// Try to read opencode.json or opencode.jsonc from the repo path.
	var data []byte
	var readErr error

	// Try opencode.json first.
	jsonPath := filepath.Join(project.RepoPath, "opencode.json")
	data, readErr = os.ReadFile(jsonPath)

	// If not found, try .opencode/opencode.json (or .opencode.json)
	if readErr != nil {
		jsonPath = filepath.Join(project.RepoPath, ".opencode", "opencode.json")
		data, readErr = os.ReadFile(jsonPath)
	}

	// Try opencode.jsonc (with comments)
	if readErr != nil {
		jsoncPath := filepath.Join(project.RepoPath, "opencode.jsonc")
		data, readErr = os.ReadFile(jsoncPath)
	}

	// Try .opencode/opencode.jsonc
	if readErr != nil {
		jsoncPath := filepath.Join(project.RepoPath, ".opencode", "opencode.jsonc")
		data, readErr = os.ReadFile(jsoncPath)
	}

	if readErr != nil {
		respondError(w, http.StatusNotFound, "opencode.json not found in project")
		return
	}

	// Parse JSON — strip comments if .jsonc extension.
	content := string(data)
	if strings.HasSuffix(jsonPath, ".jsonc") {
		// State-machine-based JSONC comment stripper.
		var err error
		content, err = stripJSONCComments(content)
		if err != nil {
			respondError(w, http.StatusBadRequest, "failed to parse opencode.jsonc: "+err.Error())
			return
		}
	}

	var opencodeConfig struct {
		Agent map[string]struct {
			Description string `json:"description"`
			Model       string `json:"model"`
			Prompt      string `json:"prompt"`
		} `json:"agent"`
	}

	if err := json.Unmarshal([]byte(content), &opencodeConfig); err != nil {
		respondError(w, http.StatusBadRequest, "failed to parse opencode.json: "+err.Error())
		return
	}

	if len(opencodeConfig.Agent) == 0 {
		respondError(w, http.StatusBadRequest, "no agent definitions found in opencode.json")
		return
	}

	// Load existing profiles once for name matching (avoid N+1 queries).
	existingProfiles, err := h.profiles.List(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list profiles: "+err.Error())
		return
	}

	// Build a lookup map by name.
	profileByName := make(map[string]*models.AgentProfile, len(existingProfiles))
	for _, p := range existingProfiles {
		profileByName[p.Name] = p
	}

	// Create or update profiles for each agent definition.
	var importedProfiles []*models.AgentProfile

	for agentName, agentDef := range opencodeConfig.Agent {
		_, ok := profileByName[agentName]
		existing := profileByName[agentName]

		if ok {
			// Update existing profile.
			existing.Description = agentDef.Description
			if existing.Capabilities == "" {
				existing.Capabilities = "[]"
			}
			if existing.MaxConcurrency == 0 {
				existing.MaxConcurrency = 5
			}
			if err := h.profiles.Update(r.Context(), existing); err != nil {
				slog.Error("failed to update imported profile", "name", agentName, "error", err)
				continue
			}
			importedProfiles = append(importedProfiles, existing)
		} else {
			// Create new profile.
			profile := &models.AgentProfile{
				Name:           agentName,
				Description:    agentDef.Description,
				Capabilities:   "[]",
				MaxConcurrency: 5,
			}
			if err := h.profiles.Create(r.Context(), profile); err != nil {
				slog.Error("failed to create imported profile", "name", agentName, "error", err)
				continue
			}
			importedProfiles = append(importedProfiles, profile)
		}
	}

	// Reload gateway profiles.
	if h.gateway != nil {
		if err := h.gateway.ReloadProfiles(r.Context()); err != nil {
			slog.Error("failed to reload gateway profiles after import", "error", err)
		}
	}

	respondJSON(w, http.StatusOK, importedProfiles)
}
