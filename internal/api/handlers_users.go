package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
)

func (h *handlers) registerUserRoutes(r chi.Router) {
	r.Get("/", h.listUsers)
	r.Post("/", h.createUserAsAdmin)
	r.Delete("/{id}", h.deleteUser)
}

// listUsers handles GET /api/users — returns all users (admin only).
func (h *handlers) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.ListAll(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list users: "+err.Error())
		return
	}
	if users == nil {
		users = []*models.User{}
	}
	respondJSON(w, http.StatusOK, users)
}

// adminCreateUserRequest is the request type for admin-driven user creation.
// It mirrors signupRequest but is intentionally decoupled to prevent accidental
// coupling between public signup and admin user creation.
type adminCreateUserRequest struct {
	Username    string          `json:"username"`
	Email       string          `json:"email"`
	DisplayName string          `json:"display_name"`
	Password    string          `json:"password"`
	Role        models.UserRole `json:"role"`
}

// createUserAsAdmin handles POST /api/users — admin creates a new user with an explicit role.
func (h *handlers) createUserAsAdmin(w http.ResponseWriter, r *http.Request) {
	var req adminCreateUserRequest
	if err := decodeJSON(r, w, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Username == "" {
		respondError(w, http.StatusBadRequest, "username is required")
		return
	}
	if req.Email == "" {
		respondError(w, http.StatusBadRequest, "email is required")
		return
	}
	if len(req.Password) < 6 {
		respondError(w, http.StatusBadRequest, "password must be at least 6 characters")
		return
	}

	role := req.Role
	if role != models.RoleAdmin && role != models.RoleNormal {
		role = models.RoleNormal
	}

	user, err := h.users.CreateUser(r.Context(), req.Username, req.Email, req.DisplayName, req.Password, role)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, user)
}

// deleteUser handles DELETE /api/users/{id} — admin deletes a user (cannot delete self).
func (h *handlers) deleteUser(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "id")
	if targetID == "" {
		respondError(w, http.StatusBadRequest, "user id is required")
		return
	}

	currentUser := GetUser(r)
	if currentUser != nil && currentUser.ID == targetID {
		respondError(w, http.StatusBadRequest, "cannot delete your own account")
		return
	}

	if err := h.users.DeleteUser(r.Context(), targetID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to delete user: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
