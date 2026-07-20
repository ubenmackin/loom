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
		profilesByName:    make(map[string]*models.AgentProfile),
		filesInUse:        make(map[string]string),
		missingBlocks:     make(map[string]map[string]int),
		profileStore:      &mockProfileStore{profiles: profiles},
		taskStore:         &mockTaskStore{tasks: make(map[string]*models.Task)},
		sessionStore:      &mockSessionStore{},
	}
	return g
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

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

func TestReloadProfiles_PicksUpChanges(t *testing.T) {
	// Verify ReloadProfiles surfaces profile changes that affect the job
	// queue's concurrency configuration without requiring a server
	// restart. The profileTaskTypes capability-search map (which the
	// original version of this test asserted on) was removed in TASK-005;
	// routing now goes through task.AgentType directly. MaxConcurrency is
	// the remaining loadProfiles-cast gateway state, so we assert on it.
	profileStore := &mockProfileStore{
		profiles: []*models.AgentProfile{
			{ID: "1", Name: "CodeAgent", MaxConcurrency: 1},
		},
	}

	g := newTestGateway(profileStore.profiles)
	g.profileStore = profileStore // ensure the store is the mock
	_ = g.loadProfiles(context.Background())

	// Initially, the profile advertises MaxConcurrency=1.
	if got := g.queue.MaxConcurrency("CodeAgent"); got != 1 {
		t.Fatalf("before reload: MaxConcurrency(CodeAgent) = %d, want 1", got)
	}

	// Pump the profile's MaxConcurrency and reload.
	profileStore.profiles[0].MaxConcurrency = 4
	if err := g.ReloadProfiles(context.Background()); err != nil {
		t.Fatalf("ReloadProfiles() returned error: %v", err)
	}

	if got := g.queue.MaxConcurrency("CodeAgent"); got != 4 {
		t.Errorf("after reload: MaxConcurrency(CodeAgent) = %d, want 4", got)
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

// ---------------------------------------------------------------------------
// Session-mode routing tests (TASK-005 / TASK-010 — see Architectural
// Decision 9). The old TestResolveAgentType_* / TestLoadProfiles_
// PopulatesProfileTaskTypes tests (which asserted the now-removed
// profileTaskTypes capability-search) were deleted in favor of the
// following tests exercising the new agent-type → SetSessionConfigOption
// routing pipeline driven by task.AgentType + defaultRoleForTaskType.
// ---------------------------------------------------------------------------

// mockModeRoutingClient is the in-package test double for the
// modeRoutingClient interface declared alongside routeSessionMode in
// internal/gateway/gateway.go. It records every SetSessionConfigOption call
// so a test can assert the routed session/mode pair without standing up a
// real opencode subprocess.
type mockModeRoutingClient struct {
	mu    sync.Mutex
	calls []setSessionConfigOptionCall
}

type setSessionConfigOptionCall struct {
	sessionID string
	configID  string
	value     string
}

func (m *mockModeRoutingClient) SetSessionConfigOption(_ context.Context, sessionID, configID, value string) (*acp.SetSessionConfigOptionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, setSessionConfigOptionCall{
		sessionID: sessionID,
		configID:  configID,
		value:     value,
	})
	return &acp.SetSessionConfigOptionResponse{}, nil
}

// snapshot returns a copy of the recorded SetSessionConfigOption calls so a
// test can assert on them without racing against the production mutator.
func (m *mockModeRoutingClient) snapshot() []setSessionConfigOptionCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]setSessionConfigOptionCall, len(m.calls))
	copy(out, m.calls)
	return out
}

// TestSetSessionConfigOption_OnNewSession_SwitchesMode exercises the happy
// path through routeSessionMode for a freshly-created session whose agent
// advertised the requested opencode block. With role="executor" and an
// availableModes slice that contains "executor", the mock ACP client must
// observe a single SetSessionConfigOption(ctx, sessionID, "mode", "executor")
// call. No config-mismatch entry should be recorded.
func TestSetSessionConfigOption_OnNewSession_SwitchesMode(t *testing.T) {
	const (
		projectID = "p1"
		sessionID = "sess-1"
	)

	g := newTestGateway(nil)
	client := &mockModeRoutingClient{}

	availableModes := []string{"planner", "executor", "reviewer"}
	if err := g.routeSessionMode(context.Background(), client, sessionID, projectID, "executor", "executor", availableModes); err != nil {
		t.Fatalf("routeSessionMode() returned unexpected error: %v", err)
	}

	calls := client.snapshot()
	if len(calls) != 1 {
		t.Fatalf("SetSessionConfigOption call count = %d, want 1 (full calls: %v)", len(calls), calls)
	}
	c := calls[0]
	if c.sessionID != sessionID {
		t.Errorf("SetSessionConfigOption sessionID = %q, want %q", c.sessionID, sessionID)
	}
	if c.configID != "mode" {
		t.Errorf("SetSessionConfigOption configID = %q, want %q", c.configID, "mode")
	}
	if c.value != "executor" {
		t.Errorf("SetSessionConfigOption value = %q, want %q", c.value, "executor")
	}

	if missing := g.MissingOpencodeBlocks(projectID); len(missing) != 0 {
		t.Errorf("MissingOpencodeBlocks(%q) = %v, want empty (no mismatch should be recorded on the happy path)", projectID, missing)
	}
}

// TestDefaultRoleForTaskType_StaticFallback is a table test asserting each
// canonical models.TaskType value maps to the expected opencode agent block
// name, with empty and unrecognized task types both falling back to
// "planner" (the default_agent in opencode_config/opencode.json).
func TestDefaultRoleForTaskType_StaticFallback(t *testing.T) {
	tests := []struct {
		name     string
		taskType string
		want     string
	}{
		{name: "planning maps to planner", taskType: "planning", want: "planner"},
		{name: "code maps to executor", taskType: "code", want: "executor"},
		{name: "build maps to builder", taskType: "build", want: "builder"},
		{name: "review maps to reviewer", taskType: "review", want: "reviewer"},
		{name: "security maps to security-auditor", taskType: "security", want: "security-auditor"},
		{name: "release maps to release-manager", taskType: "release", want: "release-manager"},
		{name: "workspace_setup maps to workspace-setup", taskType: "workspace_setup", want: "workspace-setup"},
		{name: "empty maps to planner default", taskType: "", want: "planner"},
		{name: "unrecognized maps to planner default", taskType: "not-a-real-task-type", want: "planner"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := defaultRoleForTaskType(tt.taskType); got != tt.want {
				t.Errorf("defaultRoleForTaskType(%q) = %q, want %q", tt.taskType, got, tt.want)
			}
		})
	}
}

// TestRouteAgent_MissingModeDegradesToDefault verifies the graceful-
// degradation path: when the opencode session does NOT advertise the
// requested "executor" block (availableModes is just ["planner"]),
// routeSessionMode must (1) fall back to availableModes[0] ("planner")
// when issuing SetSessionConfigOption, and (2) record the missing block via
// recordConfigMismatch so MissingOpencodeBlocks surfaces it in the UI
// (Decisions 4 / TASK-010).
func TestRouteAgent_MissingModeDegradesToDefault(t *testing.T) {
	const (
		projectID          = "p-mismatch"
		sessionID          = "sess-degrade"
		requestedAgentType = "executor"
	)

	g := newTestGateway(nil)
	client := &mockModeRoutingClient{}

	availableModes := []string{"planner"} // missing "executor"
	if err := g.routeSessionMode(context.Background(), client, sessionID, projectID, requestedAgentType, requestedAgentType, availableModes); err != nil {
		t.Fatalf("routeSessionMode() returned unexpected error: %v", err)
	}

	calls := client.snapshot()
	if len(calls) != 1 {
		t.Fatalf("SetSessionConfigOption call count = %d, want 1 (calls: %v)", len(calls), calls)
	}
	if got := calls[0].value; got != "planner" {
		t.Errorf("degraded SetSessionConfigOption value = %q, want %q (availableModes[0])", got, "planner")
	}
	if got := calls[0].sessionID; got != sessionID {
		t.Errorf("degraded SetSessionConfigOption sessionID = %q, want %q", got, sessionID)
	}
	if got := calls[0].configID; got != "mode" {
		t.Errorf("degraded SetSessionConfigOption configID = %q, want %q", got, "mode")
	}

	// The mismatch must be recorded so the API surface can surface it to
	// the operator. recordConfigMismatch stores it as
	// "missing_block:<requestedAgentType>".
	missing := g.MissingOpencodeBlocks(projectID)
	wantBlock := "missing_block:" + requestedAgentType
	found := false
	for _, b := range missing {
		if b == wantBlock {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("MissingOpencodeBlocks(%q) = %v, want slice containing %q", projectID, missing, wantBlock)
	}
}

// TestRouteAgent_EmptyAgentTypeFallsBackToTaskTypeDefault verifies that when
// a task arrives with a blank AgentType (e.g. an old task or a non-MCP
// emitter like the workspace-setup auto-task), routeSessionMode falls back
// to the static task-type → role table. With TaskType="code" the resolved
// role is "executor", so SetSessionConfigOption must be called with
// value="executor".
func TestRouteAgent_EmptyAgentTypeFallsBackToTaskTypeDefault(t *testing.T) {
	const (
		projectID = "p-blank-role"
		sessionID = "sess-blank"
	)

	g := newTestGateway(nil)
	client := &mockModeRoutingClient{}

	availableModes := []string{"planner", "executor", "reviewer"}
	// role is resolved by the caller as defaultRoleForTaskType("code")
	// in the real loop.go flow; here we mirror that resolution and pass
	// the resolved role straight through, just like loop.go does.
	role := defaultRoleForTaskType(string(models.TaskTypeCode))
	if role != "executor" {
		t.Fatalf("test setup invariant: defaultRoleForTaskType(code) = %q, want %q", role, "executor")
	}
	if err := g.routeSessionMode(context.Background(), client, sessionID, projectID, role, "", availableModes); err != nil {
		t.Fatalf("routeSessionMode() returned unexpected error: %v", err)
	}

	calls := client.snapshot()
	if len(calls) != 1 {
		t.Fatalf("SetSessionConfigOption call count = %d, want 1 (calls: %v)", len(calls), calls)
	}
	if got := calls[0].value; got != "executor" {
		t.Errorf("SetSessionConfigOption value = %q, want %q (defaultRoleForTaskType(code))", got, "executor")
	}

	// Empty requestedAgentType should NOT record a mismatch — the caller
	// had no explicit preference (this is the soft back-compat path, not
	// the degradation path).
	if missing := g.MissingOpencodeBlocks(projectID); len(missing) != 0 {
		t.Errorf("MissingOpencodeBlocks(%q) = %v, want empty (empty AgentType must not record a mismatch)", projectID, missing)
	}
}

// TestRouteAgent_AppliesSetSessionConfigOption is the canonical happy-path
// routing test: a task with task.AgentType="reviewer" that contains an
// "reviewer" mode in availableModes must produce a SetSessionConfigOption
// call whose value is exactly task.AgentType.
func TestRouteAgent_AppliesSetSessionConfigOption(t *testing.T) {
	const (
		projectID = "p-happy"
		sessionID = "sess-happy"
		agentType = "reviewer"
	)

	g := newTestGateway(nil)
	client := &mockModeRoutingClient{}

	availableModes := []string{"planner", "executor", "reviewer", "security-auditor"}
	if err := g.routeSessionMode(context.Background(), client, sessionID, projectID, agentType, agentType, availableModes); err != nil {
		t.Fatalf("routeSessionMode() returned unexpected error: %v", err)
	}

	calls := client.snapshot()
	if len(calls) != 1 {
		t.Fatalf("SetSessionConfigOption call count = %d, want 1 (calls: %v)", len(calls), calls)
	}
	if calls[0].value != agentType {
		t.Errorf("SetSessionConfigOption value = %q, want %q (matches task.AgentType)", calls[0].value, agentType)
	}
	if calls[0].sessionID != sessionID {
		t.Errorf("SetSessionConfigOption sessionID = %q, want %q", calls[0].sessionID, sessionID)
	}
	if calls[0].configID != "mode" {
		t.Errorf("SetSessionConfigOption configID = %q, want %q", calls[0].configID, "mode")
	}
}
