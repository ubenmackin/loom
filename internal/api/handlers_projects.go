package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
)

// --- Request/Response types ---

type createProjectRequest struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	RepoPath     string `json:"repo_path,omitempty"`
	Language     string `json:"language,omitempty"`
	BuildCommand string `json:"build_command,omitempty"`
}

type updateProjectRequest struct {
	Name         *string `json:"name,omitempty"`
	Description  *string `json:"description,omitempty"`
	RepoPath     *string `json:"repo_path,omitempty"`
	Language     *string `json:"language,omitempty"`
	BuildCommand *string `json:"build_command,omitempty"`
}

// --- Handlers ---

// listProjects handles GET /api/projects (user-authenticated, read-only)
func (h *handlers) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.projects.List(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list projects: "+err.Error())
		return
	}

	if projects == nil {
		projects = []*models.Project{}
	}
	respondJSON(w, http.StatusOK, projects)
}

// createProject handles POST /api/projects (admin-only)
func (h *handlers) createProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := decodeJSON(r, w, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}

	project := &models.Project{
		Name:         strings.TrimSpace(req.Name),
		Description:  req.Description,
		RepoPath:     req.RepoPath,
		Language:     req.Language,
		BuildCommand: req.BuildCommand,
	}

	if err := h.projects.Create(r.Context(), project); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create project: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, project)
}

// getProject handles GET /api/projects/{id} (admin-only)
func (h *handlers) getProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "missing project id")
		return
	}

	project, err := h.projects.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "project not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get project: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, project)
}

// updateProject handles PUT /api/projects/{id} (admin-only)
func (h *handlers) updateProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "missing project id")
		return
	}

	project, err := h.projects.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "project not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get project: "+err.Error())
		return
	}

	var req updateProjectRequest
	if err := decodeJSON(r, w, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Apply partial updates.
	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			respondError(w, http.StatusBadRequest, "name cannot be empty")
			return
		}
		project.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		project.Description = *req.Description
	}
	if req.RepoPath != nil {
		project.RepoPath = *req.RepoPath
	}
	if req.Language != nil {
		project.Language = *req.Language
	}
	if req.BuildCommand != nil {
		project.BuildCommand = *req.BuildCommand
	}

	if err := h.projects.Update(r.Context(), project); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusConflict, "project was modified or deleted concurrently")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update project: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, project)
}

// deleteProject handles DELETE /api/projects/{id} (admin-only)
func (h *handlers) deleteProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		respondError(w, http.StatusBadRequest, "missing project id")
		return
	}

	if err := h.projects.Delete(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "project not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to delete project: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
