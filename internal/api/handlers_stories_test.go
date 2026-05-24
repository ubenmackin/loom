package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/dispatcher"
	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
	"github.com/ubenmackin/loom/internal/testhelpers"
)

func newTestRouterStories(t *testing.T) (chi.Router, *store.StoryStore, *store.TaskStore, *store.SessionStore, *store.CommentStore, *store.TemplateStore, *store.ActivityStore) {
	t.Helper()

	// Set agent secret so SessionAuthenticator allows /sessions and /work routes via X-Agent-Secret.
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

	return mux, storyStore, taskStore, sessionStore, commentStore, templateStore, activityStore
}

func TestCreateStory(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _ := newTestRouterStories(t)

	rr := doRequest(t, mux, "POST", "/api/stories", map[string]any{
		"title":          "Test Story",
		"description":    "A test story",
		"requires_build": true,
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("createStory status = %d, want %d", rr.Code, http.StatusCreated)
	}

	var resp map[string]any
	decodeRespJSON(t, rr, &resp)

	if resp["id"] == "" {
		t.Fatal("createStory response missing ID")
	}

	title, _ := resp["title"].(string)
	if title != "Test Story" {
		t.Errorf("createStory title = %q, want %q", title, "Test Story")
	}

	status, _ := resp["status"].(string)
	if models.Status(status) != models.StatusNew {
		t.Errorf("createStory status = %q, want %q", status, models.StatusNew)
	}

	requiresBuild, _ := resp["requires_build"].(bool)
	if !requiresBuild {
		t.Errorf("createStory requires_build = false, want true")
	}
}

func TestCreateStory_MissingTitle(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _ := newTestRouterStories(t)

	rr := doRequest(t, mux, "POST", "/api/stories", map[string]any{
		"description": "No title",
	})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("createStory status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestGetStory(t *testing.T) {
	t.Parallel()

	mux, storyStore, taskStore, _, _, _, _ := newTestRouterStories(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Get Story"; s.Status = models.StatusReady })
	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Task 1"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

	rr := doRequest(t, mux, "GET", "/api/stories/"+story.ID, nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("getStory status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]any
	decodeRespJSON(t, rr, &resp)

	storyData, ok := resp["story"].(map[string]any)
	if !ok {
		t.Fatal("getStory response missing 'story' field")
	}

	id, _ := storyData["id"].(string)
	if id != story.ID {
		t.Errorf("getStory story ID = %q, want %q", id, story.ID)
	}

	tasksData, ok := resp["tasks"].([]any)
	if !ok {
		t.Fatal("getStory response missing 'tasks' field")
	}

	if len(tasksData) != 1 {
		t.Fatalf("getStory returned %d tasks, want 1", len(tasksData))
	}
}

func TestGetStory_NotFound(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _ := newTestRouterStories(t)

	rr := doRequest(t, mux, "GET", "/api/stories/STORY-999", nil)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("getStory status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestListStories(t *testing.T) {
	t.Parallel()

	mux, storyStore, _, _, _, _, _ := newTestRouterStories(t)

	testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Story A"; s.Status = models.StatusNew })
	testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Story B"; s.Status = models.StatusReady })
	testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Story C"; s.Status = models.StatusNew })

	t.Run("list all", func(t *testing.T) {
		rr := doRequest(t, mux, "GET", "/api/stories", nil)

		if rr.Code != http.StatusOK {
			t.Fatalf("listStories status = %d, want %d", rr.Code, http.StatusOK)
		}

		var stories []map[string]any
		decodeRespJSON(t, rr, &stories)

		if len(stories) != 3 {
			t.Fatalf("listStories returned %d stories, want 3", len(stories))
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		rr := doRequest(t, mux, "GET", "/api/stories?status=new", nil)

		if rr.Code != http.StatusOK {
			t.Fatalf("listStories status = %d, want %d", rr.Code, http.StatusOK)
		}

		var stories []map[string]any
		decodeRespJSON(t, rr, &stories)

		if len(stories) != 2 {
			t.Fatalf("listStories(status=new) returned %d stories, want 2", len(stories))
		}
	})

	t.Run("filter by status ready", func(t *testing.T) {
		rr := doRequest(t, mux, "GET", "/api/stories?status=ready", nil)

		if rr.Code != http.StatusOK {
			t.Fatalf("listStories status = %d, want %d", rr.Code, http.StatusOK)
		}

		var stories []map[string]any
		decodeRespJSON(t, rr, &stories)

		if len(stories) != 1 {
			t.Fatalf("listStories(status=ready) returned %d stories, want 1", len(stories))
		}
	})
}

func TestUpdateStory(t *testing.T) {
	t.Parallel()

	mux, storyStore, _, _, _, _, _ := newTestRouterStories(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Original Title"; s.Status = models.StatusNew })

	rr := doRequest(t, mux, "PUT", "/api/stories/"+story.ID, map[string]any{
		"title": "Updated Title",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("updateStory status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]any
	decodeRespJSON(t, rr, &resp)

	title, _ := resp["title"].(string)
	if title != "Updated Title" {
		t.Errorf("updateStory title = %q, want %q", title, "Updated Title")
	}
}

func TestUpdateStory_NotFound(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _ := newTestRouterStories(t)

	rr := doRequest(t, mux, "PUT", "/api/stories/STORY-999", map[string]any{
		"title": "Updated",
	})

	if rr.Code != http.StatusNotFound {
		t.Fatalf("updateStory status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestUpdateStory_PartialUpdate(t *testing.T) {
	t.Parallel()

	mux, storyStore, _, _, _, _, _ := newTestRouterStories(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Partial Update"; s.Status = models.StatusNew })

	rr := doRequest(t, mux, "PUT", "/api/stories/"+story.ID, map[string]any{
		"description": "New description",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("updateStory status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]any
	decodeRespJSON(t, rr, &resp)

	desc, _ := resp["description"].(string)
	if desc != "New description" {
		t.Errorf("updateStory description = %q, want %q", desc, "New description")
	}
}

func TestDeleteStory(t *testing.T) {
	t.Parallel()

	mux, storyStore, _, _, _, _, _ := newTestRouterStories(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Delete Story"; s.Status = models.StatusNew })

	rr := doRequest(t, mux, "DELETE", "/api/stories/"+story.ID, nil)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("deleteStory status = %d, want %d", rr.Code, http.StatusNoContent)
	}

	_, err := storyStore.GetByID(context.Background(), story.ID)
	if err == nil {
		t.Fatal("deleteStory: story still exists after deletion")
	}
}

func TestDeleteStory_NotNew(t *testing.T) {
	t.Parallel()

	mux, storyStore, _, _, _, _, _ := newTestRouterStories(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Delete Not New"; s.Status = models.StatusReady })

	rr := doRequest(t, mux, "DELETE", "/api/stories/"+story.ID, nil)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("deleteStory status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	decodeRespJSON(t, rr, &resp)

	if resp["error"] == "" {
		t.Fatal("deleteStory response missing error message")
	}
}

func TestDeleteStory_NotFound(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _ := newTestRouterStories(t)

	rr := doRequest(t, mux, "DELETE", "/api/stories/STORY-999", nil)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("deleteStory status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestCreateStory_WithAllFields(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _ := newTestRouterStories(t)

	rr := doRequest(t, mux, "POST", "/api/stories", map[string]any{
		"title":           "Full Story",
		"description":     "Full description",
		"requires_build":  true,
		"requires_review": true,
		"assigned_to":     "user-1",
		"assignee_type":   "human",
		"sort_order":      10,
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("createStory status = %d, want %d", rr.Code, http.StatusCreated)
	}

	var resp map[string]any
	decodeRespJSON(t, rr, &resp)

	if resp["title"] != "Full Story" {
		t.Errorf("createStory title = %v, want %q", resp["title"], "Full Story")
	}

	requiresReview, _ := resp["requires_review"].(bool)
	if !requiresReview {
		t.Errorf("createStory requires_review = false, want true")
	}

	assignedTo, _ := resp["assigned_to"].(string)
	if assignedTo != "user-1" {
		t.Errorf("createStory assigned_to = %q, want %q", assignedTo, "user-1")
	}
}

func TestListStories_Empty(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _ := newTestRouterStories(t)

	rr := doRequest(t, mux, "GET", "/api/stories", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("listStories status = %d, want %d", rr.Code, http.StatusOK)
	}

	var stories []any
	decodeRespJSON(t, rr, &stories)

	if len(stories) != 0 {
		t.Fatalf("listStories returned %d stories, want 0", len(stories))
	}
}

func TestGetStory_EmptyTasks(t *testing.T) {
	t.Parallel()

	mux, storyStore, _, _, _, _, _ := newTestRouterStories(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "No Tasks Story"; s.Status = models.StatusNew })

	rr := doRequest(t, mux, "GET", "/api/stories/"+story.ID, nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("getStory status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]any
	decodeRespJSON(t, rr, &resp)

	tasks, ok := resp["tasks"].([]any)
	if !ok {
		t.Fatal("getStory response missing 'tasks' field")
	}

	if len(tasks) != 0 {
		t.Fatalf("getStory returned %d tasks, want 0", len(tasks))
	}
}

func TestUpdateStoryStatus(t *testing.T) {
	t.Parallel()

	mux, storyStore, _, _, _, _, _ := newTestRouterStories(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Status Update Story"; s.Status = models.StatusNew })

	rr := doRequest(t, mux, "PATCH", "/api/stories/"+story.ID+"/status", map[string]string{
		"status": "ready",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("updateStoryStatus status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]any
	decodeRespJSON(t, rr, &resp)

	status, _ := resp["status"].(string)
	if models.Status(status) != models.StatusReady {
		t.Errorf("updateStoryStatus status = %q, want %q", status, models.StatusReady)
	}
}

func TestUpdateStoryStatus_Invalid(t *testing.T) {
	t.Parallel()

	mux, storyStore, _, _, _, _, _ := newTestRouterStories(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Invalid Status Story"; s.Status = models.StatusNew })

	rr := doRequest(t, mux, "PATCH", "/api/stories/"+story.ID+"/status", map[string]string{
		"status": "done",
	})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("updateStoryStatus status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateStory_InvalidJSON(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _ := newTestRouterStories(t)

	rr := doRequest(t, mux, "POST", "/api/stories", nil)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("createStory status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestUpdateStory_InvalidJSON(t *testing.T) {
	t.Parallel()

	mux, storyStore, _, _, _, _, _ := newTestRouterStories(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Invalid JSON Story"; s.Status = models.StatusNew })

	rr := doRequest(t, mux, "PUT", "/api/stories/"+story.ID, nil)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("updateStory status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateTaskUnderStory(t *testing.T) {
	t.Parallel()

	mux, storyStore, _, _, _, _, _ := newTestRouterStories(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Task Under Story"; s.Status = models.StatusReady })

	rr := doRequest(t, mux, "POST", "/api/stories/"+story.ID+"/tasks", map[string]any{
		"title":     "New Task",
		"task_type": "code",
		"status":    "ready",
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("createTaskUnderStory status = %d, want %d", rr.Code, http.StatusCreated)
	}

	var resp map[string]any
	decodeRespJSON(t, rr, &resp)

	id, _ := resp["id"].(string)
	if id == "" {
		t.Fatal("createTaskUnderStory response missing ID")
	}

	storyID, _ := resp["story_id"].(string)
	if storyID != story.ID {
		t.Errorf("createTaskUnderStory story_id = %q, want %q", storyID, story.ID)
	}

	title, _ := resp["title"].(string)
	if title != "New Task" {
		t.Errorf("createTaskUnderStory title = %q, want %q", title, "New Task")
	}
}

func TestCreateTaskUnderStory_InvalidStory(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _ := newTestRouterStories(t)

	rr := doRequest(t, mux, "POST", "/api/stories/STORY-999/tasks", map[string]any{
		"title": "New Task",
	})

	if rr.Code != http.StatusNotFound {
		t.Fatalf("createTaskUnderStory status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestGetBoard(t *testing.T) {
	t.Parallel()

	mux, storyStore, taskStore, _, _, _, _ := newTestRouterStories(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Board Story"; s.Status = models.StatusReady })
	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Board Task"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

	rr := doRequest(t, mux, "GET", "/api/board", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("getBoard status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]any
	decodeRespJSON(t, rr, &resp)

	stories, ok := resp["stories"].([]any)
	if !ok {
		t.Fatal("getBoard response missing 'stories' field")
	}

	if len(stories) != 1 {
		t.Fatalf("getBoard returned %d stories, want 1", len(stories))
	}
}

func TestCORSHeaders(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _ := newTestRouterStories(t)

	req := httptest.NewRequest("OPTIONS", "/api/stories", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("CORS OPTIONS status = %d, want %d", rr.Code, http.StatusNoContent)
	}

	if rr.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("CORS Allow-Origin = %q, want %q", rr.Header().Get("Access-Control-Allow-Origin"), "http://localhost:5173")
	}
}

func TestSessionRegister(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _ := newTestRouterStories(t)

	rr := doRequest(t, mux, "POST", "/api/sessions/register", map[string]any{
		"harness_type": "opencode",
		"capabilities": `["code","build"]`,
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("registerSession status = %d, want %d", rr.Code, http.StatusCreated)
	}

	var resp map[string]any
	decodeRespJSON(t, rr, &resp)

	harnessType, _ := resp["harness_type"].(string)
	if harnessType != "opencode" {
		t.Errorf("registerSession harness_type = %q, want %q", harnessType, "opencode")
	}

	status, _ := resp["status"].(string)
	if models.SessionStatus(status) != models.SessionStatusActive {
		t.Errorf("registerSession status = %q, want %q", status, models.SessionStatusActive)
	}
}

func TestSessionRegister_MissingHarnessType(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _ := newTestRouterStories(t)

	rr := doRequest(t, mux, "POST", "/api/sessions/register", map[string]any{})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("registerSession status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestSessionGet(t *testing.T) {
	t.Parallel()

	mux, _, _, sessionStore, _, _, _ := newTestRouterStories(t)

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})

	rr := doRequest(t, mux, "GET", "/api/sessions/"+session.ID, nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("getSession status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]any
	decodeRespJSON(t, rr, &resp)

	id, _ := resp["id"].(string)
	if id != session.ID {
		t.Errorf("getSession id = %q, want %q", id, session.ID)
	}
}

func TestSessionDisconnect(t *testing.T) {
	t.Parallel()

	mux, _, _, sessionStore, _, _, _ := newTestRouterStories(t)

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
	})

	rr := doRequest(t, mux, "DELETE", "/api/sessions/"+session.ID, nil)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("disconnectSession status = %d, want %d", rr.Code, http.StatusNoContent)
	}

	got, err := sessionStore.GetByID(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if got.Status != models.SessionStatusDisconnected {
		t.Errorf("disconnectSession status = %q, want %q", got.Status, models.SessionStatusDisconnected)
	}
}

func TestSessionGet_NotFound(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _ := newTestRouterStories(t)

	rr := doRequest(t, mux, "GET", "/api/sessions/nonexistent", nil)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("getSession status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestSessionGetTasks(t *testing.T) {
	t.Parallel()

	mux, storyStore, taskStore, sessionStore, _, _, _ := newTestRouterStories(t)

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Session Tasks Story"; s.Status = models.StatusReady })
	task := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Session Task"
		ts.Status = models.StatusInProgress
		ts.TaskType = models.TaskTypeCode
	})
	task.AssignedTo = session.ID
	task.AssigneeType = models.AssigneeTypeSession
	if err := taskStore.Update(context.Background(), task); err != nil {
		t.Fatalf("update task assignment: %v", err)
	}

	rr := doRequest(t, mux, "GET", "/api/sessions/"+session.ID+"/tasks", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("getSessionTasks status = %d, want %d", rr.Code, http.StatusOK)
	}

	var tasks []map[string]any
	decodeRespJSON(t, rr, &tasks)

	if len(tasks) != 1 {
		t.Fatalf("getSessionTasks returned %d tasks, want 1", len(tasks))
	}
}

func TestCreateTemplate(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, templateStore, _ := newTestRouterStories(t)

	// Use upsert (PUT) to create a template.
	rr := doRequest(t, mux, "PUT", "/api/templates/code", map[string]any{
		"template": "Implement {{task.title}} for {{story.title}}",
	})

	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Fatalf("createTemplate status = %d, want 200 or 201", rr.Code)
	}

	var resp map[string]any
	decodeRespJSON(t, rr, &resp)

	template, _ := resp["template"].(string)
	if template != "Implement {{task.title}} for {{story.title}}" {
		t.Errorf("createTemplate template = %q, want %q", template, "Implement {{task.title}} for {{story.title}}")
	}

	_ = templateStore
}

func TestGetTemplate(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, templateStore, _ := newTestRouterStories(t)

	testhelpers.CreateTestTemplate(t, templateStore, func(tmpl *models.PromptTemplate) {
		tmpl.TaskType = "code"
		tmpl.Template = "Code template"
	})

	rr := doRequest(t, mux, "GET", "/api/templates/code", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("getTemplate status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]any
	decodeRespJSON(t, rr, &resp)

	template, _ := resp["template"].(string)
	if template != "Code template" {
		t.Errorf("getTemplate template = %q, want %q", template, "Code template")
	}
}

func TestListTemplates(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, templateStore, _ := newTestRouterStories(t)

	testhelpers.CreateTestTemplate(t, templateStore, func(tmpl *models.PromptTemplate) {
		tmpl.TaskType = "code"
		tmpl.Template = "Code template"
	})
	testhelpers.CreateTestTemplate(t, templateStore, func(tmpl *models.PromptTemplate) {
		tmpl.TaskType = "build"
		tmpl.Template = "Build template"
	})

	rr := doRequest(t, mux, "GET", "/api/templates", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("listTemplates status = %d, want %d", rr.Code, http.StatusOK)
	}

	var templates []map[string]any
	decodeRespJSON(t, rr, &templates)

	if len(templates) != 2 {
		t.Fatalf("listTemplates returned %d templates, want 2", len(templates))
	}
}

func TestUpsertTemplate(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, templateStore, _ := newTestRouterStories(t)

	// Create initial template.
	testhelpers.CreateTestTemplate(t, templateStore, func(tmpl *models.PromptTemplate) {
		tmpl.TaskType = "review"
		tmpl.Template = "Original review template"
	})

	// Upsert with new template text.
	rr := doRequest(t, mux, "PUT", "/api/templates/review", map[string]any{
		"template": "Updated review template",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("upsertTemplate status = %d, want 200", rr.Code)
	}

	// Verify the template was updated.
	rr = doRequest(t, mux, "GET", "/api/templates/review", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("getTemplate status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]any
	decodeRespJSON(t, rr, &resp)

	template, _ := resp["template"].(string)
	if template != "Updated review template" {
		t.Errorf("upsertTemplate template = %q, want %q", template, "Updated review template")
	}
}

func TestCreateComment(t *testing.T) {
	t.Parallel()

	mux, storyStore, _, _, commentStore, _, _ := newTestRouterStories(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Comment Story"; s.Status = models.StatusNew })

	rr := doRequest(t, mux, "POST", "/api/work-items/"+story.ID+"/comments", map[string]any{
		"work_item_type": "story",
		"author_id":      "user-1",
		"author_type":    "human",
		"body":           "This is a comment",
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("createComment status = %d, want %d", rr.Code, http.StatusCreated)
	}

	var resp map[string]any
	decodeRespJSON(t, rr, &resp)

	body, _ := resp["body"].(string)
	if body != "This is a comment" {
		t.Errorf("createComment body = %q, want %q", body, "This is a comment")
	}

	_ = commentStore
}

func TestGetComments(t *testing.T) {
	t.Parallel()

	mux, storyStore, _, _, commentStore, _, _ := newTestRouterStories(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Get Comments Story"; s.Status = models.StatusNew })
	createTestComment(t, commentStore, story.ID, "story", "user-1", "human", "Comment 1")
	createTestComment(t, commentStore, story.ID, "story", "user-2", "human", "Comment 2")

	rr := doRequest(t, mux, "GET", "/api/work-items/"+story.ID+"/comments?type=story", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("getComments status = %d, want %d", rr.Code, http.StatusOK)
	}

	var comments []map[string]any
	decodeRespJSON(t, rr, &comments)

	if len(comments) != 2 {
		t.Fatalf("getComments returned %d comments, want 2", len(comments))
	}
}

func TestUpdateStory_RequiresBuild(t *testing.T) {
	t.Parallel()

	mux, storyStore, _, _, _, _, _ := newTestRouterStories(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Requires Build Story"; s.Status = models.StatusNew })

	rr := doRequest(t, mux, "PUT", "/api/stories/"+story.ID, map[string]any{
		"requires_build": true,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("updateStory status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]any
	decodeRespJSON(t, rr, &resp)

	requiresBuild, _ := resp["requires_build"].(bool)
	if !requiresBuild {
		t.Errorf("updateStory requires_build = false, want true")
	}
}

func TestUpdateStory_RequiresReview(t *testing.T) {
	t.Parallel()

	mux, storyStore, _, _, _, _, _ := newTestRouterStories(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Requires Review Story"; s.Status = models.StatusNew })

	rr := doRequest(t, mux, "PUT", "/api/stories/"+story.ID, map[string]any{
		"requires_review": true,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("updateStory status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]any
	decodeRespJSON(t, rr, &resp)

	requiresReview, _ := resp["requires_review"].(bool)
	if !requiresReview {
		t.Errorf("updateStory requires_review = false, want true")
	}
}

func TestCreateStory_EmptyBody(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _ := newTestRouterStories(t)

	rr := doRequest(t, mux, "POST", "/api/stories", nil)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("createStory status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestCreateStory_MalformedJSON(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _ := newTestRouterStories(t)

	// Send raw malformed JSON body (not valid JSON).
	req := httptest.NewRequest("POST", "/api/stories", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	if token := lookupAuthToken(t.Name()); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("createStory malformed JSON status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp["error"] == "" {
		t.Fatal("expected error message in response")
	}
}

func TestListStories_AssignedToFilter(t *testing.T) {
	t.Parallel()

	mux, storyStore, _, _, _, _, _ := newTestRouterStories(t)

	storyA := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Assigned Story A"; s.Status = models.StatusNew })
	storyA.AssignedTo = "user-1"
	if err := storyStore.Update(context.Background(), storyA); err != nil {
		t.Fatalf("update storyA: %v", err)
	}

	storyB := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Assigned Story B"; s.Status = models.StatusNew })
	storyB.AssignedTo = "user-2"
	if err := storyStore.Update(context.Background(), storyB); err != nil {
		t.Fatalf("update storyB: %v", err)
	}

	rr := doRequest(t, mux, "GET", "/api/stories?assigned_to=user-1", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("listStories status = %d, want %d", rr.Code, http.StatusOK)
	}

	var stories []map[string]any
	decodeRespJSON(t, rr, &stories)

	if len(stories) != 1 {
		t.Fatalf("listStories(assigned_to=user-1) returned %d stories, want 1", len(stories))
	}

	id, _ := stories[0]["id"].(string)
	if id != storyA.ID {
		t.Errorf("listStories story ID = %q, want %q", id, storyA.ID)
	}
}

func TestUpdateStoryStatus_MissingStatus(t *testing.T) {
	t.Parallel()

	mux, storyStore, _, _, _, _, _ := newTestRouterStories(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Missing Status Story"; s.Status = models.StatusNew })

	rr := doRequest(t, mux, "PATCH", "/api/stories/"+story.ID+"/status", map[string]string{})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("updateStoryStatus status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}
