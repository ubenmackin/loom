package acp

import (
	"reflect"
	"testing"
)

// TestExtractAvailableModes_PrefersConfigOptions verifies that when the
// session/new response carries BOTH a modern ConfigOptions surface (with a
// "mode"-category option listing values ["planner","executor"]) AND a legacy
// Modes.AvailableModes surface (set differently), the helper prefers the
// ConfigOptions entries and returns ["planner","executor"].
//
// This is the ConfigOptions-preference path added by TASK-003 to drive the
// modern session/set_config_option routing in the gateway.
func TestExtractAvailableModes_PrefersConfigOptions(t *testing.T) {
	resp := &NewSessionResponse{
		SessionID: "sess-modes",
		ConfigOptions: []SessionConfigOption{
			{
				ID:       "mode",
				Category: "mode",
				Type:     "select",
				Options: []SessionConfigOptionOption{
					{Value: "planner"},
					{Value: "executor"},
				},
			},
		},
		// Legacy Modes.AvailableModes is set differently on purpose to
		// prove the ConfigOptions form wins.
		Modes: &SessionModeState{
			CurrentModeID: "reviewer",
			AvailableModes: []SessionMode{
				{ID: "reviewer"},
				{ID: "security-auditor"},
			},
		},
	}

	got := extractAvailableModes(resp)
	want := []string{"planner", "executor"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("extractAvailableModes(resp) = %v, want %v (ConfigOptions preference)", got, want)
	}
}

// TestExtractAvailableModes_FallsBackToModes verifies the legacy fallback
// path: when the response carries no ConfigOptions but does carry a
// Modes.AvailableModes slice, the helper returns each available mode's ID.
func TestExtractAvailableModes_FallsBackToModes(t *testing.T) {
	resp := &NewSessionResponse{
		SessionID: "sess-legacy",
		// ConfigOptions deliberately nil/empty.
		Modes: &SessionModeState{
			CurrentModeID: "planner",
			AvailableModes: []SessionMode{
				{ID: "planner"},
				{ID: "reviewer"},
			},
		},
	}

	got := extractAvailableModes(resp)
	want := []string{"planner", "reviewer"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("extractAvailableModes(resp) = %v, want %v (Modes.AvailableModes fallback)", got, want)
	}
}

// TestExtractAvailableModes_NilSafe verifies the helper is nil-safe: a nil
// pointer and a response that advertises neither ConfigOptions nor Modes
// must return a nil slice (not a panic, not an empty non-nil slice).
func TestExtractAvailableModes_NilSafe(t *testing.T) {
	if got := extractAvailableModes(nil); got != nil {
		t.Errorf("extractAvailableModes(nil) = %v, want nil", got)
	}

	empty := &NewSessionResponse{SessionID: "sess-empty"}
	if got := extractAvailableModes(empty); got != nil {
		t.Errorf("extractAvailableModes(empty) = %v, want nil", got)
	}

	// ConfigOptions present but no "mode"-category entry: must fall
	// through to the Modes branch, which is also absent here, so still nil.
	otherOnly := &NewSessionResponse{
		ConfigOptions: []SessionConfigOption{
			{ID: "other", Category: "other", Type: "select"},
		},
	}
	if got := extractAvailableModes(otherOnly); got != nil {
		t.Errorf("extractAvailableModes(otherOnly) = %v, want nil", got)
	}
}
