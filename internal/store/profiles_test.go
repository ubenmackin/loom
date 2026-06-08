package store

import (
	"context"
	"errors"
	"testing"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/testhelpers"
)

func TestAgentProfileCreate(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	profileStore := NewAgentProfileStore(dbConn)
	ctx := context.Background()

	profile := &models.AgentProfile{
		Name:           "Test Agent",
		Description:    "A test agent profile",
		Capabilities:   `["code","build"]`,
		MaxConcurrency: 3,
	}

	if err := profileStore.Create(ctx, profile); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if profile.ID == "" {
		t.Fatal("Create() did not generate an ID")
	}

	if profile.Name != "Test Agent" {
		t.Errorf("Create() Name = %q, want %q", profile.Name, "Test Agent")
	}

	if profile.MaxConcurrency != 3 {
		t.Errorf("Create() MaxConcurrency = %d, want 3", profile.MaxConcurrency)
	}

	if profile.CreatedAt.IsZero() {
		t.Fatal("Create() CreatedAt is zero")
	}

	if profile.UpdatedAt.IsZero() {
		t.Fatal("Create() UpdatedAt is zero")
	}
}

func TestAgentProfileGetByID(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	profileStore := NewAgentProfileStore(dbConn)
	ctx := context.Background()

	profile := &models.AgentProfile{
		Name:           "Get Test",
		Description:    "Get test description",
		Capabilities:   `["code"]`,
		MaxConcurrency: 2,
	}

	if err := profileStore.Create(ctx, profile); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := profileStore.GetByID(ctx, profile.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if got.ID != profile.ID {
		t.Errorf("GetByID() ID = %q, want %q", got.ID, profile.ID)
	}
	if got.Name != profile.Name {
		t.Errorf("GetByID() Name = %q, want %q", got.Name, profile.Name)
	}
	if got.Description != profile.Description {
		t.Errorf("GetByID() Description = %q, want %q", got.Description, profile.Description)
	}
	if got.MaxConcurrency != profile.MaxConcurrency {
		t.Errorf("GetByID() MaxConcurrency = %d, want %d", got.MaxConcurrency, profile.MaxConcurrency)
	}
}

func TestAgentProfileGetByID_NotFound(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	profileStore := NewAgentProfileStore(dbConn)
	ctx := context.Background()

	_, err := profileStore.GetByID(ctx, "non-existent-id")
	if err == nil {
		t.Fatal("GetByID() expected error for non-existent ID, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetByID() error = %v, want ErrNotFound", err)
	}
}

func TestAgentProfileList(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	profileStore := NewAgentProfileStore(dbConn)
	ctx := context.Background()

	profiles := []*models.AgentProfile{
		{Name: "Agent Alpha", Capabilities: `["code"]`, MaxConcurrency: 1},
		{Name: "Agent Beta", Capabilities: `["build"]`, MaxConcurrency: 2},
		{Name: "Agent Gamma", Capabilities: `["review"]`, MaxConcurrency: 3},
	}

	for _, p := range profiles {
		if err := profileStore.Create(ctx, p); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	all, err := profileStore.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(all) != 3 {
		t.Fatalf("List() returned %d profiles, want 3", len(all))
	}

	// Verify ordering by name.
	names := make([]string, len(all))
	for i, p := range all {
		names[i] = p.Name
	}
	if names[0] != "Agent Alpha" || names[1] != "Agent Beta" || names[2] != "Agent Gamma" {
		t.Errorf("List() order = %v, want [Agent Alpha Agent Beta Agent Gamma]", names)
	}
}

func TestAgentProfileUpdate(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	profileStore := NewAgentProfileStore(dbConn)
	ctx := context.Background()

	profile := &models.AgentProfile{
		Name:           "Original Name",
		Description:    "Original description",
		Capabilities:   `["code"]`,
		MaxConcurrency: 1,
	}

	if err := profileStore.Create(ctx, profile); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	profile.Name = "Updated Name"
	profile.Description = "Updated description"
	profile.Capabilities = `["code","build","review"]`
	profile.MaxConcurrency = 5

	if err := profileStore.Update(ctx, profile); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, err := profileStore.GetByID(ctx, profile.ID)
	if err != nil {
		t.Fatalf("GetByID() after update error = %v", err)
	}

	if got.Name != "Updated Name" {
		t.Errorf("Update() Name = %q, want %q", got.Name, "Updated Name")
	}
	if got.Description != "Updated description" {
		t.Errorf("Update() Description = %q, want %q", got.Description, "Updated description")
	}
	if got.Capabilities != `["code","build","review"]` {
		t.Errorf("Update() Capabilities = %q, want %q", got.Capabilities, `["code","build","review"]`)
	}
	if got.MaxConcurrency != 5 {
		t.Errorf("Update() MaxConcurrency = %d, want 5", got.MaxConcurrency)
	}
}

func TestAgentProfileUpdate_NotFound(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	profileStore := NewAgentProfileStore(dbConn)
	ctx := context.Background()

	profile := &models.AgentProfile{
		ID:             "non-existent",
		Name:           "Ghost",
		MaxConcurrency: 1,
	}

	err := profileStore.Update(ctx, profile)
	if err == nil {
		t.Fatal("Update() expected error for non-existent ID, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Update() error = %v, want ErrNotFound", err)
	}
}

func TestAgentProfileDelete(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	profileStore := NewAgentProfileStore(dbConn)
	ctx := context.Background()

	profile := &models.AgentProfile{
		Name:           "Delete Test",
		MaxConcurrency: 1,
	}

	if err := profileStore.Create(ctx, profile); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := profileStore.Delete(ctx, profile.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := profileStore.GetByID(ctx, profile.ID)
	if err == nil {
		t.Fatal("Delete() profile still exists after deletion")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete() GetByID error = %v, want ErrNotFound", err)
	}
}

func TestAgentProfileDelete_NotFound(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	profileStore := NewAgentProfileStore(dbConn)
	ctx := context.Background()

	err := profileStore.Delete(ctx, "non-existent-id")
	if err == nil {
		t.Fatal("Delete() expected error for non-existent ID, got nil")
	}
}
