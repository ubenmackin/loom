package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/testhelpers"
)

func TestCreate(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	ctx := context.Background()

	story := &models.Story{
		Title:          "Test Story",
		Description:    "A test story description",
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

	dbConn := testhelpers.SetupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	ctx := context.Background()

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) {
		s.Title = "Get Test"
		s.Status = models.StatusReady
	})

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

	dbConn := testhelpers.SetupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	ctx := context.Background()

	testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Story A"; s.Status = models.StatusNew })
	testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Story B"; s.Status = models.StatusReady })
	testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Story C"; s.Status = models.StatusNew })

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

	dbConn := testhelpers.SetupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	ctx := context.Background()

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) {
		s.Title = "Update Test"
		s.Status = models.StatusNew
	})

	story.Title = "Updated Title"
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
	if !got.RequiresBuild {
		t.Errorf("Update() RequiresBuild = false, want true")
	}
}

func TestUpdateStatus_ValidTransition(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	ctx := context.Background()

	validTransitions := []struct {
		from models.Status
		to   models.Status
	}{
		{models.StatusNew, models.StatusReady},
		{models.StatusNew, models.StatusInProgress},
		{models.StatusReady, models.StatusInProgress},
		{models.StatusReady, models.StatusBlocked},
		{models.StatusInProgress, models.StatusBlocked},
		{models.StatusInProgress, models.StatusDone},
		{models.StatusBlocked, models.StatusInProgress},
	}

	for _, tt := range validTransitions {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) {
				s.Title = "Transition " + string(tt.from) + "->" + string(tt.to)
				s.Status = tt.from
			})

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

	dbConn := testhelpers.SetupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	ctx := context.Background()

	invalidTransitions := []struct {
		from models.Status
		to   models.Status
	}{
		{models.StatusNew, models.StatusDone},
		{models.StatusDone, models.StatusReady},
		{models.StatusDone, models.StatusInProgress},
		{models.StatusReady, models.StatusDone},
	}

	for _, tt := range invalidTransitions {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) {
				s.Title = "Invalid " + string(tt.from) + "->" + string(tt.to)
				s.Status = tt.from
			})

			err := storyStore.UpdateStatus(ctx, story.ID, tt.to)
			if err == nil {
				t.Fatalf("UpdateStatus() expected error for %q -> %q, got nil", tt.from, tt.to)
			}
			if !errors.Is(err, ErrInvalidTransition) {
				t.Errorf("UpdateStatus() error = %v, want ErrInvalidTransition", err)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	ctx := context.Background()

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) {
		s.Title = "Delete Test"
		s.Status = models.StatusNew
	})

	if err := storyStore.Delete(ctx, story.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := storyStore.GetByID(ctx, story.ID)
	if err == nil {
		t.Fatal("Delete() story still exists after deletion")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete() GetByID error = %v, want ErrNotFound", err)
	}
}

func TestDelete_NotNew(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	ctx := context.Background()

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) {
		s.Title = "Delete Not New"
		s.Status = models.StatusReady
	})

	err := storyStore.Delete(ctx, story.ID)
	if err == nil {
		t.Fatal("Delete() expected error for non-new status, got nil")
	}
	if !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("Delete() error = %v, want ErrInvalidTransition (story not new)", err)
	}
}

func TestGetWithTasks(t *testing.T) {
	t.Parallel()

	dbConn := testhelpers.SetupTestDB(t)
	storyStore := NewStoryStore(dbConn)
	taskStore := NewTaskStore(dbConn)
	ctx := context.Background()

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) {
		s.Title = "Story With Tasks"
		s.Status = models.StatusReady
	})
	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Task A"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})
	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Task B"
		ts.Status = models.StatusNew
		ts.TaskType = models.TaskTypeBuild
	})

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
