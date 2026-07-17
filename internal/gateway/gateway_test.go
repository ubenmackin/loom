package gateway

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ubenmackin/loom/internal/acp"
	"github.com/ubenmackin/loom/internal/dispatcher"
	"github.com/ubenmackin/loom/internal/models"
)

// gitCommitCmd returns an exec.Cmd for "git commit" with the author/committer
// env vars set so it works in CI where no global git config exists.
func gitCommitCmd(dir string, args ...string) *exec.Cmd {
	cmdArgs := append([]string{"commit"}, args...)
	cmd := exec.Command("git", cmdArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=CI Test",
		"GIT_AUTHOR_EMAIL=ci@test.local",
		"GIT_COMMITTER_NAME=CI Test",
		"GIT_COMMITTER_EMAIL=ci@test.local",
	)
	return cmd
}

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

func (m *mockTaskStore) Update(_ context.Context, _ *models.Task) error {
	return nil
}

type mockSessionStore struct{}

func (m *mockSessionStore) Register(_ context.Context, _ *models.Session) error { return nil }
func (m *mockSessionStore) GetByID(_ context.Context, _ string) (*models.Session, error) {
	return nil, nil
}
func (m *mockSessionStore) UpdateLastSeen(_ context.Context, _ string) error     { return nil }
func (m *mockSessionStore) ListAll(_ context.Context) ([]*models.Session, error) { return nil, nil }
func (m *mockSessionStore) Disconnect(_ context.Context, _ string) error         { return nil }

type mockSettingStore struct {
	data map[string]string
}

func (m *mockSettingStore) Get(_ context.Context, key string) (string, error) {
	v, ok := m.data[key]
	if !ok {
		return "", sql.ErrNoRows
	}
	return v, nil
}

// ---------------------------------------------------------------------------
// Helper to create a minimal test gateway
// ---------------------------------------------------------------------------

func newTestGateway(profiles []*models.AgentProfile) *Gateway {
	g := &Gateway{
		queue:             NewJobQueue(),
		acpClients:        make(map[string]*acp.Client),
		sessionIDtoClient: make(map[string]*acp.Client),
		profileTaskTypes:  make(map[string][]string),
		filesInUse:        make(map[string]string),
		profileStore:      &mockProfileStore{profiles: profiles},
		taskStore:         &mockTaskStore{tasks: make(map[string]*models.Task)},
		sessionStore:      &mockSessionStore{},
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

func TestNewGateway_LoadsGlobalMaxConcurrency(t *testing.T) {
	ss := &mockSettingStore{
		data: map[string]string{"global_max_concurrency": "3"},
	}

	d := dispatcher.NewDispatcher(dispatcher.DispatcherDeps{})
	gw := NewGateway(
		d,
		"opencode acp",
		5, // maxTotal — should be overridden by setting store
		&mockTaskStore{tasks: make(map[string]*models.Task)},
		&mockSessionStore{},
		nil, // projectStore
		nil, // storyStore
		nil, // commentStore
		nil, // activityStore
		&mockProfileStore{},
		ss,
		nil, // hub — no WebSocket broadcasts in tests
	)

	// With maxTotal=3, we should be able to increment 3 distinct
	// (projectID, agentType) pairs, but the 4th should be silently
	// dropped because the global cap has been reached.
	gw.queue.IncrementActive("p1", "executor")
	gw.queue.IncrementActive("p2", "planner")
	gw.queue.IncrementActive("p3", "builder")

	if gw.queue.HasCapacity("p4", "reviewer") {
		t.Fatal("expected HasCapacity to return false after 3 active sessions with maxTotal=3")
	}

	// Verify the default maxTotal of 5 was actually overridden by
	// creating a 4th session that should be silently dropped.
	if gw.queue.HasCapacity("p5", "executor") {
		t.Fatal("expected no capacity after reaching maxTotal=3 even with a new agent type")
	}
}

func TestNewGateway_NoSettingStore_UsesDefaultMaxTotal(t *testing.T) {
	d := dispatcher.NewDispatcher(dispatcher.DispatcherDeps{})
	gw := NewGateway(
		d,
		"opencode acp",
		5, // maxTotal — should remain as-is when no setting store
		&mockTaskStore{tasks: make(map[string]*models.Task)},
		&mockSessionStore{},
		nil, // projectStore
		nil, // storyStore
		nil, // commentStore
		nil, // activityStore
		&mockProfileStore{},
		nil, // settingStore — nil, so default should be used
		nil, // hub — no WebSocket broadcasts in tests
	)

	// With maxTotal=5 (default), we should be able to increment 5
	// distinct pairs before hitting the cap.
	gw.queue.IncrementActive("p1", "executor")
	gw.queue.IncrementActive("p2", "planner")
	gw.queue.IncrementActive("p3", "builder")
	gw.queue.IncrementActive("p4", "reviewer")
	gw.queue.IncrementActive("p5", "planner")

	if gw.queue.HasCapacity("p6", "executor") {
		t.Fatal("expected HasCapacity to return false after 5 active sessions with maxTotal=5")
	}
}

func TestNewGateway_SettingStoreReturnsError_UsesDefaultMaxTotal(t *testing.T) {
	// Setting store exists but the key is missing — Get returns sql.ErrNoRows.
	ss := &mockSettingStore{data: map[string]string{}}

	d := dispatcher.NewDispatcher(dispatcher.DispatcherDeps{})
	gw := NewGateway(
		d,
		"opencode acp",
		5, // maxTotal — should remain as-is when the setting is not found
		&mockTaskStore{tasks: make(map[string]*models.Task)},
		&mockSessionStore{},
		nil, // projectStore
		nil, // storyStore
		nil, // commentStore
		nil, // activityStore
		&mockProfileStore{},
		ss,
		nil, // hub — no WebSocket broadcasts in tests
	)

	// With maxTotal=5 (default), we should be able to increment 5
	// distinct pairs before hitting the cap.
	gw.queue.IncrementActive("p1", "a")
	gw.queue.IncrementActive("p2", "b")
	gw.queue.IncrementActive("p3", "c")
	gw.queue.IncrementActive("p4", "d")
	gw.queue.IncrementActive("p5", "e")

	if gw.queue.HasCapacity("p6", "f") {
		t.Fatal("expected HasCapacity to return false after 5 active sessions with maxTotal=5")
	}
}

func TestNewGateway_ZeroMaxTotalIsUnlimited(t *testing.T) {
	ss := &mockSettingStore{
		data: map[string]string{"global_max_concurrency": "0"},
	}

	d := dispatcher.NewDispatcher(dispatcher.DispatcherDeps{})
	gw := NewGateway(
		d,
		"opencode acp",
		5, // maxTotal — should be overridden by setting store value of 0
		&mockTaskStore{tasks: make(map[string]*models.Task)},
		&mockSessionStore{},
		nil, // projectStore
		nil, // storyStore
		nil, // commentStore
		nil, // activityStore
		&mockProfileStore{},
		ss,
		nil, // hub — no WebSocket broadcasts in tests
	)

	// With maxTotal=0 (unlimited), HasCapacity should always return true
	// regardless of how many sessions are active.
	for i := 0; i < 10; i++ {
		projectID := string(rune('a' + i))
		gw.queue.IncrementActive(projectID, "executor")
	}

	if !gw.queue.HasCapacity("zzz", "executor") {
		t.Fatal("expected HasCapacity to return true when maxTotal=0 (unlimited)")
	}
}

func TestNewGateway_InvalidSettingValue_UsesDefaultMaxTotal(t *testing.T) {
	ss := &mockSettingStore{
		data: map[string]string{"global_max_concurrency": "not-a-number"},
	}

	d := dispatcher.NewDispatcher(dispatcher.DispatcherDeps{})
	gw := NewGateway(
		d,
		"opencode acp",
		5, // maxTotal — should remain as-is when setting value is invalid
		&mockTaskStore{tasks: make(map[string]*models.Task)},
		&mockSessionStore{},
		nil, // projectStore
		nil, // storyStore
		nil, // commentStore
		nil, // activityStore
		&mockProfileStore{},
		ss,
		nil, // hub — no WebSocket broadcasts in tests
	)

	// With maxTotal=5 (default), we should be able to increment 5
	// distinct pairs before hitting the cap.
	gw.queue.IncrementActive("p1", "a")
	gw.queue.IncrementActive("p2", "b")
	gw.queue.IncrementActive("p3", "c")
	gw.queue.IncrementActive("p4", "d")
	gw.queue.IncrementActive("p5", "e")

	if gw.queue.HasCapacity("p6", "f") {
		t.Fatal("expected HasCapacity to return false after 5 active sessions with maxTotal=5")
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

type mockStoryStore struct {
	mu      sync.Mutex
	stories map[string]*models.Story
}

func (m *mockStoryStore) GetByID(_ context.Context, id string) (*models.Story, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.stories[id]
	if !ok {
		return nil, fmt.Errorf("story %q not found", id)
	}
	return s, nil
}

func (m *mockStoryStore) Update(_ context.Context, story *models.Story) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stories[story.ID] = story
	return nil
}

// ---------------------------------------------------------------------------
// File collision tests
// ---------------------------------------------------------------------------

// TestFileCollision_DeferOnOverlap verifies that when two tasks claim the same
// file, the second task cannot acquire the file and is not dequeued until the
// first releases it.
func TestFileCollision_DeferOnOverlap(t *testing.T) {
	g := newTestGateway(nil)

	// Task 1 acquires foo.go.
	if !g.acquireFiles("task-1", []string{"foo.go"}) {
		t.Fatal("acquireFiles(task-1, [foo.go]) should succeed")
	}

	// Task 2 tries to acquire the same file — must fail.
	if g.acquireFiles("task-2", []string{"foo.go"}) {
		t.Fatal("acquireFiles(task-2, [foo.go]) should fail (collision)")
	}

	// Verify foo.go is still held by task-1.
	g.mu.Lock()
	holder := g.filesInUse["foo.go"]
	g.mu.Unlock()
	if holder != "task-1" {
		t.Errorf("foo.go held by %q, want %q", holder, "task-1")
	}
}

// TestFileCollision_ReleaseOnCompletion verifies that when a task holding a
// file completes and releases it, another task claiming the same file can
// acquire it.
func TestFileCollision_ReleaseOnCompletion(t *testing.T) {
	g := newTestGateway(nil)

	// Task 1 acquires foo.go.
	if !g.acquireFiles("task-1", []string{"foo.go"}) {
		t.Fatal("acquireFiles(task-1, [foo.go]) should succeed")
	}

	// Release files (simulating task completion).
	g.releaseFiles("task-1", []string{"foo.go"})

	// Verify foo.go is released.
	g.mu.Lock()
	_, ok := g.filesInUse["foo.go"]
	g.mu.Unlock()
	if ok {
		t.Fatal("foo.go should be released after releaseFiles")
	}

	// Task 2 should now be able to acquire foo.go.
	if !g.acquireFiles("task-2", []string{"foo.go"}) {
		t.Fatal("acquireFiles(task-2, [foo.go]) should succeed after release")
	}
}

// ---------------------------------------------------------------------------
// Worktree tests
// ---------------------------------------------------------------------------

// TestEnsureWorktree_AppliesCwdWithWorktreeRootOverride verifies that when a
// custom git_worktree_root setting is provided, ensureWorktree uses it as the
// worktree root path.
func TestEnsureWorktree_AppliesCwdWithWorktreeRootOverride(t *testing.T) {
	// Create a temp git repository.
	repoDir, err := os.MkdirTemp("", "loom-worktree-test-*")
	if err != nil {
		t.Fatalf("failed to create temp repo dir: %v", err)
	}
	defer os.RemoveAll(repoDir)

	initCmd := exec.Command("git", "init")
	initCmd.Dir = repoDir
	if err := initCmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	writeFile := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(writeFile, []byte("# test"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	addCmd := exec.Command("git", "add", "README.md")
	addCmd.Dir = repoDir
	if err := addCmd.Run(); err != nil {
		t.Fatalf("git add: %v", err)
	}
	commitCmd := gitCommitCmd(repoDir, "-m", "initial commit")
	if err := commitCmd.Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	// Custom worktree root via the setting store.
	customRoot := filepath.Join(repoDir, "custom-worktrees")

	ss := &mockSettingStore{
		data: map[string]string{SettingKeyGitWorktreeRoot: customRoot},
	}

	g := &Gateway{
		worktreeManager: NewWorktreeManager(filepath.Join(repoDir, ".loom", "worktrees")),
		settingStore:    ss,
		storyStore: &mockStoryStore{
			stories: map[string]*models.Story{
				"story-1": {
					ID:     "story-1",
					Title:  "Test Story",
					Status: models.StatusReady,
				},
			},
		},
	}

	ctx := context.Background()
	if err := g.ensureWorktree(ctx, "story-1", repoDir); err != nil {
		t.Fatalf("ensureWorktree() error = %v", err)
	}

	// Verify the worktree was created at the custom root.
	expectedPath := filepath.Join(customRoot, "story-1")
	if info, err := os.Stat(expectedPath); err != nil || !info.IsDir() {
		t.Fatalf("worktree path %q does not exist after ensureWorktree", expectedPath)
	}
}

// TestEnsureWorktree_FallsBackToDefaultRoot verifies that when no override is
// set, ensureWorktree uses the default .loom/worktrees/{storyID} path.
func TestEnsureWorktree_FallsBackToDefaultRoot(t *testing.T) {
	// Create a temp git repository.
	repoDir, err := os.MkdirTemp("", "loom-worktree-test-*")
	if err != nil {
		t.Fatalf("failed to create temp repo dir: %v", err)
	}
	defer os.RemoveAll(repoDir)

	initCmd := exec.Command("git", "init")
	initCmd.Dir = repoDir
	if err := initCmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	writeFile := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(writeFile, []byte("# test"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	addCmd := exec.Command("git", "add", "README.md")
	addCmd.Dir = repoDir
	if err := addCmd.Run(); err != nil {
		t.Fatalf("git add: %v", err)
	}
	commitCmd := gitCommitCmd(repoDir, "-m", "initial commit")
	if err := commitCmd.Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	defaultRoot := filepath.Join(repoDir, ".loom", "worktrees")

	g := &Gateway{
		worktreeManager: NewWorktreeManager(defaultRoot),
		storyStore: &mockStoryStore{
			stories: map[string]*models.Story{
				"story-1": {
					ID:     "story-1",
					Title:  "Test Story",
					Status: models.StatusReady,
				},
			},
		},
	}

	ctx := context.Background()
	if err := g.ensureWorktree(ctx, "story-1", repoDir); err != nil {
		t.Fatalf("ensureWorktree() error = %v", err)
	}

	// Verify the worktree was created at the default root.
	expectedPath := filepath.Join(defaultRoot, "story-1")
	if info, err := os.Stat(expectedPath); err != nil || !info.IsDir() {
		t.Fatalf("worktree path %q does not exist after ensureWorktree", expectedPath)
	}
}

// TestCreateWorktree_Idempotent calls CreateWorktree twice with the same
// storyID and expects both calls to succeed without error. It creates a
// temporary git repository, creates a worktree on the first call, and
// verifies that the second call returns immediately because the directory
// already exists.
func TestCreateWorktree_Idempotent(t *testing.T) {
	// Create a temporary directory for the git repository.
	repoDir, err := os.MkdirTemp("", "loom-worktree-test-*")
	if err != nil {
		t.Fatalf("failed to create temp repo dir: %v", err)
	}
	defer os.RemoveAll(repoDir)

	// Initialize a git repo in the temp directory.
	initCmd := exec.Command("git", "init")
	initCmd.Dir = repoDir
	if err := initCmd.Run(); err != nil {
		t.Fatalf("failed to git init: %v", err)
	}

	// Create an initial commit so that worktree add has a HEAD to branch from.
	// git worktree add requires at least one commit on the current branch.
	writeFile := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(writeFile, []byte("# test"), 0o644); err != nil {
		t.Fatalf("failed to create README.md: %v", err)
	}
	addCmd := exec.Command("git", "add", "README.md")
	addCmd.Dir = repoDir
	if err := addCmd.Run(); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}
	commitCmd := gitCommitCmd(repoDir, "-m", "initial commit")
	if err := commitCmd.Run(); err != nil {
		t.Fatalf("failed to git commit: %v", err)
	}

	// Create a worktree manager rooted inside the repo's own temp area.
	wmRoot := filepath.Join(repoDir, "worktrees")
	wm := NewWorktreeManager(wmRoot)

	storyID := "test-story-id"
	branchName := "feature/story-test-story-id"

	// First call — should create the worktree successfully.
	path1, branch1, err1 := wm.CreateWorktree(repoDir, storyID, branchName)
	if err1 != nil {
		t.Fatalf("CreateWorktree (first call): unexpected error: %v", err1)
	}
	if path1 == "" {
		t.Fatal("CreateWorktree (first call): returned empty path")
	}
	if branch1 != branchName {
		t.Fatalf("CreateWorktree (first call): branch = %q, want %q", branch1, branchName)
	}

	// Verify the worktree directory actually exists on disk.
	if info, err := os.Stat(path1); err != nil || !info.IsDir() {
		t.Fatalf("CreateWorktree (first call): worktree path %q is not a directory", path1)
	}

	// Second call — should be idempotent and return immediately.
	path2, branch2, err2 := wm.CreateWorktree(repoDir, storyID, branchName)
	if err2 != nil {
		t.Fatalf("CreateWorktree (second call): unexpected error: %v", err2)
	}
	if path2 == "" {
		t.Fatal("CreateWorktree (second call): returned empty path")
	}
	if path2 != path1 {
		t.Fatalf("CreateWorktree (second call): path = %q, want %q", path2, path1)
	}
	if branch2 != branch1 {
		t.Fatalf("CreateWorktree (second call): branch = %q, want %q", branch2, branch1)
	}
}
