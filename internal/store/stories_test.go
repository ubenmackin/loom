package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/ubenmackin/loom/internal/db"
	"github.com/ubenmackin/loom/internal/models"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	// Use a unique timestamp to ensure no shared state between parallel tests.
	dbName := fmt.Sprintf("test_%s_%d", t.Name(), time.Now().UnixNano())
	dsn := "file:" + dbName + "?mode=memory&cache=shared"

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

func createTestStory(t *testing.T, store *StoryStore, title, status string) *models.Story {
	t.Helper()
	story := &models.Story{Title: title, Status: status}
	if story.Status == "" {
		story.Status = models.StatusNew
	}
	if err := store.Create(context.Background(), story); err != nil {
		t.Fatalf("create test story %q: %v", title, err)
	}
	return story
}

func createTestTask(t *testing.T, store *TaskStore, storyID, title, status, taskType string) *models.Task {
	t.Helper()
	task := &models.Task{StoryID: storyID, Title: title, Status: status, TaskType: taskType}
	if task.Status == "" {
		task.Status = models.StatusNew
	}
	if task.TaskType == "" {
		task.TaskType = models.TaskTypeCode
	}
	if err := store.Create(context.Background(), task); err != nil {
		t.Fatalf("create test task %q: %v", title, err)
	}
	return task
}

func TestCreate(t *testing.T) {
	t.Parallel()

	dbConn := setupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	ctx := context.Background()

	story := &models.Story{
		Title:          "Test Story",
		Description:    "A test story description",
		Priority:       1,
		RequiresBuild:  true,
		RequiresReview: false,
	}

	if err := storyStore.Create(ctx, story); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if story.ID == "" {
		t.Fatal("Create() did not generate an ID")
	}

	if !strings.HasPrefix(story.ID, "STORY-") {
		t.Fatalf("Create() ID %q does not match STORY-NNN format", story.ID)
	}

	if story.Status != models.StatusNew {
		t.Fatalf("Create() status = %q, want %q", story.Status, models.StatusNew)
	}

	if story.CreatedAt.IsZero() {
		t.Fatal("Create() CreatedAt is zero")
	}

	if story.UpdatedAt.IsZero() {
		t.Fatal("Create() UpdatedAt is zero")
	}
}

func TestGetByID(t *testing.T) {
	t.Parallel()

	dbConn := setupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Get Test", models.StatusReady)

	got, err := storyStore.GetByID(ctx, story.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if got.ID != story.ID {
		t.Errorf("GetByID() ID = %q, want %q", got.ID, story.ID)
	}
	if got.Title != story.Title {
		t.Errorf("GetByID() Title = %q, want %q", got.Title, story.Title)
	}
	if got.Status != story.Status {
		t.Errorf("GetByID() Status = %q, want %q", got.Status, story.Status)
	}
}

func TestList(t *testing.T) {
	t.Parallel()

	dbConn := setupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	ctx := context.Background()

	createTestStory(t, storyStore, "Story A", models.StatusNew)
	createTestStory(t, storyStore, "Story B", models.StatusReady)
	createTestStory(t, storyStore, "Story C", models.StatusNew)

	t.Run("no filter", func(t *testing.T) {
		all, err := storyStore.List(ctx, StoryFilter{})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(all) != 3 {
			t.Fatalf("List() returned %d stories, want 3", len(all))
		}
	})

	t.Run("filter by status new", func(t *testing.T) {
		newStories, err := storyStore.List(ctx, StoryFilter{Status: models.StatusNew})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(newStories) != 2 {
			t.Fatalf("List() returned %d new stories, want 2", len(newStories))
		}
	})

	t.Run("filter by status ready", func(t *testing.T) {
		readyStories, err := storyStore.List(ctx, StoryFilter{Status: models.StatusReady})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(readyStories) != 1 {
			t.Fatalf("List() returned %d ready stories, want 1", len(readyStories))
		}
	})
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	dbConn := setupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Update Test", models.StatusNew)

	story.Title = "Updated Title"
	story.Priority = 5
	story.RequiresBuild = true

	if err := storyStore.Update(ctx, story); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, err := storyStore.GetByID(ctx, story.ID)
	if err != nil {
		t.Fatalf("GetByID() after update error = %v", err)
	}

	if got.Title != "Updated Title" {
		t.Errorf("Update() Title = %q, want %q", got.Title, "Updated Title")
	}
	if got.Priority != 5 {
		t.Errorf("Update() Priority = %d, want 5", got.Priority)
	}
	if !got.RequiresBuild {
		t.Errorf("Update() RequiresBuild = false, want true")
	}
}

func TestUpdateStatus_ValidTransition(t *testing.T) {
	t.Parallel()

	dbConn := setupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	ctx := context.Background()

	validTransitions := []struct {
		from string
		to   string
	}{
		{models.StatusNew, models.StatusReady},
		{models.StatusNew, models.StatusInProgress},
		{models.StatusReady, models.StatusInProgress},
		{models.StatusReady, models.StatusBlocked},
		{models.StatusInProgress, models.StatusBlocked},
		{models.StatusInProgress, models.StatusDone},
		{models.StatusBlocked, models.StatusInProgress},
		{models.StatusBlocked, models.StatusReady},
	}

	for _, tt := range validTransitions {
		t.Run(tt.from+"->"+tt.to, func(t *testing.T) {
			story := createTestStory(t, storyStore, "Transition "+tt.from+"->"+tt.to, tt.from)

			if err := storyStore.UpdateStatus(ctx, story.ID, tt.to); err != nil {
				t.Fatalf("UpdateStatus() error = %v", err)
			}

			got, err := storyStore.GetByID(ctx, story.ID)
			if err != nil {
				t.Fatalf("GetByID() error = %v", err)
			}
			if got.Status != tt.to {
				t.Errorf("UpdateStatus() status = %q, want %q", got.Status, tt.to)
			}
		})
	}
}

func TestUpdateStatus_InvalidTransition(t *testing.T) {
	t.Parallel()

	dbConn := setupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	ctx := context.Background()

	invalidTransitions := []struct {
		from string
		to   string
	}{
		{models.StatusNew, models.StatusDone},
		{models.StatusDone, models.StatusReady},
		{models.StatusDone, models.StatusInProgress},
		{models.StatusReady, models.StatusDone},
	}

	for _, tt := range invalidTransitions {
		t.Run(tt.from+"->"+tt.to, func(t *testing.T) {
			story := createTestStory(t, storyStore, "Invalid "+tt.from+"->"+tt.to, tt.from)

			err := storyStore.UpdateStatus(ctx, story.ID, tt.to)
			if err == nil {
				t.Fatalf("UpdateStatus() expected error for %q -> %q, got nil", tt.from, tt.to)
			}
			if !strings.Contains(err.Error(), "invalid transition") {
				t.Errorf("UpdateStatus() error = %v, want 'invalid transition'", err)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()

	dbConn := setupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Delete Test", models.StatusNew)

	if err := storyStore.Delete(ctx, story.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := storyStore.GetByID(ctx, story.ID)
	if err == nil {
		t.Fatal("Delete() story still exists after deletion")
	}
	if err != sql.ErrNoRows && !strings.Contains(err.Error(), "no rows") {
		t.Fatalf("Delete() GetByID error = %v, want ErrNoRows", err)
	}
}

func TestDelete_NotNew(t *testing.T) {
	t.Parallel()

	dbConn := setupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Delete Not New", models.StatusReady)

	err := storyStore.Delete(ctx, story.ID)
	if err == nil {
		t.Fatal("Delete() expected error for non-new status, got nil")
	}
	if !strings.Contains(err.Error(), "cannot delete") {
		t.Errorf("Delete() error = %v, want 'cannot delete'", err)
	}
}

func TestGetWithTasks(t *testing.T) {
	t.Parallel()

	dbConn := setupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	story := createTestStory(t, storyStore, "Story With Tasks", models.StatusReady)
	createTestTask(t, taskStore, story.ID, "Task A", models.StatusReady, models.TaskTypeCode)
	createTestTask(t, taskStore, story.ID, "Task B", models.StatusNew, models.TaskTypeBuild)

	gotStory, tasks, err := storyStore.GetWithTasks(ctx, story.ID)
	if err != nil {
		t.Fatalf("GetWithTasks() error = %v", err)
	}

	if gotStory.ID != story.ID {
		t.Errorf("GetWithTasks() story ID = %q, want %q", gotStory.ID, story.ID)
	}

	if len(tasks) != 2 {
		t.Fatalf("GetWithTasks() returned %d tasks, want 2", len(tasks))
	}
}
