package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
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
	taskType := chi.URLParam(r, "taskType")
	if taskType == "" {
		respondError(w, http.StatusBadRequest, "missing task type")
		return
	}

	template, err := h.templates.GetByTaskType(r.Context(), models.TaskType(taskType))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
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
	taskType := chi.URLParam(r, "taskType")
	if taskType == "" {
		respondError(w, http.StatusBadRequest, "missing task type")
		return
	}

	var req upsertTemplateRequest
	if err := decodeJSON(r, w, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Template == "" {
		respondError(w, http.StatusBadRequest, "template is required")
		return
	}

	// Check if template exists to determine correct status code.
	created := false
	existing, err := h.templates.GetByTaskType(r.Context(), models.TaskType(taskType))
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusInternalServerError, "failed to check existing template: "+err.Error())
			return
		}
		created = true
	}

	t := &models.PromptTemplate{
		TaskType: models.TaskType(taskType),
		Template: req.Template,
	}
	if existing != nil {
		t.ID = existing.ID
	}

	if err := h.templates.Upsert(r.Context(), t); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to upsert template: "+err.Error())
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}

	respondJSON(w, status, t)
}
