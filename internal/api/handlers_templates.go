package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/models"
)

// --- Request/Response types ---

type upsertTemplateRequest struct {
	Template string `json:"template"`
}

// --- Route registration ---

func (h *handlers) registerTemplateRoutes(r chi.Router) {
	r.Get("/", h.listTemplates)
	r.Route("/{taskType}", func(r chi.Router) {
		r.Get("/", h.getTemplate)
		r.Put("/", h.upsertTemplate)
	})
}

// --- Handlers ---

// listTemplates handles GET /api/templates
func (h *handlers) listTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := h.templates.List(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list templates: "+err.Error())
		return
	}

	if templates == nil {
		templates = []*models.PromptTemplate{}
	}
	respondJSON(w, http.StatusOK, templates)
}

// getTemplate handles GET /api/templates/{taskType}
func (h *handlers) getTemplate(w http.ResponseWriter, r *http.Request) {
	taskType := parseID(r, "taskType")
	if taskType == "" {
		respondError(w, http.StatusBadRequest, "missing task type")
		return
	}

	template, err := h.templates.GetByTaskType(r.Context(), taskType)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondError(w, http.StatusNotFound, "template not found for task type: "+taskType)
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to get template: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, template)
}

// upsertTemplate handles PUT /api/templates/{taskType}
func (h *handlers) upsertTemplate(w http.ResponseWriter, r *http.Request) {
	taskType := parseID(r, "taskType")
	if taskType == "" {
		respondError(w, http.StatusBadRequest, "missing task type")
		return
	}

	var req upsertTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Template == "" {
		respondError(w, http.StatusBadRequest, "template is required")
		return
	}

	t := &models.PromptTemplate{
		TaskType: taskType,
		Template: req.Template,
	}

	if err := h.templates.Upsert(r.Context(), t); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to upsert template: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, t)
}
