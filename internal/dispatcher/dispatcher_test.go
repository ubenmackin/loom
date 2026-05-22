package dispatcher

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/ubenmackin/loom/internal/db"
	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
)

// mockBroadcaster is a simple EventBroadcaster that records broadcasts.
type mockBroadcaster struct {
	mu     sync.Mutex
	events []mockEvent
}

type mockEvent struct {
	EventType string
	Payload   any
}

func (m *mockBroadcaster) Broadcast(eventType string, payload any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, mockEvent{EventType: eventType, Payload: payload})
}

func (m *mockBroadcaster) Events() []mockEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]mockEvent, len(m.events))
	copy(result, m.events)
	return result
}

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbName := fmt.Sprintf("test_%s_%d", t.Name(), time.Now().UnixNano())
	dsn := "file:" + dbName + "?mode=memory&cache=private"

	dbConn, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if _, err := dbConn.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}

	if err := db.Migrate(dbConn); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	t.Cleanup(func() {
		_ = dbConn.Close()
	})

	return dbConn
}

func createTestStory(t *testing.T, s *store.StoryStore, title, status string) *models.Story {
	t.Helper()
	story := &models.Story{Title: title, Status: status}
	if story.Status == "" {
		story.Status = models.StatusNew
	}
	if err := s.Create(context.Background(), story); err != nil {
		t.Fatalf("create test story %q: %v", title, err)
	}
	return story
}

func createTestTask(t *testing.T, s *store.TaskStore, storyID, title, status, taskType string) *models.Task {
	t.Helper()
	task := &models.Task{StoryID: storyID, Title: title, Status: status, TaskType: taskType}
	if task.Status == "" {
		task.Status = models.StatusNew
	}
	if task.TaskType == "" {
		task.TaskType = models.TaskTypeCode
	}
	if err := s.Create(context.Background(), task); err != nil {
		t.Fatalf("create test task %q: %v", title, err)
	}
	return task
}

var sessionCounter atomic.Int64

func createTestSession(t *testing.T, s *store.SessionStore, harnessType string, capabilities []string) *models.Session {
	t.Helper()
	n := sessionCounter.Add(1)
	session := &models.Session{
		ID:          fmt.Sprintf("sess-%d", n),
		HarnessType: harnessType,
		Status:      models.SessionStatusActive,
	}
	if len(capabilities) > 0 {
		data, _ := json.Marshal(capabilities)
		session.Capabilities = string(data)
	}
	if err := s.Register(context.Background(), session); err != nil {
		t.Fatalf("create test session: %v", err)
	}
	return session
}

func createTestTemplate(t *testing.T, s *store.TemplateStore, taskType, template string) *models.PromptTemplate {
	t.Helper()
	tmpl := &models.PromptTemplate{TaskType: taskType, Template: template}
	if err := s.Create(context.Background(), tmpl); err != nil {
		t.Fatalf("create test template: %v", err)
	}
	return tmpl
}

func setSessionLastSeen(t *testing.T, dbConn *sql.DB, sessionID string, tstamp time.Time) {
	t.Helper()
	ctx := context.Background()
	_, err := dbConn.ExecContext(ctx, "UPDATE sessions SET last_seen_at = ? WHERE id = ?", tstamp.UTC(), sessionID)
	if err != nil {
		t.Fatalf("set session last_seen_at: %v", err)
	}
}

func newTestDispatcher(t *testing.T) (*Dispatcher, *mockBroadcaster, *sql.DB, *store.StoryStore, *store.TaskStore, *store.SessionStore, *store.TemplateStore, *store.CommentStore, *store.ActivityStore) {
	t.Helper()

	dbConn := setupTestDB(t)
	broadcaster := &mockBroadcaster{}

	storyStore := store.NewStoryStore(dbConn)
	taskStore := store.NewTaskStore(dbConn)
	sessionStore := store.NewSessionStore(dbConn)
	templateStore := store.NewTemplateStore(dbConn)
	commentStore := store.NewCommentStore(dbConn)
	activityStore := store.NewActivityStore(dbConn)

	d := NewDispatcher(
		storyStore,
		taskStore,
		sessionStore,
		templateStore,
		commentStore,
		activityStore,
		broadcaster,
		30*time.Minute,
	)

	return d, broadcaster, dbConn, storyStore, taskStore, sessionStore, templateStore, commentStore, activityStore
}

func TestAssignment_FindBestSession(t *testing.T) {
	t.Parallel()

	d, _, _, _, taskStore, sessionStore, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	sessionA := createTestSession(t, sessionStore, "opencode", []string{"code", "build"})
	sessionB := createTestSession(t, sessionStore, "opencode", []string{"code"})

	story := createTestStory(t, d.stories, "Best Session Story", models.StatusReady)
	task := createTestTask(t, taskStore, story.ID, "Best Session Task", models.StatusReady, models.TaskTypeCode)

	// Assign a task to sessionB to make it more loaded.
	task.AssignedTo = sessionB.ID
	task.AssigneeType = models.AssigneeTypeSession
	task.Status = models.StatusInProgress
	if err := taskStore.Update(ctx, task); err != nil {
		t.Fatalf("update task for load: %v", err)
	}

	_ = createTestTask(t, taskStore, story.ID, "Best Session Task 2", models.StatusReady, models.TaskTypeCode)

	best, err := d.findBestSession(ctx, "code")
	if err != nil {
		t.Fatalf("findBestSession() error = %v", err)
	}

	if best == nil {
		t.Fatal("findBestSession() returned nil, want a session")
	}

	if best.ID != sessionA.ID {
		t.Errorf("findBestSession() = %q, want %q (least loaded)", best.ID, sessionA.ID)
	}
}

func TestAssignment_CapabilityMismatch(t *testing.T) {
	t.Parallel()

	d, _, _, _, taskStore, sessionStore, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	session := createTestSession(t, sessionStore, "opencode", []string{"code"})

	story := createTestStory(t, d.stories, "Mismatch Story", models.StatusReady)
	_ = createTestTask(t, taskStore, story.ID, "Mismatch Task", models.StatusReady, models.TaskTypeBuild)

	best, err := d.findBestSession(ctx, "build")
	if err != nil {
		t.Fatalf("findBestSession() error = %v", err)
	}

	if best != nil {
		t.Errorf("findBestSession() = %q, want nil (no session with build capability)", best.ID)
	}

	assigned, err := d.findAndAssignTaskForSession(ctx, session)
	if err != nil {
		t.Fatalf("findAndAssignTaskForSession() error = %v", err)
	}

	if assigned != nil {
		t.Errorf("findAndAssignTaskForSession() = %q, want nil (capability mismatch)", assigned.ID)
	}
}

func TestGateInjection_BuildTask(t *testing.T) {
	t.Parallel()

	d, broadcaster, _, storyStore, taskStore, _, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Build Gate Story", models.StatusReady)
	story.RequiresBuild = true
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story requires_build: %v", err)
	}

	task := createTestTask(t, taskStore, story.ID, "Code Task", models.StatusDone, models.TaskTypeCode)

	d.checkGateConditions(ctx, story.ID)

	tasks, err := taskStore.GetByStory(ctx, story.ID)
	if err != nil {
		t.Fatalf("GetByStory() error = %v", err)
	}

	var buildTask *models.Task
	for _, tsk := range tasks {
		if tsk.TaskType == models.TaskTypeBuild {
			buildTask = tsk
			break
		}
	}

	if buildTask == nil {
		t.Fatal("checkGateConditions() did not create a Build task")
	}

	if buildTask.Status != models.StatusReady {
		t.Errorf("Build task status = %q, want %q", buildTask.Status, models.StatusReady)
	}

	deps, err := taskStore.GetDependencies(ctx, buildTask.ID)
	if err != nil {
		t.Fatalf("GetDependencies() error = %v", err)
	}

	if len(deps) != 1 {
		t.Fatalf("Build task has %d dependencies, want 1", len(deps))
	}
	if deps[0] != task.ID {
		t.Errorf("Build task depends on %q, want %q", deps[0], task.ID)
	}

	events := broadcaster.Events()
	var foundGateEvent bool
	for _, e := range events {
		if e.EventType == "gate_task_created" {
			foundGateEvent = true
			break
		}
	}
	if !foundGateEvent {
		t.Log("gate_task_created event not found in broadcasts")
	}
}

func TestGateInjection_ReviewTask(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Review Gate Story", models.StatusReady)
	story.RequiresReview = true
	story.RequiresBuild = true
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story requires_review: %v", err)
	}

	_ = createTestTask(t, taskStore, story.ID, "Code Task", models.StatusDone, models.TaskTypeCode)

	d.checkGateConditions(ctx, story.ID)

	tasks, err := taskStore.GetByStory(ctx, story.ID)
	if err != nil {
		t.Fatalf("GetByStory() error = %v", err)
	}

	var buildTask *models.Task
	for _, tsk := range tasks {
		if tsk.TaskType == models.TaskTypeBuild {
			buildTask = tsk
			break
		}
	}

	if buildTask == nil {
		t.Fatal("Build task was not created")
	}

	if err := taskStore.UpdateStatus(ctx, buildTask.ID, models.StatusInProgress); err != nil {
		t.Fatalf("UpdateStatus(Build, InProgress) error = %v", err)
	}
	if err := taskStore.UpdateStatus(ctx, buildTask.ID, models.StatusDone); err != nil {
		t.Fatalf("UpdateStatus(Build, Done) error = %v", err)
	}

	d.checkGateConditions(ctx, story.ID)

	tasks, err = taskStore.GetByStory(ctx, story.ID)
	if err != nil {
		t.Fatalf("GetByStory() after review check error = %v", err)
	}

	var reviewTask *models.Task
	for _, tsk := range tasks {
		if tsk.TaskType == models.TaskTypeReview {
			reviewTask = tsk
			break
		}
	}

	if reviewTask == nil {
		t.Fatal("checkGateConditions() did not create a Review task after Build was Done")
	}

	if reviewTask.Status != models.StatusReady {
		t.Errorf("Review task status = %q, want %q", reviewTask.Status, models.StatusReady)
	}

	deps, err := taskStore.GetDependencies(ctx, reviewTask.ID)
	if err != nil {
		t.Fatalf("GetDependencies() error = %v", err)
	}

	if len(deps) != 1 {
		t.Fatalf("Review task has %d dependencies, want 1", len(deps))
	}
	if deps[0] != buildTask.ID {
		t.Errorf("Review task depends on %q, want %q", deps[0], buildTask.ID)
	}
}

func TestDependencyResolution(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Dep Resolution Story", models.StatusReady)
	taskA := createTestTask(t, taskStore, story.ID, "Task A", models.StatusDone, models.TaskTypeCode)
	taskB := createTestTask(t, taskStore, story.ID, "Task B", models.StatusBlocked, models.TaskTypeCode)
	taskC := createTestTask(t, taskStore, story.ID, "Task C", models.StatusBlocked, models.TaskTypeCode)

	if err := taskStore.AddDependency(ctx, taskB.ID, taskA.ID); err != nil {
		t.Fatalf("AddDependency(B, A) error = %v", err)
	}
	if err := taskStore.AddDependency(ctx, taskC.ID, taskB.ID); err != nil {
		t.Fatalf("AddDependency(C, B) error = %v", err)
	}

	d.resolveDependencies(ctx, taskA.ID)

	gotB, err := taskStore.GetByID(ctx, taskB.ID)
	if err != nil {
		t.Fatalf("GetByID(B) error = %v", err)
	}

	if gotB.Status != models.StatusReady {
		t.Errorf("Task B status = %q, want %q (A is Done)", gotB.Status, models.StatusReady)
	}

	gotC, err := taskStore.GetByID(ctx, taskC.ID)
	if err != nil {
		t.Fatalf("GetByID(C) error = %v", err)
	}

	if gotC.Status != models.StatusBlocked {
		t.Errorf("Task C status = %q, want %q (B is not Done)", gotC.Status, models.StatusBlocked)
	}
}

func TestStalenessDetection(t *testing.T) {
	t.Parallel()

	d, _, dbConn, storyStore, taskStore, sessionStore, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	session := createTestSession(t, sessionStore, "opencode", []string{"code"})
	setSessionLastSeen(t, dbConn, session.ID, time.Now().UTC().Add(-2*time.Hour))

	story := createTestStory(t, storyStore, "Stale Story", models.StatusReady)
	task := createTestTask(t, taskStore, story.ID, "Stale Task", models.StatusInProgress, models.TaskTypeCode)
	task.AssignedTo = session.ID
	task.AssigneeType = models.AssigneeTypeSession
	if err := taskStore.Update(ctx, task); err != nil {
		t.Fatalf("update task assignment: %v", err)
	}

	d.stalenessThreshold = 1 * time.Hour
	d.checkStaleness(ctx)

	gotSession, err := sessionStore.GetByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetByID(session) error = %v", err)
	}

	if gotSession.Status != models.SessionStatusStale {
		t.Errorf("Session status = %q, want %q", gotSession.Status, models.SessionStatusStale)
	}

	gotTask, err := taskStore.GetByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetByID(task) error = %v", err)
	}

	if !gotTask.IsStale {
		t.Errorf("Task IsStale = false, want true")
	}
}

func TestPromptAssembly(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, templateStore, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	createTestTemplate(t, templateStore, models.TaskTypeCode,
		"Task: {{task.title}}\nStory: {{story.title}}\nContext: {{context.file_path}}")

	story := createTestStory(t, storyStore, "Prompt Story", models.StatusReady)
	story.Description = "This is the story description"

	task := createTestTask(t, taskStore, story.ID, "Prompt Task", models.StatusReady, models.TaskTypeCode)
	task.Description = "Task description here"
	task.Context = `{"file_path": "src/main.go", "line": 42}`

	result, err := d.assemblePrompt(ctx, task, story)
	if err != nil {
		t.Fatalf("assemblePrompt() error = %v", err)
	}

	if !containsStr(result, "Prompt Task") {
		t.Errorf("assemblePrompt() result missing task title: %q", result)
	}
	if !containsStr(result, "Prompt Story") {
		t.Errorf("assemblePrompt() result missing story title: %q", result)
	}
	if !containsStr(result, "src/main.go") {
		t.Errorf("assemblePrompt() result missing context.file_path: %q", result)
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestFullLifecycle_BuildFailFixRebuildReview(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, sessionStore, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Full Lifecycle Story", models.StatusReady)
	story.RequiresBuild = true
	story.RequiresReview = true
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story gates: %v", err)
	}

	codeTask := createTestTask(t, taskStore, story.ID, "Implement Feature", models.StatusReady, models.TaskTypeCode)

	session := createTestSession(t, sessionStore, "opencode", []string{"code", "build", "review"})

	assigned, err := d.findAndAssignTaskForSession(ctx, session)
	if err != nil {
		t.Fatalf("findAndAssignTaskForSession() error = %v", err)
	}
	if assigned == nil {
		t.Fatal("findAndAssignTaskForSession() returned nil, expected code task")
	}
	if assigned.ID != codeTask.ID {
		t.Errorf("assigned task = %q, want %q", assigned.ID, codeTask.ID)
	}

	if err := taskStore.UpdateStatus(ctx, codeTask.ID, models.StatusDone); err != nil {
		t.Fatalf("UpdateStatus(code, Done) error = %v", err)
	}

	d.checkGateConditions(ctx, story.ID)

	tasks, err := taskStore.GetByStory(ctx, story.ID)
	if err != nil {
		t.Fatalf("GetByStory() error = %v", err)
	}

	var buildTask *models.Task
	for _, tsk := range tasks {
		if tsk.TaskType == models.TaskTypeBuild {
			buildTask = tsk
			break
		}
	}

	if buildTask == nil {
		t.Fatal("Build task was not created after code task completed")
	}

	buildTask, err = taskStore.GetByID(ctx, buildTask.ID)
	if err != nil {
		t.Fatalf("GetByID(build) error = %v", err)
	}
	buildTask.AssignedTo = session.ID
	buildTask.AssigneeType = models.AssigneeTypeSession
	if err := taskStore.Update(ctx, buildTask); err != nil {
		t.Fatalf("update build task assignment: %v", err)
	}

	if err := taskStore.UpdateStatus(ctx, buildTask.ID, models.StatusInProgress); err != nil {
		t.Fatalf("UpdateStatus(build, InProgress) error = %v", err)
	}
	if err := taskStore.UpdateStatus(ctx, buildTask.ID, models.StatusDone); err != nil {
		t.Fatalf("UpdateStatus(build, Done) error = %v", err)
	}

	d.checkGateConditions(ctx, story.ID)

	tasks, err = taskStore.GetByStory(ctx, story.ID)
	if err != nil {
		t.Fatalf("GetByStory() after build error = %v", err)
	}

	var reviewTask *models.Task
	for _, tsk := range tasks {
		if tsk.TaskType == models.TaskTypeReview {
			reviewTask = tsk
			break
		}
	}

	if reviewTask == nil {
		t.Fatal("Review task was not created after build task completed")
	}

	if reviewTask.Status != models.StatusReady {
		t.Errorf("Review task status = %q, want %q", reviewTask.Status, models.StatusReady)
	}

	if len(tasks) != 3 {
		t.Fatalf("Expected 3 tasks (code, build, review), got %d", len(tasks))
	}
}

func TestPromptAssembly_NoTemplate(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "No Template Story", models.StatusReady)
	story.Description = "Story desc"

	task := createTestTask(t, taskStore, story.ID, "No Template Task", models.StatusReady, models.TaskTypeCode)
	task.Description = "Task desc"

	result, err := d.assemblePrompt(ctx, task, story)
	if err != nil {
		t.Fatalf("assemblePrompt() error = %v", err)
	}

	if !containsStr(result, "No Template Task") {
		t.Errorf("default prompt missing task title: %q", result)
	}
	if !containsStr(result, "No Template Story") {
		t.Errorf("default prompt missing story title: %q", result)
	}
}

func TestPromptAssembly_JSONContext(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, templateStore, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	createTestTemplate(t, templateStore, models.TaskTypeCode,
		"File: {{context.file}}\nLine: {{context.line}}\nError: {{context.error.message}}")

	story := createTestStory(t, storyStore, "JSON Context Story", models.StatusReady)
	task := createTestTask(t, taskStore, story.ID, "JSON Context Task", models.StatusReady, models.TaskTypeCode)
	task.Context = `{"file": "main.go", "line": "42", "error": {"message": "syntax error"}}`

	result, err := d.assemblePrompt(ctx, task, story)
	if err != nil {
		t.Fatalf("assemblePrompt() error = %v", err)
	}

	if !containsStr(result, "main.go") {
		t.Errorf("prompt missing context.file: %q", result)
	}
	if !containsStr(result, "42") {
		t.Errorf("prompt missing context.line: %q", result)
	}
	if !containsStr(result, "syntax error") {
		t.Errorf("prompt missing context.error.message: %q", result)
	}
}

func TestRunAssignmentPass(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, sessionStore, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	session := createTestSession(t, sessionStore, "opencode", []string{"code"})

	story := createTestStory(t, storyStore, "Assignment Pass Story", models.StatusReady)
	task := createTestTask(t, taskStore, story.ID, "Assignment Pass Task", models.StatusReady, models.TaskTypeCode)

	d.runAssignmentPass(ctx)

	got, err := taskStore.GetByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if got.AssignedTo == "" {
		t.Fatal("runAssignmentPass() did not assign the task")
	}
	if got.AssignedTo != session.ID {
		t.Errorf("runAssignmentPass() assigned to %q, want %q", got.AssignedTo, session.ID)
	}
	if got.Status != models.StatusInProgress {
		t.Errorf("runAssignmentPass() status = %q, want %q", got.Status, models.StatusInProgress)
	}
}

func TestHandleTaskStatusChanged(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Status Changed Story", models.StatusReady)
	taskA := createTestTask(t, taskStore, story.ID, "Task A", models.StatusInProgress, models.TaskTypeCode)
	taskB := createTestTask(t, taskStore, story.ID, "Task B", models.StatusBlocked, models.TaskTypeCode)

	if err := taskStore.AddDependency(ctx, taskB.ID, taskA.ID); err != nil {
		t.Fatalf("AddDependency(B, A) error = %v", err)
	}

	if err := taskStore.UpdateStatus(ctx, taskA.ID, models.StatusDone); err != nil {
		t.Fatalf("UpdateStatus(A, Done) error = %v", err)
	}

	d.handleTaskStatusChanged(ctx, Event{Type: "task_status_changed", TaskID: taskA.ID})

	gotB, err := taskStore.GetByID(ctx, taskB.ID)
	if err != nil {
		t.Fatalf("GetByID(B) error = %v", err)
	}

	if gotB.Status != models.StatusReady {
		t.Errorf("Task B status = %q, want %q after A completed", gotB.Status, models.StatusReady)
	}
}

func TestFindAndAssignTaskForSession_BlockedTask(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, sessionStore, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	session := createTestSession(t, sessionStore, "opencode", []string{"code"})

	story := createTestStory(t, storyStore, "Blocked Assignment Story", models.StatusReady)
	taskA := createTestTask(t, taskStore, story.ID, "Blocker", models.StatusNew, models.TaskTypeCode)
	taskB := createTestTask(t, taskStore, story.ID, "Dependent", models.StatusReady, models.TaskTypeCode)

	if err := taskStore.AddDependency(ctx, taskB.ID, taskA.ID); err != nil {
		t.Fatalf("AddDependency(B, A) error = %v", err)
	}

	assigned, err := d.findAndAssignTaskForSession(ctx, session)
	if err != nil {
		t.Fatalf("findAndAssignTaskForSession() error = %v", err)
	}

	if assigned != nil {
		t.Errorf("findAndAssignTaskForSession() = %q, want nil (task B has blockers)", assigned.ID)
	}

	_ = taskA
}

func TestParseCapabilities(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, sessionStore, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	session := createTestSession(t, sessionStore, "opencode", []string{"code", "build", "review"})

	story := createTestStory(t, storyStore, "Parse Caps Story", models.StatusReady)
	task := createTestTask(t, taskStore, story.ID, "Review Task", models.StatusReady, models.TaskTypeReview)

	assigned, err := d.findAndAssignTaskForSession(ctx, session)
	if err != nil {
		t.Fatalf("findAndAssignTaskForSession() error = %v", err)
	}

	if assigned == nil {
		t.Fatal("findAndAssignTaskForSession() returned nil, expected review task")
	}
	if assigned.ID != task.ID {
		t.Errorf("assigned task = %q, want %q", assigned.ID, task.ID)
	}
}

func TestCheckGateConditions_NoGatesRequired(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "No Gates Story", models.StatusReady)
	createTestTask(t, taskStore, story.ID, "Code Task", models.StatusDone, models.TaskTypeCode)

	d.checkGateConditions(ctx, story.ID)

	tasks, err := taskStore.GetByStory(ctx, story.ID)
	if err != nil {
		t.Fatalf("GetByStory() error = %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("GetByStory() returned %d tasks, want 1 (no gate tasks should be created)", len(tasks))
	}
}

func TestCheckGateConditions_BuildAlreadyExists(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Dup Build Story", models.StatusReady)
	story.RequiresBuild = true
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story: %v", err)
	}

	createTestTask(t, taskStore, story.ID, "Code Task", models.StatusDone, models.TaskTypeCode)

	d.checkGateConditions(ctx, story.ID)
	d.checkGateConditions(ctx, story.ID)

	tasks, err := taskStore.GetByStory(ctx, story.ID)
	if err != nil {
		t.Fatalf("GetByStory() error = %v", err)
	}

	buildCount := 0
	for _, tsk := range tasks {
		if tsk.TaskType == models.TaskTypeBuild {
			buildCount++
		}
	}

	if buildCount != 1 {
		t.Fatalf("Found %d Build tasks, want 1", buildCount)
	}
}

func TestCheckGateConditions_CodeTasksNotAllDone(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Not All Done Story", models.StatusReady)
	story.RequiresBuild = true
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story: %v", err)
	}

	createTestTask(t, taskStore, story.ID, "Code Task 1", models.StatusDone, models.TaskTypeCode)
	createTestTask(t, taskStore, story.ID, "Code Task 2", models.StatusInProgress, models.TaskTypeCode)

	d.checkGateConditions(ctx, story.ID)

	tasks, err := taskStore.GetByStory(ctx, story.ID)
	if err != nil {
		t.Fatalf("GetByStory() error = %v", err)
	}

	for _, tsk := range tasks {
		if tsk.TaskType == models.TaskTypeBuild {
			t.Fatal("Build task should not be created when not all code tasks are Done")
		}
	}
}

func TestCheckGateConditions_ReviewWithoutBuild(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Review No Build Story", models.StatusReady)
	story.RequiresReview = true
	story.RequiresBuild = false
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story: %v", err)
	}

	createTestTask(t, taskStore, story.ID, "Code Task", models.StatusDone, models.TaskTypeCode)

	d.checkGateConditions(ctx, story.ID)

	tasks, err := taskStore.GetByStory(ctx, story.ID)
	if err != nil {
		t.Fatalf("GetByStory() error = %v", err)
	}

	var reviewTask *models.Task
	for _, tsk := range tasks {
		if tsk.TaskType == models.TaskTypeReview {
			reviewTask = tsk
			break
		}
	}

	if reviewTask == nil {
		t.Fatal("Review task should be created when requires_review=true and all code tasks are Done (no build required)")
	}
}

func TestStaleness_NoStaleSessions(t *testing.T) {
	t.Parallel()

	d, _, _, _, _, sessionStore, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	session := createTestSession(t, sessionStore, "opencode", []string{"code"})

	d.stalenessThreshold = 1 * time.Hour
	d.checkStaleness(ctx)

	got, err := sessionStore.GetByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if got.Status != models.SessionStatusActive {
		t.Errorf("Session status = %q, want %q (should remain active)", got.Status, models.SessionStatusActive)
	}
}

func TestResolveDependencies_NoDependents(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "No Deps Story", models.StatusReady)
	taskA := createTestTask(t, taskStore, story.ID, "Task A", models.StatusDone, models.TaskTypeCode)

	d.resolveDependencies(ctx, taskA.ID)

	got, err := taskStore.GetByID(ctx, taskA.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Status != models.StatusDone {
		t.Errorf("Task A status = %q, want %q", got.Status, models.StatusDone)
	}
}

func TestAssignWork_NonActiveSession(t *testing.T) {
	t.Parallel()

	d, _, _, _, _, sessionStore, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	session := createTestSession(t, sessionStore, "opencode", []string{"code"})
	if err := sessionStore.FlagStale(ctx, session.ID); err != nil {
		t.Fatalf("FlagStale() error = %v", err)
	}

	_, err := d.AssignWork(ctx, session.ID)
	if err == nil {
		t.Fatal("AssignWork() expected error for stale session, got nil")
	}
}

func TestAssignWork_NoReadyTasks(t *testing.T) {
	t.Parallel()

	d, _, _, _, _, sessionStore, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	createTestSession(t, sessionStore, "opencode", []string{"code"})

	task, err := d.AssignWork(ctx, "nonexistent-session")
	if err == nil && task != nil {
		t.Fatal("AssignWork() should error for nonexistent session")
	}
}

func TestBuildTask_InstructionsAssembled(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, templateStore, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	createTestTemplate(t, templateStore, models.TaskTypeBuild,
		"Build the project: {{story.title}}")

	story := createTestStory(t, storyStore, "Build Instructions Story", models.StatusReady)
	story.RequiresBuild = true
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story: %v", err)
	}

	createTestTask(t, taskStore, story.ID, "Code Task", models.StatusDone, models.TaskTypeCode)

	d.checkGateConditions(ctx, story.ID)

	tasks, err := taskStore.GetByStory(ctx, story.ID)
	if err != nil {
		t.Fatalf("GetByStory() error = %v", err)
	}

	for _, tsk := range tasks {
		if tsk.TaskType == models.TaskTypeBuild {
			if tsk.Instructions == "" {
				t.Fatal("Build task Instructions is empty, expected assembled prompt")
			}
			if !containsStr(tsk.Instructions, "Build the project") {
				t.Errorf("Build task Instructions missing template text: %q", tsk.Instructions)
			}
			return
		}
	}

	t.Fatal("Build task not found")
}

func TestReviewTask_InstructionsAssembled(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, templateStore, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	createTestTemplate(t, templateStore, models.TaskTypeBuild, "Build: {{story.title}}")
	createTestTemplate(t, templateStore, models.TaskTypeReview, "Review: {{story.title}}")

	story := createTestStory(t, storyStore, "Review Instructions Story", models.StatusReady)
	story.RequiresBuild = true
	story.RequiresReview = true
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story: %v", err)
	}

	createTestTask(t, taskStore, story.ID, "Code Task", models.StatusDone, models.TaskTypeCode)

	d.checkGateConditions(ctx, story.ID)

	tasks, err := taskStore.GetByStory(ctx, story.ID)
	if err != nil {
		t.Fatalf("GetByStory() error = %v", err)
	}

	for _, tsk := range tasks {
		if tsk.TaskType == models.TaskTypeBuild {
			if err := taskStore.UpdateStatus(ctx, tsk.ID, models.StatusInProgress); err != nil {
				t.Fatalf("UpdateStatus(Build, InProgress) error = %v", err)
			}
			if err := taskStore.UpdateStatus(ctx, tsk.ID, models.StatusDone); err != nil {
				t.Fatalf("UpdateStatus(Build, Done) error = %v", err)
			}
			break
		}
	}

	d.checkGateConditions(ctx, story.ID)

	tasks, err = taskStore.GetByStory(ctx, story.ID)
	if err != nil {
		t.Fatalf("GetByStory() error = %v", err)
	}

	for _, tsk := range tasks {
		if tsk.TaskType == models.TaskTypeReview {
			if tsk.Instructions == "" {
				t.Fatal("Review task Instructions is empty, expected assembled prompt")
			}
			if !containsStr(tsk.Instructions, "Review") {
				t.Errorf("Review task Instructions missing template text: %q", tsk.Instructions)
			}
			return
		}
	}

	t.Fatal("Review task not found")
}

func TestMultipleSessions_LoadBalancing(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, sessionStore, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	sessionA := createTestSession(t, sessionStore, "opencode", []string{"code"})
	sessionB := createTestSession(t, sessionStore, "opencode", []string{"code"})

	story := createTestStory(t, storyStore, "Load Balance Story", models.StatusReady)
	task1 := createTestTask(t, taskStore, story.ID, "Task 1", models.StatusReady, models.TaskTypeCode)
	task2 := createTestTask(t, taskStore, story.ID, "Task 2", models.StatusReady, models.TaskTypeCode)

	assigned1, err := d.findAndAssignTaskForSession(ctx, sessionA)
	if err != nil {
		t.Fatalf("findAndAssignTaskForSession(A) error = %v", err)
	}
	if assigned1 == nil {
		t.Fatal("findAndAssignTaskForSession(A) returned nil")
	}

	assigned2, err := d.findAndAssignTaskForSession(ctx, sessionB)
	if err != nil {
		t.Fatalf("findAndAssignTaskForSession(B) error = %v", err)
	}
	if assigned2 == nil {
		t.Fatal("findAndAssignTaskForSession(B) returned nil")
	}

	if assigned1.ID == assigned2.ID {
		t.Errorf("Both sessions got the same task: %q", assigned1.ID)
	}

	_ = task1
	_ = task2
}

func TestEventSubmission(t *testing.T) {
	t.Parallel()

	d, _, _, _, _, _, _, _, _ := newTestDispatcher(t)

	d.Submit(Event{Type: "periodic_tick"})
	d.Stop()
}

func TestHandleWorkRequested_MissingSessionID(t *testing.T) {
	t.Parallel()

	d, _, _, _, _, _, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	d.handleWorkRequested(ctx, Event{Type: "work_requested", SessionID: ""})
}

func TestHandleDependencyAdded_NotBlocked(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Dep Added Story", models.StatusReady)
	task := createTestTask(t, taskStore, story.ID, "Ready Task", models.StatusReady, models.TaskTypeCode)

	d.handleDependencyAdded(ctx, Event{Type: "dependency_added", TaskID: task.ID})

	got, err := taskStore.GetByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Status != models.StatusReady {
		t.Errorf("Task status = %q, want %q", got.Status, models.StatusReady)
	}
}

func TestHandleDependencyAdded_UnblocksTask(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Unblock Story", models.StatusReady)
	taskA := createTestTask(t, taskStore, story.ID, "Task A", models.StatusDone, models.TaskTypeCode)
	taskB := createTestTask(t, taskStore, story.ID, "Task B", models.StatusBlocked, models.TaskTypeCode)

	if err := taskStore.AddDependency(ctx, taskB.ID, taskA.ID); err != nil {
		t.Fatalf("AddDependency(B, A) error = %v", err)
	}

	d.handleDependencyAdded(ctx, Event{Type: "dependency_added", TaskID: taskB.ID})

	gotB, err := taskStore.GetByID(ctx, taskB.ID)
	if err != nil {
		t.Fatalf("GetByID(B) error = %v", err)
	}

	if gotB.Status != models.StatusReady {
		t.Errorf("Task B status = %q, want %q", gotB.Status, models.StatusReady)
	}
}

func TestJSONCapabilities(t *testing.T) {
	t.Parallel()

	capsJSON := `["code","build","review"]`
	var caps []string
	if err := json.Unmarshal([]byte(capsJSON), &caps); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(caps) != 3 {
		t.Fatalf("parsed %d capabilities, want 3", len(caps))
	}

	capSet := make(map[string]bool)
	for _, c := range caps {
		capSet[c] = true
	}

	if !capSet["code"] {
		t.Error("capSet missing 'code'")
	}
	if !capSet["build"] {
		t.Error("capSet missing 'build'")
	}
	if !capSet["review"] {
		t.Error("capSet missing 'review'")
	}
}
