package gateway

import (
	"context"
	"sync"
	"testing"

	"github.com/ubenmackin/loom/internal/dispatcher"
	"github.com/ubenmackin/loom/internal/models"
)

// ---------------------------------------------------------------------------
// Mock stores for testing
// ---------------------------------------------------------------------------

type mockProfileStore struct {
	profiles []*models.AgentProfile
}

func (m *mockProfileStore) List(_ context.Context) ([]*models.AgentProfile, error) {
	return m.profiles, nil
}

func (m *mockProfileStore) GetByID(_ context.Context, id string) (*models.AgentProfile, error) {
	for _, p := range m.profiles {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, nil
}

type mockTaskStore struct {
	mu    sync.Mutex
	tasks map[string]*models.Task
}

func (m *mockTaskStore) GetByID(_ context.Context, id string) (*models.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tasks[id], nil
}

func (m *mockTaskStore) UpdateStatus(_ context.Context, _ string, _ models.Status) error {
	return nil
}

func (m *mockTaskStore) GetByStory(_ context.Context, _ string) ([]*models.Task, error) {
	return nil, nil
}

type mockSessionStore struct{}

func (m *mockSessionStore) Register(_ context.Context, _ *models.Session) error { return nil }
func (m *mockSessionStore) GetByID(_ context.Context, _ string) (*models.Session, error) {
	return nil, nil
}
func (m *mockSessionStore) UpdateLastSeen(_ context.Context, _ string) error     { return nil }
func (m *mockSessionStore) ListAll(_ context.Context) ([]*models.Session, error) { return nil, nil }
func (m *mockSessionStore) Disconnect(_ context.Context, _ string) error         { return nil }

// ---------------------------------------------------------------------------
// Helper to create a minimal test gateway
// ---------------------------------------------------------------------------

func newTestGateway(profiles []*models.AgentProfile) *Gateway {
	g := &Gateway{
		queue:            NewJobQueue(),
		profileTaskTypes: make(map[string][]string),
		profileStore:     &mockProfileStore{profiles: profiles},
		taskStore:        &mockTaskStore{tasks: make(map[string]*models.Task)},
		sessionStore:     &mockSessionStore{},
	}
	return g
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestLoadProfiles_PopulatesProfileTaskTypes(t *testing.T) {
	profiles := []*models.AgentProfile{
		{ID: "1", Name: "Coder", TaskTypes: []string{"code", "build"}, MaxConcurrency: 1},
		{ID: "2", Name: "Reviewer", TaskTypes: []string{"review"}, MaxConcurrency: 1},
		{ID: "3", Name: "Planner", TaskTypes: []string{"planning"}, MaxConcurrency: 1},
	}

	g := newTestGateway(profiles)
	err := g.loadProfiles(context.Background())
	if err != nil {
		t.Fatalf("loadProfiles() returned error: %v", err)
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	if len(g.profileTaskTypes) != 3 {
		t.Fatalf("profileTaskTypes length = %d, want 3", len(g.profileTaskTypes))
	}

	expected := map[string][]string{
		"Coder":    {"code", "build"},
		"Reviewer": {"review"},
		"Planner":  {"planning"},
	}

	for name, expectedTypes := range expected {
		got, ok := g.profileTaskTypes[name]
		if !ok {
			t.Errorf("missing profileTaskTypes entry for %q", name)
			continue
		}
		if len(got) != len(expectedTypes) {
			t.Errorf("profileTaskTypes[%q] length = %d, want %d", name, len(got), len(expectedTypes))
			continue
		}
		for i := range got {
			if got[i] != expectedTypes[i] {
				t.Errorf("profileTaskTypes[%q][%d] = %q, want %q", name, i, got[i], expectedTypes[i])
			}
		}
	}
}

func TestResolveAgentType_MatchByTaskType(t *testing.T) {
	profiles := []*models.AgentProfile{
		{ID: "1", Name: "CodeAgent", TaskTypes: []string{"code", "build"}, MaxConcurrency: 1},
		{ID: "2", Name: "ReviewAgent", TaskTypes: []string{"review"}, MaxConcurrency: 1},
	}

	g := newTestGateway(profiles)
	err := g.loadProfiles(context.Background())
	if err != nil {
		t.Fatalf("loadProfiles() returned error: %v", err)
	}

	// Add a task with task_type "code" and no explicit AgentType.
	g.taskStore.(*mockTaskStore).tasks["task-1"] = &models.Task{
		ID:       "task-1",
		TaskType: models.TaskTypeCode,
	}

	event := dispatcher.Event{
		TaskID: "task-1",
	}

	agentType := g.resolveAgentType(context.Background(), event)
	if agentType != "CodeAgent" {
		t.Errorf("resolveAgentType() = %q, want %q", agentType, "CodeAgent")
	}
}

func TestResolveAgentType_FallbackToTaskAgentType(t *testing.T) {
	profiles := []*models.AgentProfile{
		{ID: "1", Name: "CodeAgent", TaskTypes: []string{"code"}, MaxConcurrency: 1},
	}

	g := newTestGateway(profiles)
	_ = g.loadProfiles(context.Background())

	// Task has task_type "review" which no profile handles, but has an explicit AgentType.
	g.taskStore.(*mockTaskStore).tasks["task-1"] = &models.Task{
		ID:        "task-1",
		TaskType:  models.TaskTypeReview,
		AgentType: "fallback-agent",
	}

	event := dispatcher.Event{
		TaskID: "task-1",
	}

	agentType := g.resolveAgentType(context.Background(), event)
	if agentType != "fallback-agent" {
		t.Errorf("resolveAgentType() = %q, want %q", agentType, "fallback-agent")
	}
}

func TestResolveAgentType_NoMatchReturnsEmpty(t *testing.T) {
	profiles := []*models.AgentProfile{
		{ID: "1", Name: "CodeAgent", TaskTypes: []string{"code"}, MaxConcurrency: 1},
	}

	g := newTestGateway(profiles)
	_ = g.loadProfiles(context.Background())

	// Task with "planning" task_type, no profiles handle "planning".
	g.taskStore.(*mockTaskStore).tasks["task-1"] = &models.Task{
		ID:       "task-1",
		TaskType: models.TaskTypePlanning,
	}

	event := dispatcher.Event{
		TaskID: "task-1",
	}

	agentType := g.resolveAgentType(context.Background(), event)
	if agentType != "" {
		t.Errorf("resolveAgentType() = %q, want empty string", agentType)
	}
}

func TestResolveAgentType_DeterministicPickWhenMultipleMatch(t *testing.T) {
	// Both profiles handle "code". Should pick the first alphabetically.
	profiles := []*models.AgentProfile{
		{ID: "1", Name: "ZuluAgent", TaskTypes: []string{"code"}, MaxConcurrency: 1},
		{ID: "2", Name: "AlphaAgent", TaskTypes: []string{"code"}, MaxConcurrency: 1},
	}

	g := newTestGateway(profiles)
	_ = g.loadProfiles(context.Background())

	g.taskStore.(*mockTaskStore).tasks["task-1"] = &models.Task{
		ID:       "task-1",
		TaskType: models.TaskTypeCode,
	}

	event := dispatcher.Event{
		TaskID: "task-1",
	}

	agentType := g.resolveAgentType(context.Background(), event)
	if agentType != "AlphaAgent" {
		t.Errorf("resolveAgentType() = %q, want %q (first alphabetically)", agentType, "AlphaAgent")
	}
}

func TestResolveAgentType_ConcurrentSafety(t *testing.T) {
	profiles := []*models.AgentProfile{
		{ID: "1", Name: "CodeAgent", TaskTypes: []string{"code"}, MaxConcurrency: 1},
	}

	g := newTestGateway(profiles)
	_ = g.loadProfiles(context.Background())

	g.taskStore.(*mockTaskStore).tasks["task-1"] = &models.Task{
		ID:       "task-1",
		TaskType: models.TaskTypeCode,
	}

	event := dispatcher.Event{TaskID: "task-1"}

	// Run resolveAgentType concurrently with ReloadProfiles.
	done := make(chan struct{})
	const goroutines = 10
	for i := 0; i < goroutines; i++ {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					g.resolveAgentType(context.Background(), event)
				}
			}
		}()
	}

	// Reload while reads are happening.
	for i := 0; i < 100; i++ {
		_ = g.ReloadProfiles(context.Background())
	}

	close(done)

	// Verify no panics and final state is correct.
	agentType := g.resolveAgentType(context.Background(), event)
	if agentType != "CodeAgent" {
		t.Errorf("resolveAgentType() = %q, want %q", agentType, "CodeAgent")
	}
}

func TestReloadProfiles_PicksUpChanges(t *testing.T) {
	profileStore := &mockProfileStore{
		profiles: []*models.AgentProfile{
			{ID: "1", Name: "CodeAgent", TaskTypes: []string{"code"}, MaxConcurrency: 1},
		},
	}

	g := newTestGateway(profileStore.profiles)
	g.profileStore = profileStore // ensure the store is the mock
	_ = g.loadProfiles(context.Background())

	// Initially, no profile handles "build".
	g.taskStore.(*mockTaskStore).tasks["task-1"] = &models.Task{
		ID:       "task-1",
		TaskType: models.TaskTypeBuild,
	}

	event := dispatcher.Event{TaskID: "task-1"}
	agentType := g.resolveAgentType(context.Background(), event)
	if agentType != "" {
		t.Errorf("before reload: resolveAgentType() = %q, want empty string", agentType)
	}

	// Update the profile to also handle "build".
	profileStore.profiles[0].TaskTypes = []string{"code", "build"}
	_ = g.ReloadProfiles(context.Background())

	agentType = g.resolveAgentType(context.Background(), event)
	if agentType != "CodeAgent" {
		t.Errorf("after reload: resolveAgentType() = %q, want %q", agentType, "CodeAgent")
	}
}
