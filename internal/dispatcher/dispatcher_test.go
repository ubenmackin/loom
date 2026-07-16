package dispatcher

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
	"github.com/ubenmackin/loom/internal/testhelpers"
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

func newTestDispatcher(t *testing.T) (*Dispatcher, *mockBroadcaster, *sql.DB, *store.StoryStore, *store.TaskStore, *store.SessionStore, *store.TemplateStore, *store.CommentStore, *store.ActivityStore) {
	t.Helper()

	dbConn := testhelpers.SetupTestDB(t)
	broadcaster := &mockBroadcaster{}

	storyStore := store.NewStoryStore(dbConn)
	taskStore := store.NewTaskStore(dbConn)
	sessionStore := store.NewSessionStore(dbConn)
	templateStore := store.NewTemplateStore(dbConn)
	commentStore := store.NewCommentStore(dbConn)
	activityStore := store.NewActivityStore(dbConn)

	d := NewDispatcher(DispatcherDeps{
		StoryStore:         storyStore,
		TaskStore:          taskStore,
		SessionStore:       sessionStore,
		TemplateStore:      templateStore,
		CommentStore:       commentStore,
		ActivityStore:      activityStore,
		Broadcaster:        broadcaster,
		StalenessThreshold: 30 * time.Minute,
	})

	return d, broadcaster, dbConn, storyStore, taskStore, sessionStore, templateStore, commentStore, activityStore
}

func TestAssignment_FindBestSession(t *testing.T) {
	t.Parallel()

	d, _, _, _, taskStore, sessionStore, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	sessionA := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code", "build"})
		s.Capabilities = string(data)
	})
	sessionB := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})

	story := testhelpers.CreateTestStory(t, d.stories, func(s *models.Story) { s.Title = "Best Session Story"; s.Status = models.StatusReady })
	task := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Best Session Task"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

	// Assign a task to sessionB to make it more loaded.
	task.AssignedTo = sessionB.ID
	task.AssigneeType = models.AssigneeTypeSession
	task.Status = models.StatusInProgress
	if err := taskStore.Update(ctx, task); err != nil {
		t.Fatalf("update task for load: %v", err)
	}

	_ = testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Best Session Task 2"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

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

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})

	story := testhelpers.CreateTestStory(t, d.stories, func(s *models.Story) { s.Title = "Mismatch Story"; s.Status = models.StatusReady })
	_ = testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Mismatch Task"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeBuild
	})

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

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Build Gate Story"; s.Status = models.StatusReady })
	story.RequiresBuild = true
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story requires_build: %v", err)
	}

	task := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Code Task"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeCode
	})

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

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Review Gate Story"; s.Status = models.StatusReady })
	story.RequiresReview = true
	story.RequiresBuild = true
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story requires_review: %v", err)
	}

	_ = testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Code Task"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeCode
	})

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

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Dep Resolution Story"; s.Status = models.StatusReady })
	taskA := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Task A"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeCode
	})
	taskB := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Task B"
		ts.Status = models.StatusBlocked
		ts.TaskType = models.TaskTypeCode
	})
	taskC := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Task C"
		ts.Status = models.StatusBlocked
		ts.TaskType = models.TaskTypeCode
	})

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

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})
	testhelpers.SetSessionLastSeen(t, dbConn, session.ID, time.Now().UTC().Add(-2*time.Hour))

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Stale Story"; s.Status = models.StatusReady })
	task := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Stale Task"
		ts.Status = models.StatusInProgress
		ts.TaskType = models.TaskTypeCode
	})
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

	testhelpers.CreateTestTemplate(t, templateStore, func(tmpl *models.PromptTemplate) {
		tmpl.TaskType = models.TaskTypeCode
		tmpl.Template = "Task: {{task.title}}\nStory: {{story.title}}\nDesc: {{task.description}}"
	})

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Prompt Story"; s.Status = models.StatusReady })
	story.Description = "This is the story description"

	task := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Prompt Task"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})
	task.Description = "Task description here"

	result, err := d.assemblePrompt(ctx, task, story, "")
	if err != nil {
		t.Fatalf("assemblePrompt() error = %v", err)
	}

	if !strings.Contains(result, "Prompt Task") {
		t.Errorf("assemblePrompt() result missing task title: %q", result)
	}
	if !strings.Contains(result, "Prompt Story") {
		t.Errorf("assemblePrompt() result missing story title: %q", result)
	}
	if !strings.Contains(result, "Task description here") {
		t.Errorf("assemblePrompt() result missing description: %q", result)
	}
}

func TestFullLifecycle_BuildFailFixRebuildReview(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, sessionStore, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Full Lifecycle Story"; s.Status = models.StatusReady })
	story.RequiresBuild = true
	story.RequiresReview = true
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story gates: %v", err)
	}

	codeTask := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Implement Feature"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code", "build", "review"})
		s.Capabilities = string(data)
	})

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

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "No Template Story"; s.Status = models.StatusReady })
	story.Description = "Story desc"

	task := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "No Template Task"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})
	task.Description = "Task desc"

	result, err := d.assemblePrompt(ctx, task, story, "")
	if err != nil {
		t.Fatalf("assemblePrompt() error = %v", err)
	}

	if !strings.Contains(result, "No Template Task") {
		t.Errorf("default prompt missing task title: %q", result)
	}
	if !strings.Contains(result, "No Template Story") {
		t.Errorf("default prompt missing story title: %q", result)
	}
}

func TestPromptAssembly_JSONContext(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, templateStore, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	testhelpers.CreateTestTemplate(t, templateStore, func(tmpl *models.PromptTemplate) {
		tmpl.TaskType = models.TaskTypeCode
		tmpl.Template = "Task: {{task.title}}"
	})

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "JSON Context Story"; s.Status = models.StatusReady })
	task := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "JSON Context Task"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

	result, err := d.assemblePrompt(ctx, task, story, "")
	if err != nil {
		t.Fatalf("assemblePrompt() error = %v", err)
	}

	if !strings.Contains(result, "JSON Context Task") {
		t.Errorf("prompt missing task title: %q", result)
	}
}

func TestRunAssignmentPass(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, sessionStore, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Assignment Pass Story"; s.Status = models.StatusReady })
	task := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Assignment Pass Task"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

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

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Status Changed Story"; s.Status = models.StatusReady })
	taskA := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Task A"
		ts.Status = models.StatusInProgress
		ts.TaskType = models.TaskTypeCode
	})
	taskB := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Task B"
		ts.Status = models.StatusBlocked
		ts.TaskType = models.TaskTypeCode
	})

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

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Blocked Assignment Story"; s.Status = models.StatusReady })
	taskA := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Blocker"
		ts.Status = models.StatusNew
		ts.TaskType = models.TaskTypeCode
	})
	taskB := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Dependent"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

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

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code", "build", "review"})
		s.Capabilities = string(data)
	})

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Parse Caps Story"; s.Status = models.StatusReady })
	task := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Review Task"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeReview
	})

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

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "No Gates Story"; s.Status = models.StatusReady })
	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Code Task"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeCode
	})

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

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Dup Build Story"; s.Status = models.StatusReady })
	story.RequiresBuild = true
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story: %v", err)
	}

	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Code Task"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeCode
	})

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

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Not All Done Story"; s.Status = models.StatusReady })
	story.RequiresBuild = true
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story: %v", err)
	}

	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Code Task 1"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeCode
	})
	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Code Task 2"
		ts.Status = models.StatusInProgress
		ts.TaskType = models.TaskTypeCode
	})

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

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Review No Build Story"; s.Status = models.StatusReady })
	story.RequiresReview = true
	story.RequiresBuild = false
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story: %v", err)
	}

	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Code Task"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeCode
	})

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

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})

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

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "No Deps Story"; s.Status = models.StatusReady })
	taskA := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Task A"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeCode
	})

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

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})
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

	testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})

	task, err := d.AssignWork(ctx, "nonexistent-session")
	if err == nil && task != nil {
		t.Fatal("AssignWork() should error for nonexistent session")
	}
}

func TestBuildTask_InstructionsAssembled(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, templateStore, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	testhelpers.CreateTestTemplate(t, templateStore, func(tmpl *models.PromptTemplate) {
		tmpl.TaskType = models.TaskTypeBuild
		tmpl.Template = "Build the project: {{story.title}}"
	})

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Build Instructions Story"; s.Status = models.StatusReady })
	story.RequiresBuild = true
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story: %v", err)
	}

	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Code Task"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeCode
	})

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
			if !strings.Contains(tsk.Instructions, "Build the project") {
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

	testhelpers.CreateTestTemplate(t, templateStore, func(tmpl *models.PromptTemplate) {
		tmpl.TaskType = models.TaskTypeBuild
		tmpl.Template = "Build: {{story.title}}"
	})
	testhelpers.CreateTestTemplate(t, templateStore, func(tmpl *models.PromptTemplate) {
		tmpl.TaskType = models.TaskTypeReview
		tmpl.Template = "Review: {{story.title}}"
	})

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Review Instructions Story"; s.Status = models.StatusReady })
	story.RequiresBuild = true
	story.RequiresReview = true
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story: %v", err)
	}

	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Code Task"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeCode
	})

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
			if !strings.Contains(tsk.Instructions, "Review") {
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

	sessionA := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})
	sessionB := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Load Balance Story"; s.Status = models.StatusReady })
	task1 := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Task 1"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})
	task2 := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Task 2"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

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

	d.Submit(context.Background(), Event{Type: EventPeriodicTick})
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

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Dep Added Story"; s.Status = models.StatusReady })
	task := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Ready Task"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

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

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Unblock Story"; s.Status = models.StatusReady })
	taskA := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Task A"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeCode
	})
	taskB := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Task B"
		ts.Status = models.StatusBlocked
		ts.TaskType = models.TaskTypeCode
	})

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

func TestReleaseGateCreation_SkipsCancelledTasks(t *testing.T) {
	t.Parallel()

	t.Run("story with cancelled code task plus done code tasks produces Release gate", func(t *testing.T) {
		t.Parallel()

		d, _, _, storyStore, taskStore, _, _, _, _ := newTestDispatcher(t)
		ctx := context.Background()

		story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) {
			s.Title = "Release With Cancelled Story"
			s.Status = models.StatusReady
		})
		story.RequiresReview = true
		if err := storyStore.Update(ctx, story); err != nil {
			t.Fatalf("update story: %v", err)
		}

		// Two code tasks: one Done, one Cancelled.
		testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
			ts.StoryID = story.ID
			ts.Title = "Code Task Done"
			ts.Status = models.StatusDone
			ts.TaskType = models.TaskTypeCode
		})
		testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
			ts.StoryID = story.ID
			ts.Title = "Code Task Cancelled"
			ts.Status = models.StatusCancelled
			ts.TaskType = models.TaskTypeCode
		})

		// First pass: creates Review task (all code tasks terminal).
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
			t.Fatal("Review task should be created when all code tasks are Done or Cancelled")
		}

		// Complete the Review task (ready → in_progress → done).
		if err := taskStore.UpdateStatus(ctx, reviewTask.ID, models.StatusInProgress); err != nil {
			t.Fatalf("update review task status to in_progress: %v", err)
		}
		if err := taskStore.UpdateStatus(ctx, reviewTask.ID, models.StatusDone); err != nil {
			t.Fatalf("update review task status to done: %v", err)
		}

		// Second pass: should create Release task (cancelled code task treated as terminal).
		d.checkGateConditions(ctx, story.ID)

		tasks, err = taskStore.GetByStory(ctx, story.ID)
		if err != nil {
			t.Fatalf("GetByStory() error = %v", err)
		}

		var releaseTask *models.Task
		for _, tsk := range tasks {
			if tsk.TaskType == models.TaskTypeRelease {
				releaseTask = tsk
				break
			}
		}

		if releaseTask == nil {
			t.Fatal("Release gate should be created when all non-release tasks are Done or Cancelled")
		}
	})

	t.Run("story with pending uncancelled code task does not produce Release gate", func(t *testing.T) {
		t.Parallel()

		d, _, _, storyStore, taskStore, _, _, _, _ := newTestDispatcher(t)
		ctx := context.Background()

		story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) {
			s.Title = "Release Blocked By Pending Story"
			s.Status = models.StatusReady
		})
		story.RequiresReview = true
		if err := storyStore.Update(ctx, story); err != nil {
			t.Fatalf("update story: %v", err)
		}

		// One done code task, one still pending (not cancelled).
		testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
			ts.StoryID = story.ID
			ts.Title = "Code Task Done"
			ts.Status = models.StatusDone
			ts.TaskType = models.TaskTypeCode
		})
		testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
			ts.StoryID = story.ID
			ts.Title = "Code Task InProgress"
			ts.Status = models.StatusInProgress
			ts.TaskType = models.TaskTypeCode
		})

		// First pass: should NOT create Review because not all code tasks are terminal.
		d.checkGateConditions(ctx, story.ID)

		tasks, err := taskStore.GetByStory(ctx, story.ID)
		if err != nil {
			t.Fatalf("GetByStory() error = %v", err)
		}

		for _, tsk := range tasks {
			if tsk.TaskType == models.TaskTypeReview {
				t.Fatal("Review task should not be created when a code task is still pending")
			}
			if tsk.TaskType == models.TaskTypeRelease {
				t.Fatal("Release task should not be created when a code task is still pending")
			}
		}
	})
}

// TestGateReopensAfterRemediation_BuildTask verifies that when a remediation
// code task completes (Done), the highest-priority Done gate task — Build — is
// transitioned back to Ready so it can be re-run.
func TestGateReopensAfterRemediation_BuildTask(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) {
		s.Title = "Reopen Build Story"
		s.Status = models.StatusReady
	})
	story.RequiresBuild = true
	story.FailureCount = 1 // below the circuit breaker threshold (3)
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story: %v", err)
	}

	// The original code task is Done.
	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Original Code Task"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeCode
	})

	// The Build gate task is Done (the failing gate we want to re-open).
	buildTask := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Build Gate"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeBuild
	})

	// A remediation code task is in progress and now completes.
	remediationTask := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Remediation Code"
		ts.Status = models.StatusInProgress
		ts.TaskType = models.TaskTypeCode
	})
	if err := taskStore.UpdateStatus(ctx, remediationTask.ID, models.StatusDone); err != nil {
		t.Fatalf("UpdateStatus(remediation, Done) error = %v", err)
	}

	d.handleTaskStatusChanged(ctx, Event{Type: EventTaskCompleted, TaskID: remediationTask.ID})

	gotBuild, err := taskStore.GetByID(ctx, buildTask.ID)
	if err != nil {
		t.Fatalf("GetByID(build) error = %v", err)
	}
	if gotBuild.Status != models.StatusReady {
		t.Errorf("Build task status = %q, want %q (re-opened after remediation)",
			gotBuild.Status, models.StatusReady)
	}
}

// TestGateReopensAfterRemediation_SecurityTask verifies that when the Build
// gate is not Done but the Security gate is, completing a remediation code
// task re-opens the Security gate.
func TestGateReopensAfterRemediation_SecurityTask(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) {
		s.Title = "Reopen Security Story"
		s.Status = models.StatusReady
	})
	story.RequiresSecurity = true
	story.RequiresBuild = false // no build gate — security is the highest priority Done gate
	story.FailureCount = 1
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story: %v", err)
	}

	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Original Code Task"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeCode
	})

	securityTask := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Security Gate"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeSecurity
	})

	remediationTask := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Remediation Code"
		ts.Status = models.StatusInProgress
		ts.TaskType = models.TaskTypeCode
	})
	if err := taskStore.UpdateStatus(ctx, remediationTask.ID, models.StatusDone); err != nil {
		t.Fatalf("UpdateStatus(remediation, Done) error = %v", err)
	}

	d.handleTaskStatusChanged(ctx, Event{Type: EventTaskCompleted, TaskID: remediationTask.ID})

	gotSecurity, err := taskStore.GetByID(ctx, securityTask.ID)
	if err != nil {
		t.Fatalf("GetByID(security) error = %v", err)
	}
	if gotSecurity.Status != models.StatusReady {
		t.Errorf("Security task status = %q, want %q (re-opened after remediation)",
			gotSecurity.Status, models.StatusReady)
	}
}

// TestGateReopensAfterRemediation_CircuitBreakerStopsLoop verifies that when
// the circuit breaker has tripped (story is "failed" after the 3rd gate
// failure), completing a remediation code task does NOT re-open the gate —
// the loop must stop.
func TestGateReopensAfterRemediation_CircuitBreakerStopsLoop(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) {
		s.Title = "Breaker Story"
		s.Status = models.StatusReady
	})
	story.RequiresBuild = true
	story.FailureCount = 3 // circuit breaker already tripped
	story.Status = models.StatusFailed
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story: %v", err)
	}

	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Original Code Task"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeCode
	})

	buildTask := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Build Gate"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeBuild
	})

	// A remediation code task completes after the breaker tripped.
	remediationTask := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Remediation Code"
		ts.Status = models.StatusInProgress
		ts.TaskType = models.TaskTypeCode
	})
	if err := taskStore.UpdateStatus(ctx, remediationTask.ID, models.StatusDone); err != nil {
		t.Fatalf("UpdateStatus(remediation, Done) error = %v", err)
	}

	d.handleTaskStatusChanged(ctx, Event{Type: EventTaskCompleted, TaskID: remediationTask.ID})

	gotBuild, err := taskStore.GetByID(ctx, buildTask.ID)
	if err != nil {
		t.Fatalf("GetByID(build) error = %v", err)
	}
	if gotBuild.Status != models.StatusDone {
		t.Errorf("Build task status = %q, want %q (circuit breaker should prevent re-open)",
			gotBuild.Status, models.StatusDone)
	}
}

// TestGateReopensOnlyForCodeRemediation verifies that completing a non-code
// task (e.g., a gate task itself) does NOT trigger the gate re-open flow.
func TestGateReopensOnlyForCodeRemediation(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) {
		s.Title = "No Reopen For Non-Code Story"
		s.Status = models.StatusReady
	})
	story.RequiresBuild = true
	story.RequiresReview = true
	story.FailureCount = 1
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story: %v", err)
	}

	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Original Code Task"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeCode
	})

	buildTask := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Build Gate"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeBuild
	})

	// A Review gate task completes. Since it is NOT a code task, the
	// re-open flow must not run, and the Build gate must stay Done.
	reviewTask := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Review Gate"
		ts.Status = models.StatusInProgress
		ts.TaskType = models.TaskTypeReview
	})
	if err := taskStore.UpdateStatus(ctx, reviewTask.ID, models.StatusDone); err != nil {
		t.Fatalf("UpdateStatus(review, Done) error = %v", err)
	}

	d.handleTaskStatusChanged(ctx, Event{Type: EventTaskCompleted, TaskID: reviewTask.ID})

	gotBuild, err := taskStore.GetByID(ctx, buildTask.ID)
	if err != nil {
		t.Fatalf("GetByID(build) error = %v", err)
	}
	if gotBuild.Status != models.StatusDone {
		t.Errorf("Build task status = %q, want %q (non-code completion should not re-open gates)",
			gotBuild.Status, models.StatusDone)
	}
}

// ---------------------------------------------------------------------------
// Circuit breaker tests
// ---------------------------------------------------------------------------

// TestCircuitBreaker_TripsAtThreeFailures verifies that when FailureCount
// reaches 3, the story transitions to StatusFailed and EventStoryFailed is
// broadcast.
func TestCircuitBreaker_TripsAtThreeFailures(t *testing.T) {
	t.Parallel()

	d, broadcaster, _, storyStore, taskStore, _, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) {
		s.Title = "Breaker Trips Story"
		s.Status = models.StatusReady
	})
	story.RequiresBuild = true
	story.FailureCount = 2
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story: %v", err)
	}

	// A remediation code task that is still InProgress — this causes
	// detectGateFailure to return true when a gate task completes.
	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Remediation Code"
		ts.Status = models.StatusInProgress
		ts.TaskType = models.TaskTypeCode
	})

	// Create a Build gate task and complete it to trigger the failure.
	buildTask := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Build Gate"
		ts.Status = models.StatusInProgress
		ts.TaskType = models.TaskTypeBuild
	})
	if err := taskStore.UpdateStatus(ctx, buildTask.ID, models.StatusDone); err != nil {
		t.Fatalf("UpdateStatus(build, Done) error = %v", err)
	}

	d.handleTaskStatusChanged(ctx, Event{Type: EventTaskCompleted, TaskID: buildTask.ID})

	// Story should be failed.
	gotStory, err := storyStore.GetByID(ctx, story.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if gotStory.Status != models.StatusFailed {
		t.Errorf("Story status = %q, want %q", gotStory.Status, models.StatusFailed)
	}
	if gotStory.FailureCount != 3 {
		t.Errorf("Story FailureCount = %d, want 3", gotStory.FailureCount)
	}

	// Verify EventStoryFailed was broadcast.
	events := broadcaster.Events()
	var foundStoryFailed bool
	for _, e := range events {
		if e.EventType == EventStoryFailed {
			foundStoryFailed = true
			break
		}
	}
	if !foundStoryFailed {
		t.Error("EventStoryFailed broadcast not found")
	}
}

// TestCircuitBreaker_DoesNotTripBelowThreshold verifies that when FailureCount
// is below 3, completing a gate task increments the count but does not fail
// the story.
func TestCircuitBreaker_DoesNotTripBelowThreshold(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) {
		s.Title = "Below Threshold Story"
		s.Status = models.StatusReady
	})
	story.RequiresBuild = true
	story.FailureCount = 1
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story: %v", err)
	}

	// A remediation code task that is still InProgress.
	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Remediation Code"
		ts.Status = models.StatusInProgress
		ts.TaskType = models.TaskTypeCode
	})

	// Create a Build gate task and complete it.
	buildTask := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Build Gate"
		ts.Status = models.StatusInProgress
		ts.TaskType = models.TaskTypeBuild
	})
	if err := taskStore.UpdateStatus(ctx, buildTask.ID, models.StatusDone); err != nil {
		t.Fatalf("UpdateStatus(build, Done) error = %v", err)
	}

	d.handleTaskStatusChanged(ctx, Event{Type: EventTaskCompleted, TaskID: buildTask.ID})

	gotStory, err := storyStore.GetByID(ctx, story.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if gotStory.FailureCount != 2 {
		t.Errorf("Story FailureCount = %d, want 2", gotStory.FailureCount)
	}
	if gotStory.Status == models.StatusFailed {
		t.Error("Story should not be failed with FailureCount < 3")
	}
}

// TestCircuitBreaker_ResetAndRetry verifies that a failed story can be reset
// (transitioned to StatusReady with FailureCount reset to 0) and then gate
// creation works normally.
func TestCircuitBreaker_ResetAndRetry(t *testing.T) {
	t.Parallel()

	d, _, _, storyStore, taskStore, _, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	// Start with a failed story (circuit breaker tripped).
	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) {
		s.Title = "Reset And Retry Story"
		s.Status = models.StatusFailed
	})
	story.RequiresBuild = true
	story.FailureCount = 3
	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("update story: %v", err)
	}

	// Reset: transition StatusFailed → StatusReady and clear failure count.
	if err := storyStore.UpdateStatus(ctx, story.ID, models.StatusReady); err != nil {
		t.Fatalf("UpdateStatus(failed -> ready) error = %v", err)
	}
	// Re-fetch the story to get the fresh status before updating other fields.
	freshStory, err := storyStore.GetByID(ctx, story.ID)
	if err != nil {
		t.Fatalf("GetByID() after status update error = %v", err)
	}
	freshStory.FailureCount = 0
	freshStory.RequiresBuild = true
	if err := storyStore.Update(ctx, freshStory); err != nil {
		t.Fatalf("update story failure_count: %v", err)
	}

	// Verify the story is ready again.
	gotStory, err := storyStore.GetByID(ctx, story.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if gotStory.Status != models.StatusReady {
		t.Errorf("After reset: Story status = %q, want %q", gotStory.Status, models.StatusReady)
	}
	if gotStory.FailureCount != 0 {
		t.Errorf("After reset: Story FailureCount = %d, want 0", gotStory.FailureCount)
	}

	// Run a gate cycle: add a Done code task, then check conditions.
	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Code Task"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeCode
	})

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
		t.Fatal("checkGateConditions() did not create a Build task after reset")
	}
	if buildTask.Status != models.StatusReady {
		t.Errorf("Build task status = %q, want %q", buildTask.Status, models.StatusReady)
	}
}

// TestCircuitBreaker_AffectsOnlyItsStory verifies that when the circuit
// breaker trips for one story, other stories are unaffected.
func TestCircuitBreaker_AffectsOnlyItsStory(t *testing.T) {
	t.Parallel()

	d, broadcaster, _, storyStore, taskStore, _, _, _, _ := newTestDispatcher(t)
	ctx := context.Background()

	// Story A: close to tripping (FailureCount=2).
	storyA := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) {
		s.Title = "Story A"
		s.Status = models.StatusReady
	})
	storyA.RequiresBuild = true
	storyA.FailureCount = 2
	if err := storyStore.Update(ctx, storyA); err != nil {
		t.Fatalf("update story A: %v", err)
	}

	// Story B: clean (FailureCount=0).
	storyB := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) {
		s.Title = "Story B"
		s.Status = models.StatusReady
	})
	storyB.RequiresBuild = true
	storyB.FailureCount = 0
	if err := storyStore.Update(ctx, storyB); err != nil {
		t.Fatalf("update story B: %v", err)
	}

	// Add remediation code tasks (InProgress) for both stories.
	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = storyA.ID
		ts.Title = "Remediation A"
		ts.Status = models.StatusInProgress
		ts.TaskType = models.TaskTypeCode
	})
	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = storyB.ID
		ts.Title = "Remediation B"
		ts.Status = models.StatusInProgress
		ts.TaskType = models.TaskTypeCode
	})

	// Create Build gate tasks for both stories.
	buildA := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = storyA.ID
		ts.Title = "Build A"
		ts.Status = models.StatusInProgress
		ts.TaskType = models.TaskTypeBuild
	})
	_ = testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = storyB.ID
		ts.Title = "Build B"
		ts.Status = models.StatusInProgress
		ts.TaskType = models.TaskTypeBuild
	})

	// Complete Story A's Build task — should trip circuit breaker.
	if err := taskStore.UpdateStatus(ctx, buildA.ID, models.StatusDone); err != nil {
		t.Fatalf("UpdateStatus(buildA, Done) error = %v", err)
	}
	d.handleTaskStatusChanged(ctx, Event{Type: EventTaskCompleted, TaskID: buildA.ID})

	// Story A should be failed.
	gotA, err := storyStore.GetByID(ctx, storyA.ID)
	if err != nil {
		t.Fatalf("GetByID(storyA) error = %v", err)
	}
	if gotA.Status != models.StatusFailed {
		t.Errorf("Story A status = %q, want %q", gotA.Status, models.StatusFailed)
	}

	// Story B should be unaffected.
	gotB, err := storyStore.GetByID(ctx, storyB.ID)
	if err != nil {
		t.Fatalf("GetByID(storyB) error = %v", err)
	}
	if gotB.Status == models.StatusFailed {
		t.Error("Story B should not be failed")
	}
	if gotB.FailureCount != 0 {
		t.Errorf("Story B FailureCount = %d, want 0 (unaffected)", gotB.FailureCount)
	}

	// Only one EventStoryFailed should have been broadcast (for story A).
	events := broadcaster.Events()
	storyFailedCount := 0
	for _, e := range events {
		if e.EventType == EventStoryFailed {
			storyFailedCount++
		}
	}
	if storyFailedCount != 1 {
		t.Errorf("EventStoryFailed broadcast count = %d, want 1", storyFailedCount)
	}
}
