package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/testhelpers"
)

func TestTaskCreate(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Task Story"; s.Status = models.StatusNew })

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

	dbConn := testhelpers.SetupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Get Task Story"; s.Status = models.StatusNew })
	task := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Get Task"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

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

	dbConn := testhelpers.SetupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	storyA := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Story A"; s.Status = models.StatusNew })
	storyB := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Story B"; s.Status = models.StatusNew })

	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = storyA.ID
		ts.Title = "Task A1"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})
	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = storyA.ID
		ts.Title = "Task A2"
		ts.Status = models.StatusNew
		ts.TaskType = models.TaskTypeBuild
	})
	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = storyB.ID
		ts.Title = "Task B1"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

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

	dbConn := testhelpers.SetupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Update Task Story"; s.Status = models.StatusNew })
	task := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Update Task"
		ts.Status = models.StatusNew
		ts.TaskType = models.TaskTypeCode
	})

	task.Title = "Updated Task Title"
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
	if got.TaskType != models.TaskTypeBuild {
		t.Errorf("Update() TaskType = %q, want %q", got.TaskType, models.TaskTypeBuild)
	}
}

func TestTaskUpdateStatus(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	t.Run("valid transition", func(t *testing.T) {
		story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Valid Status Story"; s.Status = models.StatusNew })
		task := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
			ts.StoryID = story.ID
			ts.Title = "Valid Status Task"
			ts.Status = models.StatusNew
			ts.TaskType = models.TaskTypeCode
		})

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
		story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Invalid Status Story"; s.Status = models.StatusNew })
		task := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
			ts.StoryID = story.ID
			ts.Title = "Invalid Status Task"
			ts.Status = models.StatusNew
			ts.TaskType = models.TaskTypeCode
		})

		err := taskStore.UpdateStatus(ctx, task.ID, models.StatusDone)
		if err == nil {
			t.Fatal("UpdateStatus() expected error for new->done, got nil")
		}
		if !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("UpdateStatus() error = %v, want ErrInvalidTransition", err)
		}
	})
}

func TestAddDependency(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Dependency Story"; s.Status = models.StatusNew })
	taskA := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Task A"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeCode
	})
	taskB := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Task B"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

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

	dbConn := testhelpers.SetupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Cycle Story"; s.Status = models.StatusNew })
	taskA := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Cycle A"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})
	taskB := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Cycle B"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

	// A depends on B (B -> A in dependency graph: A depends on B)
	if err := taskStore.AddDependency(ctx, taskA.ID, taskB.ID); err != nil {
		t.Fatalf("AddDependency(A, B) error = %v", err)
	}

	// Try to add B depends on A — this would create a cycle
	err := taskStore.AddDependency(ctx, taskB.ID, taskA.ID)
	if err == nil {
		t.Fatal("AddDependency(B, A) expected cycle error, got nil")
	}
	if !errors.Is(err, ErrCycleDetected) {
		t.Errorf("AddDependency(B, A) error = %v, want ErrCycleDetected", err)
	}
}

func TestAddDependency_SelfCycle(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Self Cycle Story"; s.Status = models.StatusNew })
	taskA := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Self Cycle A"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

	err := taskStore.AddDependency(ctx, taskA.ID, taskA.ID)
	if err == nil {
		t.Fatal("AddDependency(A, A) expected self-dependency error, got nil")
	}
	if !errors.Is(err, ErrSelfDependency) {
		t.Errorf("AddDependency(A, A) error = %v, want ErrSelfDependency", err)
	}
}

func TestRemoveDependency(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Remove Dep Story"; s.Status = models.StatusNew })
	taskA := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Remove A"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeCode
	})
	taskB := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Remove B"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

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

	dbConn := testhelpers.SetupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Blockers Story"; s.Status = models.StatusNew })
	taskA := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Blocker A"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeCode
	})
	taskB := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Blocker B"
		ts.Status = models.StatusInProgress
		ts.TaskType = models.TaskTypeCode
	})
	taskC := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Dependent C"
		ts.Status = models.StatusBlocked
		ts.TaskType = models.TaskTypeCode
	})

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

	dbConn := testhelpers.SetupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Detect Cycle Story"; s.Status = models.StatusNew })
	taskA := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Cycle Detect A"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})
	taskB := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Cycle Detect B"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})
	taskC := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Cycle Detect C"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

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

	dbConn := testhelpers.SetupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "GetByStory Story"; s.Status = models.StatusNew })
	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Task 1"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})
	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Task 2"
		ts.Status = models.StatusNew
		ts.TaskType = models.TaskTypeBuild
	})
	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Task 3"
		ts.Status = models.StatusDone
		ts.TaskType = models.TaskTypeReview
	})

	tasks, err := taskStore.GetByStory(ctx, story.ID)
	if err != nil {
		t.Fatalf("GetByStory() error = %v", err)
	}

	if len(tasks) != 3 {
		t.Fatalf("GetByStory() returned %d tasks, want 3", len(tasks))
	}
}
