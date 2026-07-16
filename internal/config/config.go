// Package config holds application configurations and environmental helpers.
package config

import (
	"os"
	"strconv"
	"strings"
)

// GetAllowedOrigins reads the LOOM_ALLOWED_ORIGINS environment variable
// and returns a list of allowed origins. Defaults to localhost patterns.
func GetAllowedOrigins() []string {
	raw := os.Getenv("LOOM_ALLOWED_ORIGINS")
	if raw == "" {
		return []string{
			"http://localhost:3000",
			"http://localhost:5173",
			"http://localhost:8080",
			"http://127.0.0.1:3000",
			"http://127.0.0.1:5173",
			"http://127.0.0.1:8080",
		}
	}
	var origins []string
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			origins = append(origins, o)
		}
	}
	return origins
}

// IsOriginAllowed checks if the given origin is in the allowed list.
func IsOriginAllowed(origin string, allowed []string) bool {
	for _, ao := range allowed {
		if ao == "*" || ao == origin {
			return true
		}
	}
	return false
}

// IsAgentSecretDisabled reads the LOOM_DISABLE_AGENT_SECRET environment variable.
// When set to any truthy value ("1", "true", "yes"), the shared-secret
// authentication fallback (X-Agent-Secret header) is disabled entirely.
// This is intended for production deployments that rely solely on
// session-based or token-based authentication.
func IsAgentSecretDisabled() bool {
	raw := os.Getenv("LOOM_DISABLE_AGENT_SECRET")
	if raw == "" {
		return false
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		// Accept common truthy strings that strconv doesn't handle.
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "yes", "1":
			return true
		default:
			return false
		}
	}
	return v
}
