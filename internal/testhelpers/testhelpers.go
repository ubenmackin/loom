// Package testhelpers provides shared test helpers for the Loom test suite.
package testhelpers

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/ubenmackin/loom/internal/db"
	"github.com/ubenmackin/loom/internal/models"
)

// StoryStoreInterface is the minimal interface needed by CreateTestStory.
type StoryStoreInterface interface {
	Create(ctx context.Context, story *models.Story) error
}

// TaskStoreInterface is the minimal interface needed by CreateTestTask.
type TaskStoreInterface interface {
	Create(ctx context.Context, t *models.Task) error
}

// SessionStoreInterface is the minimal interface needed by CreateTestSession.
type SessionStoreInterface interface {
	Register(ctx context.Context, session *models.Session) error
	UpdateLastSeen(ctx context.Context, id string) error
}

// TemplateStoreInterface is the minimal interface needed by CreateTestTemplate.
type TemplateStoreInterface interface {
	Upsert(ctx context.Context, t *models.PromptTemplate) error
}

// SetupTestDB creates an in-memory SQLite database for testing.
// Uses cache=private for isolation and timestamps for unique DSN names.
// Registers t.Cleanup to close the database.
func SetupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbName := fmt.Sprintf("test_%s_%d", t.Name(), time.Now().UnixNano())
	dsn := "file:" + dbName + "?mode=memory&cache=private"

	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	if _, err := database.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("failed to enable foreign keys: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Logf("close test db: %v", err)
		}
	})
	return database
}

// sessionMu protects sessionCounter from concurrent access.
var sessionMu sync.Mutex

// sessionCounter provides unique session IDs for tests.
var sessionCounter int64

func nextSessionID() string {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	sessionCounter++
	return fmt.Sprintf("session-test-%d", sessionCounter)
}

// CreateTestStory creates a story with default values for testing.
func CreateTestStory(t *testing.T, s StoryStoreInterface, overrides ...func(*models.Story)) *models.Story {
	t.Helper()
	story := &models.Story{
		Title:  fmt.Sprintf("Test Story %d", time.Now().UnixNano()),
		Status: models.StatusNew,
	}
	for _, o := range overrides {
		o(story)
	}
	if err := s.Create(context.Background(), story); err != nil {
		t.Fatalf("failed to create test story: %v", err)
	}
	return story
}

// CreateTestTask creates a task with default values for testing.
func CreateTestTask(t *testing.T, s TaskStoreInterface, overrides ...func(*models.Task)) *models.Task {
	t.Helper()
	ts := &models.Task{
		Title:    fmt.Sprintf("Test Task %d", time.Now().UnixNano()),
		Status:   models.StatusNew,
		TaskType: models.TaskTypeCode,
	}
	for _, o := range overrides {
		o(ts)
	}
	if err := s.Create(context.Background(), ts); err != nil {
		t.Fatalf("failed to create test task: %v", err)
	}
	return ts
}

// CreateTestSession creates a session with default values for testing.
func CreateTestSession(t *testing.T, s SessionStoreInterface, overrides ...func(*models.Session)) *models.Session {
	t.Helper()
	session := &models.Session{
		ID:          nextSessionID(),
		Status:      models.SessionStatusActive,
		HarnessType: "test-harness",
		LastSeenAt:  time.Now(),
	}
	for _, o := range overrides {
		o(session)
	}
	if err := s.Register(context.Background(), session); err != nil {
		t.Fatalf("failed to register test session: %v", err)
	}
	return session
}

// SetSessionLastSeen sets a session's last_seen timestamp by directly updating
// the database. This provides precise control over timestamps for staleness tests.
func SetSessionLastSeen(t *testing.T, dbConn *sql.DB, sessionID string, lastSeen time.Time) {
	t.Helper()
	ctx := context.Background()
	_, err := dbConn.ExecContext(ctx, "UPDATE sessions SET last_seen_at = ? WHERE id = ?", lastSeen.UTC(), sessionID)
	if err != nil {
		t.Fatalf("set session last_seen_at: %v", err)
	}
}

// CreateTestTemplate creates a prompt template with default values for testing.
func CreateTestTemplate(t *testing.T, s TemplateStoreInterface, overrides ...func(*models.PromptTemplate)) *models.PromptTemplate {
	t.Helper()
	tmpl := &models.PromptTemplate{
		TaskType: models.TaskTypeCode,
		Template: "Default prompt template for {{title}}",
	}
	for _, o := range overrides {
		o(tmpl)
	}
	if err := s.Upsert(context.Background(), tmpl); err != nil {
		t.Fatalf("failed to create test template: %v", err)
	}
	return tmpl
}
