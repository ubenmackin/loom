// Package api provides HTTP handlers and middleware for the Loom server.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/ubenmackin/loom/internal/config"
	"github.com/ubenmackin/loom/internal/models"
)

// contextKeySessionID is the context key for the extracted session ID.
type contextKeySessionID struct{}

// Logger returns a request logging middleware using slog.
func Logger(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"elapsed", time.Since(start).String(),
		)
	}
	return http.HandlerFunc(fn)
}

// Recovery returns a panic recovery middleware.
func Recovery(next http.Handler) http.Handler {
	return middleware.Recoverer(next)
}

// CORS returns a middleware that sets CORS headers. Allowed origins are
// read from the LOOM_ALLOWED_ORIGINS environment variable (comma-separated).
// Defaults to localhost patterns for development.
func CORS(next http.Handler) http.Handler {
	allowedOrigins := config.GetAllowedOrigins()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			// Not a CORS request — proceed normally.
			next.ServeHTTP(w, r)
			return
		}

		// Check if the request origin is in the allowed list.
		if !config.IsOriginAllowed(origin, allowedOrigins) {
			// Origin not allowed — proceed without CORS headers.
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Session-ID, X-Agent-Secret, Authorization")
		w.Header().Set("Access-Control-Max-Age", "300")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// SessionExtractor extracts session_id from the X-Session-ID header or
// session_id query parameter and stores it in the request context.
func SessionExtractor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.Header.Get("X-Session-ID")
		if sessionID == "" {
			sessionID = r.URL.Query().Get("session_id")
		}
		if sessionID != "" {
			ctx := context.WithValue(r.Context(), contextKeySessionID{}, sessionID)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

// GetSessionID retrieves the session ID from the request context.
func GetSessionID(r *http.Request) string {
	if v, ok := r.Context().Value(contextKeySessionID{}).(string); ok {
		return v
	}
	return ""
}

// respondJSON marshals data to JSON and writes it with the given status code.
func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			slog.Error("failed to encode JSON response", "error", err)
		}
	}
}

// respondError writes a JSON error response with the given status and message.
func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		slog.Error("failed to encode error response", "status", status, "error", err)
	}
}

// contextKeyUser is the context key for the authenticated user.
type contextKeyUser struct{}

// GetUser retrieves the User from the request context.
func GetUser(r *http.Request) *models.User {
	if v, ok := r.Context().Value(contextKeyUser{}).(*models.User); ok {
		return v
	}
	return nil
}

// UserAuthenticator is a middleware that extracts a Bearer token, verifies it against the sessions store,
// and injects the authenticated User object into the request context.
func (h *handlers) UserAuthenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			respondError(w, http.StatusUnauthorized, "authorization header with Bearer token required")
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")

		user, err := h.users.GetUserBySessionToken(r.Context(), token)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "invalid or expired session token")
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyUser{}, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// agentSecret is the shared secret for agent authentication, read once at init.
var (
	agentSecret     string
	agentSecretOnce sync.Once
)

// getAgentSecret returns the shared agent secret, reading it from the environment once.
func getAgentSecret() string {
	agentSecretOnce.Do(func() {
		agentSecret = os.Getenv("LOOM_AGENT_SECRET")
	})
	return agentSecret
}

// SessionAuthenticator is a middleware that authenticates agent requests to
// the /sessions and /work route groups. It requires either:
//  1. A valid X-Session-ID header for an existing, active session, OR
//  2. A shared secret via the LOOM_AGENT_SECRET environment variable,
//     sent as the X-Agent-Secret header.
//
// This prevents unauthenticated clients from registering sessions, claiming
// tasks, or modifying board state.
func (h *handlers) SessionAuthenticator(next http.Handler) http.Handler {
	secret := getAgentSecret()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow requests that present the shared agent secret.
		if secret != "" {
			if reqSecret := r.Header.Get("X-Agent-Secret"); reqSecret != "" && reqSecret == secret {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Otherwise, require a valid active session ID.
		sessionID := r.Header.Get("X-Session-ID")
		if sessionID == "" {
			sessionID = r.URL.Query().Get("session_id")
		}
		if sessionID == "" {
			respondError(w, http.StatusUnauthorized, "X-Session-ID header or X-Agent-Secret required")
			return
		}

		session, err := h.sessions.GetByID(r.Context(), sessionID)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "invalid session ID")
			return
		}
		if session.Status != models.SessionStatusActive {
			respondError(w, http.StatusForbidden, fmt.Sprintf("session %q is not active (status=%q)", sessionID, session.Status))
			return
		}

		// Store the validated session ID in context.
		ctx := context.WithValue(r.Context(), contextKeySessionID{}, sessionID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AdminOnly is a middleware that rejects requests from non-admin users.
func (h *handlers) AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r)
		if user == nil {
			respondError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if user.Role != models.RoleAdmin {
			respondError(w, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// resolveWorkItemID resolves a numeric or string ID to the standard string ID.
// It uses strconv.Atoi which requires the full string to be numeric,
// preventing partial matches like "42abc" from being accepted.
func (h *handlers) resolveWorkItemID(ctx context.Context, idStr string, itemType string) (string, error) {
	if idStr == "" {
		return "", errors.New("missing id")
	}
	if numID, err := strconv.Atoi(idStr); err == nil {
		if itemType == string(models.WorkItemTypeStory) {
			story, err := h.stories.GetByNumericID(ctx, numID)
			if err != nil {
				return "", err
			}
			return story.ID, nil
		} else {
			task, err := h.tasks.GetByNumericID(ctx, numID)
			if err != nil {
				return "", err
			}
			return task.ID, nil
		}
	}
	// For non-numeric IDs (e.g., "STORY-999"), verify the item exists
	if itemType == string(models.WorkItemTypeStory) {
		_, err := h.stories.GetByID(ctx, idStr)
		if err != nil {
			return "", err
		}
	} else {
		_, err := h.tasks.GetByID(ctx, idStr)
		if err != nil {
			return "", err
		}
	}
	return idStr, nil
}
