// Package config provides shared configuration helpers for the Loom server.
package config

import (
	"os"
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
			"http://127.0.0.1:3000",
			"http://127.0.0.1:5173",
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
// A wildcard entry ("*") permits any origin.
func IsOriginAllowed(origin string, allowed []string) bool {
	for _, ao := range allowed {
		if ao == "*" || ao == origin {
			return true
		}
	}
	return false
}
