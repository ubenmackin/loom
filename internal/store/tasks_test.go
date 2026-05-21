package store

import (
	"context"
	"strings"
	"testing"

	"github.com/ubenmackin/loom/internal/models"
)

func TestTaskCreate(t *testing.T) {
	t.Parallel()

	dbConn := setupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Task Story", models.StatusNew)

	task := &models.Task{
		StoryID:  story.ID,
		Title:    "Test Task",
		Status:   models.StatusReady,
		TaskType: models.TaskTypeCode,
	}

	if err := taskStore.Create(ctx, task); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if task.ID == "" {
		t.Fatal("Create() did not generate an ID")
	}

	if !strings.HasPrefix(task.ID, "TASK-") {
		t.Fatalf("Create() ID %q does not match TASK-NNN format", task.ID)
	}

	if task.StoryID != story.ID {
		t.Errorf("Create() StoryID = %q, want %q", task.StoryID, story.ID)
	}
}

func TestTaskGetByID(t *testing.T) {
	t.Parallel()

	dbConn := setupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Get Task Story", models.StatusNew)
	task := createTestTask(t, taskStore, story.ID, "Get Task", models.StatusReady, models.TaskTypeCode)

	got, err := taskStore.GetByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if got.ID != task.ID {
		t.Errorf("GetByID() ID = %q, want %q", got.ID, task.ID)
	}
	if got.Title != task.Title {
		t.Errorf("GetByID() Title = %q, want %q", got.Title, task.Title)
	}
	if got.StoryID != story.ID {
		t.Errorf("GetByID() StoryID = %q, want %q", got.StoryID, story.ID)
	}
}

func TestTaskList(t *testing.T) {
	t.Parallel()

	dbConn := setupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	storyA := createTestStory(t, storyStore, "Story A", models.StatusNew)
	storyB := createTestStory(t, storyStore, "Story B", models.StatusNew)

	createTestTask(t, taskStore, storyA.ID, "Task A1", models.StatusReady, models.TaskTypeCode)
	createTestTask(t, taskStore, storyA.ID, "Task A2", models.StatusNew, models.TaskTypeBuild)
	createTestTask(t, taskStore, storyB.ID, "Task B1", models.StatusReady, models.TaskTypeCode)

	t.Run("no filter", func(t *testing.T) {
		all, err := taskStore.List(ctx, TaskFilter{})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(all) != 3 {
			t.Fatalf("List() returned %d tasks, want 3", len(all))
		}
	})

	t.Run("filter by story_id", func(t *testing.T) {
		tasks, err := taskStore.List(ctx, TaskFilter{StoryID: storyA.ID})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(tasks) != 2 {
			t.Fatalf("List() returned %d tasks for story A, want 2", len(tasks))
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		tasks, err := taskStore.List(ctx, TaskFilter{Status: models.StatusReady})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(tasks) != 2 {
			t.Fatalf("List() returned %d ready tasks, want 2", len(tasks))
		}
	})

	t.Run("filter by task_type", func(t *testing.T) {
		tasks, err := taskStore.List(ctx, TaskFilter{TaskType: models.TaskTypeBuild})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(tasks) != 1 {
			t.Fatalf("List() returned %d build tasks, want 1", len(tasks))
		}
	})
}

func TestTaskUpdate(t *testing.T) {
	t.Parallel()

	dbConn := setupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Update Task Story", models.StatusNew)
	task := createTestTask(t, taskStore, story.ID, "Update Task", models.StatusNew, models.TaskTypeCode)

	task.Title = "Updated Task Title"
	task.Priority = 10
	task.TaskType = models.TaskTypeBuild

	if err := taskStore.Update(ctx, task); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, err := taskStore.GetByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetByID() after update error = %v", err)
	}

	if got.Title != "Updated Task Title" {
		t.Errorf("Update() Title = %q, want %q", got.Title, "Updated Task Title")
	}
	if got.Priority != 10 {
		t.Errorf("Update() Priority = %d, want 10", got.Priority)
	}
	if got.TaskType != models.TaskTypeBuild {
		t.Errorf("Update() TaskType = %q, want %q", got.TaskType, models.TaskTypeBuild)
	}
}

func TestTaskUpdateStatus(t *testing.T) {
	t.Parallel()

	dbConn := setupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	t.Run("valid transition", func(t *testing.T) {
		story := createTestStory(t, storyStore, "Valid Status Story", models.StatusNew)
		task := createTestTask(t, taskStore, story.ID, "Valid Status Task", models.StatusNew, models.TaskTypeCode)

		if err := taskStore.UpdateStatus(ctx, task.ID, models.StatusReady); err != nil {
			t.Fatalf("UpdateStatus() error = %v", err)
		}

		got, err := taskStore.GetByID(ctx, task.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if got.Status != models.StatusReady {
			t.Errorf("UpdateStatus() status = %q, want %q", got.Status, models.StatusReady)
		}
	})

	t.Run("invalid transition", func(t *testing.T) {
		story := createTestStory(t, storyStore, "Invalid Status Story", models.StatusNew)
		task := createTestTask(t, taskStore, story.ID, "Invalid Status Task", models.StatusNew, models.TaskTypeCode)

		err := taskStore.UpdateStatus(ctx, task.ID, models.StatusDone)
		if err == nil {
			t.Fatal("UpdateStatus() expected error for new->done, got nil")
		}
		if !strings.Contains(err.Error(), "invalid transition") {
			t.Errorf("UpdateStatus() error = %v, want 'invalid transition'", err)
		}
	})
}

func TestAddDependency(t *testing.T) {
	t.Parallel()

	dbConn := setupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Dependency Story", models.StatusNew)
	taskA := createTestTask(t, taskStore, story.ID, "Task A", models.StatusDone, models.TaskTypeCode)
	taskB := createTestTask(t, taskStore, story.ID, "Task B", models.StatusReady, models.TaskTypeCode)

	if err := taskStore.AddDependency(ctx, taskB.ID, taskA.ID); err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}

	deps, err := taskStore.GetDependencies(ctx, taskB.ID)
	if err != nil {
		t.Fatalf("GetDependencies() error = %v", err)
	}

	if len(deps) != 1 {
		t.Fatalf("GetDependencies() returned %d deps, want 1", len(deps))
	}
	if deps[0] != taskA.ID {
		t.Errorf("GetDependencies() dep = %q, want %q", deps[0], taskA.ID)
	}
}

func TestAddDependency_Cycle(t *testing.T) {
	t.Parallel()

	dbConn := setupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Cycle Story", models.StatusNew)
	taskA := createTestTask(t, taskStore, story.ID, "Cycle A", models.StatusReady, models.TaskTypeCode)
	taskB := createTestTask(t, taskStore, story.ID, "Cycle B", models.StatusReady, models.TaskTypeCode)

	// A depends on B (B -> A in dependency graph: A depends on B)
	if err := taskStore.AddDependency(ctx, taskA.ID, taskB.ID); err != nil {
		t.Fatalf("AddDependency(A, B) error = %v", err)
	}

	// Try to add B depends on A — this would create a cycle
	err := taskStore.AddDependency(ctx, taskB.ID, taskA.ID)
	if err == nil {
		t.Fatal("AddDependency(B, A) expected cycle error, got nil")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Errorf("AddDependency(B, A) error = %v, want 'cycle'", err)
	}
}

func TestAddDependency_SelfCycle(t *testing.T) {
	t.Parallel()

	dbConn := setupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Self Cycle Story", models.StatusNew)
	taskA := createTestTask(t, taskStore, story.ID, "Self Cycle A", models.StatusReady, models.TaskTypeCode)

	err := taskStore.AddDependency(ctx, taskA.ID, taskA.ID)
	if err == nil {
		t.Fatal("AddDependency(A, A) expected self-dependency error, got nil")
	}
	if !strings.Contains(err.Error(), "depend on itself") {
		t.Errorf("AddDependency(A, A) error = %v, want 'depend on itself'", err)
	}
}

func TestRemoveDependency(t *testing.T) {
	t.Parallel()

	dbConn := setupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Remove Dep Story", models.StatusNew)
	taskA := createTestTask(t, taskStore, story.ID, "Remove A", models.StatusDone, models.TaskTypeCode)
	taskB := createTestTask(t, taskStore, story.ID, "Remove B", models.StatusReady, models.TaskTypeCode)

	if err := taskStore.AddDependency(ctx, taskB.ID, taskA.ID); err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}

	if err := taskStore.RemoveDependency(ctx, taskB.ID, taskA.ID); err != nil {
		t.Fatalf("RemoveDependency() error = %v", err)
	}

	deps, err := taskStore.GetDependencies(ctx, taskB.ID)
	if err != nil {
		t.Fatalf("GetDependencies() error = %v", err)
	}

	if len(deps) != 0 {
		t.Fatalf("GetDependencies() returned %d deps after removal, want 0", len(deps))
	}
}

func TestGetBlockers(t *testing.T) {
	t.Parallel()

	dbConn := setupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Blockers Story", models.StatusNew)
	taskA := createTestTask(t, taskStore, story.ID, "Blocker A", models.StatusDone, models.TaskTypeCode)
	taskB := createTestTask(t, taskStore, story.ID, "Blocker B", models.StatusInProgress, models.TaskTypeCode)
	taskC := createTestTask(t, taskStore, story.ID, "Dependent C", models.StatusBlocked, models.TaskTypeCode)

	// C depends on A and B
	if err := taskStore.AddDependency(ctx, taskC.ID, taskA.ID); err != nil {
		t.Fatalf("AddDependency(C, A) error = %v", err)
	}
	if err := taskStore.AddDependency(ctx, taskC.ID, taskB.ID); err != nil {
		t.Fatalf("AddDependency(C, B) error = %v", err)
	}

	blockers, err := taskStore.GetBlockers(ctx, taskC.ID)
	if err != nil {
		t.Fatalf("GetBlockers() error = %v", err)
	}

	// Only B should be a blocker (A is Done)
	if len(blockers) != 1 {
		t.Fatalf("GetBlockers() returned %d blockers, want 1", len(blockers))
	}
	if blockers[0].ID != taskB.ID {
		t.Errorf("GetBlockers() blocker = %q, want %q", blockers[0].ID, taskB.ID)
	}
}

func TestDetectCycle(t *testing.T) {
	t.Parallel()

	dbConn := setupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Detect Cycle Story", models.StatusNew)
	taskA := createTestTask(t, taskStore, story.ID, "Cycle Detect A", models.StatusReady, models.TaskTypeCode)
	taskB := createTestTask(t, taskStore, story.ID, "Cycle Detect B", models.StatusReady, models.TaskTypeCode)
	taskC := createTestTask(t, taskStore, story.ID, "Cycle Detect C", models.StatusReady, models.TaskTypeCode)

	// Create chain: B depends on A, C depends on B
	if err := taskStore.AddDependency(ctx, taskB.ID, taskA.ID); err != nil {
		t.Fatalf("AddDependency(B, A) error = %v", err)
	}
	if err := taskStore.AddDependency(ctx, taskC.ID, taskB.ID); err != nil {
		t.Fatalf("AddDependency(C, B) error = %v", err)
	}

	t.Run("A->C creates cycle", func(t *testing.T) {
		// "A depends on C" — DFS from A looking for C.
		// From A: tasks where depends_on_task_id = A → B.
		// From B: tasks where depends_on_task_id = B → C. Found! Cycle.
		hasCycle, err := taskStore.DetectCycle(ctx, taskA.ID, taskC.ID)
		if err != nil {
			t.Fatalf("DetectCycle() error = %v", err)
		}
		if !hasCycle {
			t.Fatal("DetectCycle(A, C) expected cycle, got false")
		}
	})

	t.Run("C->A does not create cycle", func(t *testing.T) {
		// "C depends on A" — DFS from C looking for A.
		// From C: tasks where depends_on_task_id = C → nothing. No cycle.
		hasCycle, err := taskStore.DetectCycle(ctx, taskC.ID, taskA.ID)
		if err != nil {
			t.Fatalf("DetectCycle() error = %v", err)
		}
		if hasCycle {
			t.Fatal("DetectCycle(C, A) expected no cycle, got true")
		}
	})
}

func TestGetByStory(t *testing.T) {
	t.Parallel()

	dbConn := setupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "GetByStory Story", models.StatusNew)
	createTestTask(t, taskStore, story.ID, "Task 1", models.StatusReady, models.TaskTypeCode)
	createTestTask(t, taskStore, story.ID, "Task 2", models.StatusNew, models.TaskTypeBuild)
	createTestTask(t, taskStore, story.ID, "Task 3", models.StatusDone, models.TaskTypeReview)

	tasks, err := taskStore.GetByStory(ctx, story.ID)
	if err != nil {
		t.Fatalf("GetByStory() error = %v", err)
	}

	if len(tasks) != 3 {
		t.Fatalf("GetByStory() returned %d tasks, want 3", len(tasks))
	}
}
