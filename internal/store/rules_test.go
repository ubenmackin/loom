package store

import (
	"context"
	"errors"
	"testing"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/testhelpers"
)

func createTestProfile(t *testing.T, profileStore *AgentProfileStore, ctx context.Context) *models.AgentProfile {
	t.Helper()
	profile := &models.AgentProfile{
		Name:           "Rule Test Profile",
		Description:    "Profile for rule testing",
		Capabilities:   `["code"]`,
		MaxConcurrency: 1,
	}
	if err := profileStore.Create(ctx, profile); err != nil {
		t.Fatalf("Create profile: %v", err)
	}
	return profile
}

func TestTriggerRuleCreate(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	profileStore := NewAgentProfileStore(dbConn)
	ruleStore := NewTriggerRuleStore(dbConn)
	ctx := context.Background()

	profile := createTestProfile(t, profileStore, ctx)

	rule := &models.TriggerRule{
		AgentProfileID: profile.ID,
		EventType:      "story.created",
		Action:         "assign_agent",
		Priority:       10,
		Enabled:        true,
	}

	if err := ruleStore.Create(ctx, rule); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if rule.ID == "" {
		t.Fatal("Create() did not generate an ID")
	}

	if rule.AgentProfileID != profile.ID {
		t.Errorf("Create() AgentProfileID = %q, want %q", rule.AgentProfileID, profile.ID)
	}
	if rule.EventType != "story.created" {
		t.Errorf("Create() EventType = %q, want %q", rule.EventType, "story.created")
	}
	if rule.Action != "assign_agent" {
		t.Errorf("Create() Action = %q, want %q", rule.Action, "assign_agent")
	}
	if rule.Priority != 10 {
		t.Errorf("Create() Priority = %d, want 10", rule.Priority)
	}
	if !rule.Enabled {
		t.Error("Create() Enabled = false, want true")
	}
	if rule.CreatedAt.IsZero() {
		t.Fatal("Create() CreatedAt is zero")
	}
}

func TestTriggerRuleGetByID(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	profileStore := NewAgentProfileStore(dbConn)
	ruleStore := NewTriggerRuleStore(dbConn)
	ctx := context.Background()

	profile := createTestProfile(t, profileStore, ctx)

	rule := &models.TriggerRule{
		AgentProfileID: profile.ID,
		EventType:      "task.done",
		Action:         "review_task",
		Priority:       5,
		Enabled:        false,
	}
	if err := ruleStore.Create(ctx, rule); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, err := ruleStore.GetByID(ctx, rule.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if got.ID != rule.ID {
		t.Errorf("GetByID() ID = %q, want %q", got.ID, rule.ID)
	}
	if got.AgentProfileID != profile.ID {
		t.Errorf("GetByID() AgentProfileID = %q, want %q", got.AgentProfileID, profile.ID)
	}
	if got.EventType != rule.EventType {
		t.Errorf("GetByID() EventType = %q, want %q", got.EventType, rule.EventType)
	}
	if got.Action != rule.Action {
		t.Errorf("GetByID() Action = %q, want %q", got.Action, rule.Action)
	}
	if got.Priority != rule.Priority {
		t.Errorf("GetByID() Priority = %d, want %d", got.Priority, rule.Priority)
	}
	if got.Enabled != rule.Enabled {
		t.Errorf("GetByID() Enabled = %v, want %v", got.Enabled, rule.Enabled)
	}
}

func TestTriggerRuleGetByID_NotFound(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	ruleStore := NewTriggerRuleStore(dbConn)
	ctx := context.Background()

	_, err := ruleStore.GetByID(ctx, "non-existent-rule")
	if err == nil {
		t.Fatal("GetByID() expected error for non-existent ID, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetByID() error = %v, want ErrNotFound", err)
	}
}

func TestTriggerRuleListByProfile(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	profileStore := NewAgentProfileStore(dbConn)
	ruleStore := NewTriggerRuleStore(dbConn)
	ctx := context.Background()

	profile := createTestProfile(t, profileStore, ctx)

	rules := []*models.TriggerRule{
		{AgentProfileID: profile.ID, EventType: "story.created", Action: "assign_agent", Priority: 1, Enabled: true},
		{AgentProfileID: profile.ID, EventType: "task.done", Action: "review_task", Priority: 2, Enabled: true},
		{AgentProfileID: profile.ID, EventType: "build.failed", Action: "notify", Priority: 3, Enabled: false},
	}

	for _, r := range rules {
		if err := ruleStore.Create(ctx, r); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	got, err := ruleStore.ListByProfile(ctx, profile.ID)
	if err != nil {
		t.Fatalf("ListByProfile() error = %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("ListByProfile() returned %d rules, want 3", len(got))
	}

	// Verify ordering by priority.
	if got[0].Priority != 1 || got[1].Priority != 2 || got[2].Priority != 3 {
		t.Errorf("ListByProfile() order by priority = [%d %d %d], want [1 2 3]",
			got[0].Priority, got[1].Priority, got[2].Priority)
	}
}

func TestTriggerRuleListByProfile_Empty(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	profileStore := NewAgentProfileStore(dbConn)
	ruleStore := NewTriggerRuleStore(dbConn)
	ctx := context.Background()

	profile := createTestProfile(t, profileStore, ctx)

	rules, err := ruleStore.ListByProfile(ctx, profile.ID)
	if err != nil {
		t.Fatalf("ListByProfile() error = %v", err)
	}

	if len(rules) != 0 {
		t.Fatalf("ListByProfile() returned %d rules, want 0", len(rules))
	}
}

func TestTriggerRuleList(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	profileStore := NewAgentProfileStore(dbConn)
	ruleStore := NewTriggerRuleStore(dbConn)
	ctx := context.Background()

	profileA := createTestProfile(t, profileStore, ctx)
	profileB := &models.AgentProfile{Name: "Profile B", MaxConcurrency: 1}
	if err := profileStore.Create(ctx, profileB); err != nil {
		t.Fatalf("Create profile B: %v", err)
	}

	ruleA := &models.TriggerRule{AgentProfileID: profileA.ID, EventType: "e1", Action: "a1", Priority: 1, Enabled: true}
	ruleB := &models.TriggerRule{AgentProfileID: profileB.ID, EventType: "e2", Action: "a2", Priority: 2, Enabled: false}
	if err := ruleStore.Create(ctx, ruleA); err != nil {
		t.Fatalf("Create rule A: %v", err)
	}
	if err := ruleStore.Create(ctx, ruleB); err != nil {
		t.Fatalf("Create rule B: %v", err)
	}

	all, err := ruleStore.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(all) != 2 {
		t.Fatalf("List() returned %d rules, want 2", len(all))
	}
}

func TestTriggerRuleUpdate(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	profileStore := NewAgentProfileStore(dbConn)
	ruleStore := NewTriggerRuleStore(dbConn)
	ctx := context.Background()

	profile := createTestProfile(t, profileStore, ctx)

	rule := &models.TriggerRule{
		AgentProfileID: profile.ID,
		EventType:      "story.created",
		Action:         "assign_agent",
		Priority:       1,
		Enabled:        true,
	}
	if err := ruleStore.Create(ctx, rule); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	rule.EventType = "task.updated"
	rule.Action = "notify_agents"
	rule.Priority = 99
	rule.Enabled = false

	if err := ruleStore.Update(ctx, rule); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, err := ruleStore.GetByID(ctx, rule.ID)
	if err != nil {
		t.Fatalf("GetByID() after update error = %v", err)
	}

	if got.EventType != "task.updated" {
		t.Errorf("Update() EventType = %q, want %q", got.EventType, "task.updated")
	}
	if got.Action != "notify_agents" {
		t.Errorf("Update() Action = %q, want %q", got.Action, "notify_agents")
	}
	if got.Priority != 99 {
		t.Errorf("Update() Priority = %d, want 99", got.Priority)
	}
	if got.Enabled != false {
		t.Error("Update() Enabled = true, want false")
	}
}

func TestTriggerRuleUpdate_NotFound(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	ruleStore := NewTriggerRuleStore(dbConn)
	ctx := context.Background()

	rule := &models.TriggerRule{
		ID:        "non-existent",
		EventType: "test.event",
		Action:    "test_action",
	}

	err := ruleStore.Update(ctx, rule)
	if err == nil {
		t.Fatal("Update() expected error for non-existent ID, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Update() error = %v, want ErrNotFound", err)
	}
}

func TestTriggerRuleDelete(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	profileStore := NewAgentProfileStore(dbConn)
	ruleStore := NewTriggerRuleStore(dbConn)
	ctx := context.Background()

	profile := createTestProfile(t, profileStore, ctx)

	rule := &models.TriggerRule{
		AgentProfileID: profile.ID,
		EventType:      "test.event",
		Action:         "test_action",
		Priority:       1,
		Enabled:        true,
	}
	if err := ruleStore.Create(ctx, rule); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := ruleStore.Delete(ctx, rule.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := ruleStore.GetByID(ctx, rule.ID)
	if err == nil {
		t.Fatal("Delete() rule still exists after deletion")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete() GetByID error = %v, want ErrNotFound", err)
	}
}

func TestTriggerRuleDelete_NotFound(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	ruleStore := NewTriggerRuleStore(dbConn)
	ctx := context.Background()

	err := ruleStore.Delete(ctx, "non-existent-rule")
	if err == nil {
		t.Fatal("Delete() expected error for non-existent ID, got nil")
	}
}
