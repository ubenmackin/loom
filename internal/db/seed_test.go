package db

import (
	"context"
	"database/sql"
	"testing"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"

	_ "modernc.org/sqlite"
)

// setupSeedTestDB opens an in-memory sqlite database, applies all migrations,
// and registers a cleanup to close it. Unlike testhelpers.SetupTestDB, this
// helper lives inside package db so it can call Migrate directly without
// creating an import cycle (testhelpers imports db).
func setupSeedTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := database.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("PRAGMA foreign_keys: %v", err)
	}
	if err := Migrate(database); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})
	return database
}

// TestSeedBackfillsPrompt asserts the post-migration-016 backfill loop in
// SeedDefaultAgentProfiles. The scenario:
//  1. An agent_profiles row already exists with prompt=” and a name that
//     maps to a known built-in role (e.g. "executor").
//  2. SeedDefaultAgentProfiles runs the backfill pass.
//  3. After the call, the row's prompt column is non-empty (the default
//     ExecutorPrompt const was stamped onto it).
func TestSeedBackfillsPrompt(t *testing.T) {
	dbConn := setupSeedTestDB(t)
	profileStore := store.NewAgentProfileStore(dbConn)
	ctx := context.Background()

	// Pre-seed one profile row with a blank prompt.
	pre := &models.AgentProfile{
		ID:             "prof-001",
		Name:           "executor",
		AgentRole:      "executor",
		MaxConcurrency: 2,
		Prompt:         "",
	}
	if err := profileStore.Create(ctx, pre); err != nil {
		t.Fatalf("Create pre-existing profile: %v", err)
	}

	// Run the backfill pass. The store already has one profile (created
	// above), so the seed function will enter the backfill branch and
	// should stamp the prompt.
	if err := SeedDefaultAgentProfiles(ctx, profileStore); err != nil {
		t.Fatalf("SeedDefaultAgentProfiles: %v", err)
	}

	got, err := profileStore.GetByID(ctx, pre.ID)
	if err != nil {
		t.Fatalf("GetByID after backfill: %v", err)
	}
	if got.Prompt == "" {
		t.Fatal("prompt is empty after SeedDefaultAgentProfiles — backfill did not run")
	}
	if got.Prompt != ExecutorPrompt {
		t.Errorf("prompt = %q, want %q", got.Prompt, ExecutorPrompt)
	}
}

// TestSeedBackfillsPrompt_Idempotent verifies a second run of the backfill
// does not overwrite a profile that already has a non-empty prompt.
func TestSeedBackfillsPrompt_Idempotent(t *testing.T) {
	dbConn := setupSeedTestDB(t)
	profileStore := store.NewAgentProfileStore(dbConn)
	ctx := context.Background()

	const custom = "Do what I say."
	pre := &models.AgentProfile{
		ID:             "prof-custom",
		Name:           "executor",
		AgentRole:      "executor",
		MaxConcurrency: 1,
		Prompt:         custom,
	}
	if err := profileStore.Create(ctx, pre); err != nil {
		t.Fatalf("Create pre-existing profile with custom prompt: %v", err)
	}

	// First run: should be a no-op (already has prompt).
	if err := SeedDefaultAgentProfiles(ctx, profileStore); err != nil {
		t.Fatalf("SeedDefaultAgentProfiles (first): %v", err)
	}

	got, err := profileStore.GetByID(ctx, pre.ID)
	if err != nil {
		t.Fatalf("GetByID after first backfill: %v", err)
	}
	if got.Prompt != custom {
		t.Errorf("prompt = %q, want %q (custom prompt must be preserved)", got.Prompt, custom)
	}
}

// TestSeedDefaultAgentProfiles_CreatesOnEmptyTable is a sanity check: when
// the agent_profiles table is empty, SeedDefaultAgentProfiles seeds the 7
// built-in default profiles and each one carries a non-empty prompt.
func TestSeedDefaultAgentProfiles_CreatesOnEmptyTable(t *testing.T) {
	dbConn := setupSeedTestDB(t)
	profileStore := store.NewAgentProfileStore(dbConn)
	ctx := context.Background()

	if err := SeedDefaultAgentProfiles(ctx, profileStore); err != nil {
		t.Fatalf("SeedDefaultAgentProfiles (empty table): %v", err)
	}

	all, err := profileStore.List(ctx)
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(all) < 7 {
		t.Fatalf("expected at least 7 seeded profiles, got %d", len(all))
	}

	for _, p := range all {
		if p.Prompt == "" {
			t.Errorf("seeded profile %q has empty prompt", p.Name)
		}
	}
}
