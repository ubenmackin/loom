package api

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/models"
)

// createTestProfileViaAPI creates a minimal agent profile via the API and
// returns its ID. The caller must have already set up admin access.
func createTestProfileViaAPI(t *testing.T, mux chi.Router, name string) string {
	t.Helper()
	body := map[string]any{
		"name":            name,
		"max_concurrency": 1,
	}
	rr := doRequest(t, mux, http.MethodPost, "/api/profiles", body)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create profile %q status = %d, want %d", name, rr.Code, http.StatusCreated)
	}
	var profile models.AgentProfile
	decodeRespJSON(t, rr, &profile)
	return profile.ID
}

func TestRules_ListEmpty(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	profileID := createTestProfileViaAPI(t, mux, "Rule Test Profile")

	rr := doRequest(t, mux, http.MethodGet, "/api/profiles/"+profileID+"/rules", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/profiles/{id}/rules status = %d, want %d", rr.Code, http.StatusOK)
	}

	var rules []*models.TriggerRule
	decodeRespJSON(t, rr, &rules)

	if len(rules) != 0 {
		t.Fatalf("GET /api/profiles/{id}/rules returned %d rules, want 0", len(rules))
	}
}

func TestRules_CreateAndGet(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	profileID := createTestProfileViaAPI(t, mux, "Create Rule Profile")

	// Create a rule.
	createBody := map[string]any{
		"event_type": "story.created",
		"action":     "assign_agent",
		"priority":   10,
		"enabled":    true,
	}
	rr := doRequest(t, mux, http.MethodPost, "/api/profiles/"+profileID+"/rules", createBody)
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST /api/profiles/{id}/rules status = %d, want %d", rr.Code, http.StatusCreated)
	}

	var created models.TriggerRule
	decodeRespJSON(t, rr, &created)

	if created.ID == "" {
		t.Fatal("POST /api/profiles/{id}/rules did not return an ID")
	}
	if created.AgentProfileID != profileID {
		t.Errorf("AgentProfileID = %q, want %q", created.AgentProfileID, profileID)
	}
	if created.EventType != "story.created" {
		t.Errorf("EventType = %q, want %q", created.EventType, "story.created")
	}
	if created.Action != "assign_agent" {
		t.Errorf("Action = %q, want %q", created.Action, "assign_agent")
	}
	if created.Priority != 10 {
		t.Errorf("Priority = %d, want 10", created.Priority)
	}
	if !created.Enabled {
		t.Error("Enabled = false, want true")
	}
	if created.CreatedAt.IsZero() {
		t.Fatal("CreatedAt is zero")
	}
}

func TestRules_CreateMissingFields(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	profileID := createTestProfileViaAPI(t, mux, "Missing Fields Profile")

	// Missing event_type.
	body := map[string]any{"action": "test"}
	rr := doRequest(t, mux, http.MethodPost, "/api/profiles/"+profileID+"/rules", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("POST with missing event_type status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	// Missing action.
	body = map[string]any{"event_type": "test.event"}
	rr = doRequest(t, mux, http.MethodPost, "/api/profiles/"+profileID+"/rules", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("POST with missing action status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestRules_ListByProfile(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	profileID := createTestProfileViaAPI(t, mux, "List By Profile")

	// Create two rules.
	rule1 := map[string]any{"event_type": "story.created", "action": "assign", "priority": 1, "enabled": true}
	rr := doRequest(t, mux, http.MethodPost, "/api/profiles/"+profileID+"/rules", rule1)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create rule 1 status = %d, want %d", rr.Code, http.StatusCreated)
	}

	rule2 := map[string]any{"event_type": "task.done", "action": "review", "priority": 2, "enabled": false}
	rr = doRequest(t, mux, http.MethodPost, "/api/profiles/"+profileID+"/rules", rule2)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create rule 2 status = %d, want %d", rr.Code, http.StatusCreated)
	}

	// List rules for the profile.
	rr = doRequest(t, mux, http.MethodGet, "/api/profiles/"+profileID+"/rules", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/profiles/{id}/rules status = %d, want %d", rr.Code, http.StatusOK)
	}

	var rules []*models.TriggerRule
	decodeRespJSON(t, rr, &rules)

	if len(rules) != 2 {
		t.Fatalf("List returned %d rules, want 2", len(rules))
	}
}

func TestRules_Update(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	profileID := createTestProfileViaAPI(t, mux, "Update Rule Profile")

	// Create.
	createBody := map[string]any{"event_type": "original.event", "action": "original_action", "priority": 1, "enabled": true}
	rr := doRequest(t, mux, http.MethodPost, "/api/profiles/"+profileID+"/rules", createBody)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create rule status = %d, want %d", rr.Code, http.StatusCreated)
	}

	var created models.TriggerRule
	decodeRespJSON(t, rr, &created)

	// Update the rule. The rule endpoint requires ruleID in the URL.
	updateBody := map[string]any{
		"event_type": "updated.event",
		"action":     "updated_action",
		"priority":   99,
		"enabled":    false,
	}
	rr = doRequest(t, mux, http.MethodPut, "/api/profiles/"+profileID+"/rules/"+created.ID, updateBody)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT /api/profiles/{id}/rules/{ruleID} status = %d, want %d", rr.Code, http.StatusOK)
	}

	var updated models.TriggerRule
	decodeRespJSON(t, rr, &updated)

	if updated.EventType != "updated.event" {
		t.Errorf("EventType = %q, want %q", updated.EventType, "updated.event")
	}
	if updated.Action != "updated_action" {
		t.Errorf("Action = %q, want %q", updated.Action, "updated_action")
	}
	if updated.Priority != 99 {
		t.Errorf("Priority = %d, want 99", updated.Priority)
	}
	if updated.Enabled != false {
		t.Error("Enabled = true, want false")
	}
}

func TestRules_UpdateNotFound(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	profileID := createTestProfileViaAPI(t, mux, "Update Not Found Profile")

	updateBody := map[string]any{"event_type": "test.event", "action": "test_action"}
	rr := doRequest(t, mux, http.MethodPut, "/api/profiles/"+profileID+"/rules/non-existent-rule", updateBody)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("PUT non-existent rule status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestRules_Delete(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	profileID := createTestProfileViaAPI(t, mux, "Delete Rule Profile")

	// Create.
	createBody := map[string]any{"event_type": "delete.me", "action": "delete_action", "priority": 1, "enabled": true}
	rr := doRequest(t, mux, http.MethodPost, "/api/profiles/"+profileID+"/rules", createBody)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create rule status = %d, want %d", rr.Code, http.StatusCreated)
	}

	var created models.TriggerRule
	decodeRespJSON(t, rr, &created)

	// Delete.
	rr = doRequest(t, mux, http.MethodDelete, "/api/profiles/"+profileID+"/rules/"+created.ID, nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/profiles/{id}/rules/{ruleID} status = %d, want %d", rr.Code, http.StatusNoContent)
	}

	// Verify deletion by listing (should be empty).
	rr = doRequest(t, mux, http.MethodGet, "/api/profiles/"+profileID+"/rules", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET after delete status = %d, want %d", rr.Code, http.StatusOK)
	}

	var rules []*models.TriggerRule
	decodeRespJSON(t, rr, &rules)

	if len(rules) != 0 {
		t.Fatalf("List after delete returned %d rules, want 0", len(rules))
	}
}

func TestRules_DeleteNotFound(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	profileID := createTestProfileViaAPI(t, mux, "Delete Not Found Profile")

	rr := doRequest(t, mux, http.MethodDelete, "/api/profiles/"+profileID+"/rules/non-existent-rule", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("DELETE non-existent rule status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}
