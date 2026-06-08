package api

import (
	"net/http"
	"testing"

	"github.com/ubenmackin/loom/internal/models"
)

func TestProfiles_ListEmpty(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	rr := doRequest(t, mux, http.MethodGet, "/api/profiles", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/profiles status = %d, want %d", rr.Code, http.StatusOK)
	}

	var profiles []*models.AgentProfile
	decodeRespJSON(t, rr, &profiles)

	if len(profiles) != 0 {
		t.Fatalf("GET /api/profiles returned %d profiles, want 0", len(profiles))
	}
}

func TestProfiles_CreateAndGet(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	// Create a profile.
	createBody := map[string]any{
		"name":            "Test Agent",
		"description":     "A test agent profile",
		"capabilities":    `["code","build"]`,
		"max_concurrency": 3,
	}
	rr := doRequest(t, mux, http.MethodPost, "/api/profiles", createBody)
	if rr.Code != http.StatusCreated {
		t.Fatalf("POST /api/profiles status = %d, want %d", rr.Code, http.StatusCreated)
	}

	var created models.AgentProfile
	decodeRespJSON(t, rr, &created)

	if created.ID == "" {
		t.Fatal("POST /api/profiles did not return an ID")
	}
	if created.Name != "Test Agent" {
		t.Errorf("POST /api/profiles Name = %q, want %q", created.Name, "Test Agent")
	}
	if created.MaxConcurrency != 3 {
		t.Errorf("POST /api/profiles MaxConcurrency = %d, want 3", created.MaxConcurrency)
	}
	if created.CreatedAt.IsZero() {
		t.Fatal("POST /api/profiles CreatedAt is zero")
	}

	// Get the created profile.
	rr = doRequest(t, mux, http.MethodGet, "/api/profiles/"+created.ID, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/profiles/{id} status = %d, want %d", rr.Code, http.StatusOK)
	}

	var got models.AgentProfile
	decodeRespJSON(t, rr, &got)

	if got.ID != created.ID {
		t.Errorf("GET /api/profiles/{id} ID = %q, want %q", got.ID, created.ID)
	}
	if got.Name != "Test Agent" {
		t.Errorf("GET /api/profiles/{id} Name = %q, want %q", got.Name, "Test Agent")
	}
}

func TestProfiles_GetNotFound(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	rr := doRequest(t, mux, http.MethodGet, "/api/profiles/non-existent", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("GET /api/profiles/non-existent status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestProfiles_List(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	// Create two profiles.
	bodyA := map[string]any{"name": "Alpha", "max_concurrency": 1}
	rr := doRequest(t, mux, http.MethodPost, "/api/profiles", bodyA)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create profile Alpha status = %d, want %d", rr.Code, http.StatusCreated)
	}

	bodyB := map[string]any{"name": "Beta", "max_concurrency": 2}
	rr = doRequest(t, mux, http.MethodPost, "/api/profiles", bodyB)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create profile Beta status = %d, want %d", rr.Code, http.StatusCreated)
	}

	// List all profiles.
	rr = doRequest(t, mux, http.MethodGet, "/api/profiles", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/profiles status = %d, want %d", rr.Code, http.StatusOK)
	}

	var profiles []*models.AgentProfile
	decodeRespJSON(t, rr, &profiles)

	if len(profiles) < 2 {
		t.Fatalf("GET /api/profiles returned %d profiles, want at least 2", len(profiles))
	}
}

func TestProfiles_Update(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	// Create.
	createBody := map[string]any{"name": "Original", "max_concurrency": 1}
	rr := doRequest(t, mux, http.MethodPost, "/api/profiles", createBody)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create profile status = %d, want %d", rr.Code, http.StatusCreated)
	}

	var created models.AgentProfile
	decodeRespJSON(t, rr, &created)

	// Update.
	updateBody := map[string]any{
		"name":            "Updated",
		"description":     "Updated description",
		"capabilities":    `["review"]`,
		"max_concurrency": 5,
	}
	rr = doRequest(t, mux, http.MethodPut, "/api/profiles/"+created.ID, updateBody)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT /api/profiles/{id} status = %d, want %d", rr.Code, http.StatusOK)
	}

	var updated models.AgentProfile
	decodeRespJSON(t, rr, &updated)

	if updated.Name != "Updated" {
		t.Errorf("PUT /api/profiles/{id} Name = %q, want %q", updated.Name, "Updated")
	}
	if updated.MaxConcurrency != 5 {
		t.Errorf("PUT /api/profiles/{id} MaxConcurrency = %d, want 5", updated.MaxConcurrency)
	}

	// Verify via GET.
	rr = doRequest(t, mux, http.MethodGet, "/api/profiles/"+created.ID, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET after update status = %d, want %d", rr.Code, http.StatusOK)
	}

	var got models.AgentProfile
	decodeRespJSON(t, rr, &got)

	if got.Name != "Updated" {
		t.Errorf("GET after update Name = %q, want %q", got.Name, "Updated")
	}
	if got.Description != "Updated description" {
		t.Errorf("GET after update Description = %q, want %q", got.Description, "Updated description")
	}
}

func TestProfiles_UpdateNotFound(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	updateBody := map[string]any{"name": "Ghost", "max_concurrency": 1}
	rr := doRequest(t, mux, http.MethodPut, "/api/profiles/non-existent", updateBody)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("PUT /api/profiles/non-existent status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestProfiles_Delete(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	// Create.
	createBody := map[string]any{"name": "To Delete", "max_concurrency": 1}
	rr := doRequest(t, mux, http.MethodPost, "/api/profiles", createBody)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create profile status = %d, want %d", rr.Code, http.StatusCreated)
	}

	var created models.AgentProfile
	decodeRespJSON(t, rr, &created)

	// Delete.
	rr = doRequest(t, mux, http.MethodDelete, "/api/profiles/"+created.ID, nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/profiles/{id} status = %d, want %d", rr.Code, http.StatusNoContent)
	}

	// Verify deletion.
	rr = doRequest(t, mux, http.MethodGet, "/api/profiles/"+created.ID, nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("GET after delete status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestProfiles_DeleteNotFound(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	rr := doRequest(t, mux, http.MethodDelete, "/api/profiles/non-existent", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("DELETE /api/profiles/non-existent status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestProfiles_CreateMissingName(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, dbConn := newTestRouterWithDB(t)
	makeTestUserAdmin(t, dbConn)

	body := map[string]any{"max_concurrency": 1}
	rr := doRequest(t, mux, http.MethodPost, "/api/profiles", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/profiles with missing name status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

