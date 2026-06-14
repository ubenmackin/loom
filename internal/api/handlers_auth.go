package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// --- Request/Response types for Auth ---

type signupRequest struct {
	Username    string          `json:"username"`
	Email       string          `json:"email"`
	DisplayName string          `json:"display_name"`
	Password    string          `json:"password"`
	Role        models.UserRole `json:"role"`
}

type loginRequest struct {
	UsernameOrEmail string `json:"username_or_email"`
	Password        string `json:"password"`
}

type authResponse struct {
	User  *models.User `json:"user"`
	Token string       `json:"token"`
}

type onboardingCheckResponse struct {
	OnboardingRequired bool `json:"onboarding_required"`
}

type meResponse struct {
	User *models.User `json:"user"`
}

type updateProfileRequest struct {
	DisplayName     string `json:"display_name,omitempty"`
	Email           string `json:"email,omitempty"`
	CurrentPassword string `json:"current_password,omitempty"`
	NewPassword     string `json:"new_password,omitempty"`
}

// --- Route registration ---

func (h *handlers) registerAuthRoutes(r chi.Router) {
	r.Get("/onboarding-check", h.onboardingCheck)
	r.Post("/signup", h.signup)
	r.Post("/login", h.login)

	// Protected endpoints (require authentication)
	r.Group(func(r chi.Router) {
		r.Use(h.UserAuthenticator)
		r.Post("/logout", h.logout)
		r.Get("/me", h.me)
		r.Put("/me", h.updateMyProfile)
	})
}

// --- Handlers ---

// onboardingCheck handles GET /api/auth/onboarding-check
func (h *handlers) onboardingCheck(w http.ResponseWriter, r *http.Request) {
	count, err := h.users.CountUsers(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to check onboarding status: "+err.Error())
		return
	}
	respondJSON(w, http.StatusOK, onboardingCheckResponse{
		OnboardingRequired: count == 0,
	})
}

// signup handles POST /api/auth/signup
func (h *handlers) signup(w http.ResponseWriter, r *http.Request) {
	var req signupRequest
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

	// Auto-assign admin role to the very first user; everyone else is normal
	// unless an explicit role is supplied (admin-created users).
	role := req.Role
	if role == "" {
		count, err := h.users.CountUsers(r.Context())
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to check user count: "+err.Error())
			return
		}
		if count == 0 {
			role = models.RoleAdmin
		} else {
			role = models.RoleNormal
		}
	}

	user, err := h.users.CreateUser(r.Context(), req.Username, req.Email, req.DisplayName, req.Password, role)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Log in automatically on signup by creating a session
	token, err := h.users.CreateSession(r.Context(), user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create session: "+err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, authResponse{
		User:  user,
		Token: token,
	})
}

// login handles POST /api/auth/login
func (h *handlers) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, w, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.UsernameOrEmail == "" {
		respondError(w, http.StatusBadRequest, "username or email is required")
		return
	}
	if req.Password == "" {
		respondError(w, http.StatusBadRequest, "password is required")
		return
	}

	user, err := h.users.AuthenticateUser(r.Context(), req.UsernameOrEmail, req.Password)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := h.users.CreateSession(r.Context(), user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create session: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, authResponse{
		User:  user,
		Token: token,
	})
}

// logout handles POST /api/auth/logout
func (h *handlers) logout(w http.ResponseWriter, r *http.Request) {
	// Extract Bearer token from header
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		respondError(w, http.StatusUnauthorized, "authorization token required")
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")

	if err := h.users.DeleteSession(r.Context(), token); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete session: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// me handles GET /api/auth/me
func (h *handlers) me(w http.ResponseWriter, r *http.Request) {
	currentUser := GetUser(r)
	if currentUser == nil {
		respondError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	respondJSON(w, http.StatusOK, meResponse{
		User: currentUser,
	})
}

// updateMyProfile handles PUT /api/auth/me
func (h *handlers) updateMyProfile(w http.ResponseWriter, r *http.Request) {
	currentUser := GetUser(r)
	if currentUser == nil {
		respondError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var req updateProfileRequest
	if err := decodeJSON(r, w, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	hasProfileChanges := req.DisplayName != "" || req.Email != ""

	// Validate password change
	if req.CurrentPassword != "" || req.NewPassword != "" {
		if req.CurrentPassword == "" {
			respondError(w, http.StatusBadRequest, "current_password is required for password change")
			return
		}
		if req.NewPassword == "" {
			respondError(w, http.StatusBadRequest, "new_password is required for password change")
			return
		}
		if len(req.NewPassword) < 6 {
			respondError(w, http.StatusBadRequest, "new_password must be at least 6 characters")
			return
		}

		// Verify current password
		_, err := h.users.AuthenticateUser(r.Context(), currentUser.Username, req.CurrentPassword)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "current password is incorrect")
			return
		}

		// Hash new password
		hashBytes, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), 12)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to hash password")
			return
		}

		if err := h.users.UpdateUserPassword(r.Context(), currentUser.ID, string(hashBytes)); err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, store.ErrNotFound) {
				respondError(w, http.StatusNotFound, "user not found")
				return
			}
			respondError(w, http.StatusInternalServerError, "failed to update password: "+err.Error())
			return
		}
	}

	// Update profile fields (display_name, email) if provided
	if hasProfileChanges {
		displayName := currentUser.DisplayName
		email := currentUser.Email
		if req.DisplayName != "" {
			displayName = req.DisplayName
		}
		if req.Email != "" {
			email = req.Email
		}

		if err := h.users.UpdateUserProfile(r.Context(), currentUser.ID, displayName, email); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				respondError(w, http.StatusNotFound, "user not found")
				return
			}
			respondError(w, http.StatusInternalServerError, "failed to update profile: "+err.Error())
			return
		}

		currentUser.DisplayName = displayName
		currentUser.Email = email
	}

	respondJSON(w, http.StatusOK, meResponse{
		User: currentUser,
	})
}
