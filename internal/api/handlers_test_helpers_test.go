package api

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/dispatcher"
	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
	"github.com/ubenmackin/loom/internal/testhelpers"
)

// testBroadcaster implements dispatcher.EventBroadcaster for testing.
type testBroadcaster struct{}

func (m *testBroadcaster) Broadcast(eventType string, payload any) {}

// mockHub implements HubInterface for testing.
type mockHub struct{}

func (m *mockHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusSwitchingProtocols)
}

// newTestRouter creates a fully wired chi.Router backed by an isolated SQLite database,
// along with all store references for test assertions. This is the single shared
// helper for all API tests, eliminating the 88% duplication between the former
// newTestRouter and newTestRouterStories.
// newTestRouter creates a fully wired chi.Router backed by an isolated SQLite database,
// along with all store references for test assertions. This is the single shared
// helper for all API tests, eliminating the 88% duplication between the former
// newTestRouter and newTestRouterStories.
func newTestRouter(t *testing.T) (
	chi.Router,
	*store.StoryStore,
	*store.TaskStore,
	*store.SessionStore,
	*store.CommentStore,
	*store.TemplateStore,
	*store.ActivityStore,
) {
	t.Helper()

	mux, storyStore, taskStore, sessionStore, commentStore, templateStore, activityStore, _ := newTestRouterWithDB(t)
	return mux, storyStore, taskStore, sessionStore, commentStore, templateStore, activityStore
}

// newTestRouterWithDB is like newTestRouter but also returns the raw *sql.DB
// for tests that need direct database manipulation (e.g., staleness tests).
func newTestRouterWithDB(t *testing.T) (
	chi.Router,
	*store.StoryStore,
	*store.TaskStore,
	*store.SessionStore,
	*store.CommentStore,
	*store.TemplateStore,
	*store.ActivityStore,
	*sql.DB,
) {
	t.Helper()

	// Set agent secret so SessionAuthenticator allows /sessions and /work routes
	// via X-Agent-Secret.
	origAgentSecret := os.Getenv("LOOM_AGENT_SECRET")
	os.Setenv("LOOM_AGENT_SECRET", "test-agent-secret")
	t.Cleanup(func() {
		if origAgentSecret == "" {
			os.Unsetenv("LOOM_AGENT_SECRET")
		} else {
			os.Setenv("LOOM_AGENT_SECRET", origAgentSecret)
		}
	})

	dbConn := testhelpers.SetupTestDB(t)

	storyStore := store.NewStoryStore(dbConn)
	taskStore := store.NewTaskStore(dbConn)
	sessionStore := store.NewSessionStore(dbConn)
	commentStore := store.NewCommentStore(dbConn)
	templateStore := store.NewTemplateStore(dbConn)
	activityStore := store.NewActivityStore(dbConn)
	userStore := store.NewUserStore(dbConn)

	// Create a test user and session token so protected routes work in tests.
	testUser, err := userStore.CreateUser(context.Background(), "testuser", "test@example.com", "Test User", "password123", models.RoleNormal)
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}
	testToken, err := userStore.CreateSession(context.Background(), testUser.ID)
	if err != nil {
		t.Fatalf("create test user session: %v", err)
	}
	testAuthTokens.Store(t.Name(), testToken)
	t.Cleanup(func() { testAuthTokens.Delete(t.Name()) })

	broadcaster := &testBroadcaster{}
	d := dispatcher.NewDispatcher(dispatcher.DispatcherDeps{
		StoryStore:         storyStore,
		TaskStore:          taskStore,
		SessionStore:       sessionStore,
		TemplateStore:      templateStore,
		CommentStore:       commentStore,
		ActivityStore:      activityStore,
		Broadcaster:        broadcaster,
		StalenessThreshold: 30 * time.Minute,
	})

	apiRouter := NewRouter(
		storyStore,
		taskStore,
		sessionStore,
		commentStore,
		templateStore,
		activityStore,
		userStore,
		d,
		&mockHub{},
	)

	// Mount under /api — same as production (cmd/server/main.go).
	mux := chi.NewRouter()
	mux.Mount("/api", apiRouter)

	return mux, storyStore, taskStore, sessionStore, commentStore, templateStore, activityStore, dbConn
}
