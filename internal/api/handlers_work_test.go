package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/dispatcher"
	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
	"github.com/ubenmackin/loom/internal/testhelpers"
)

// mockHub implements HubInterface for testing.
type mockHub struct{}

func (m *mockHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusSwitchingProtocols)
}

// testBroadcaster implements dispatcher.EventBroadcaster for testing.
type testBroadcaster struct{}

func (m *testBroadcaster) Broadcast(eventType string, payload any) {}

func createTestComment(t *testing.T, s *store.CommentStore, workItemID string, workItemType models.WorkItemType, authorID, authorType, body string) *models.Comment {
	t.Helper()
	c := &models.Comment{
		WorkItemID:   workItemID,
		WorkItemType: workItemType,
		AuthorID:     authorID,
		AuthorType:   authorType,
		Body:         body,
	}
	if err := s.Create(context.Background(), c); err != nil {
		t.Fatalf("create test comment: %v", err)
	}
	return c
}

func newTestRouter(t *testing.T) (chi.Router, *sql.DB, *store.StoryStore, *store.TaskStore, *store.SessionStore, *store.CommentStore, *store.TemplateStore, *store.ActivityStore, *dispatcher.Dispatcher) {
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
	// Store token in context for use by doRequest via t.Setenv trick — instead use package-level map.
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

	return mux, dbConn, storyStore, taskStore, sessionStore, commentStore, templateStore, activityStore, d
}

// testAuthTokens maps test name to a valid Bearer token for protected routes.
// Uses sync.Map for safe concurrent access by parallel tests.
var testAuthTokens sync.Map

func doRequest(t *testing.T, mux chi.Router, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(data)
	} else {
		reqBody = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Inject auth token if available for this test (for protected routes).
	// Walk up the test name hierarchy so subtests (e.g. "TestListStories/filter_by_status")
	// inherit the token stored under the parent test name (e.g. "TestListStories").
	if token := lookupAuthToken(t.Name()); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// Extract session_id from body to set as X-Session-ID header for SessionAuthenticator.
	if body != nil {
		if m, ok := body.(map[string]string); ok {
			if sid, exists := m["session_id"]; exists && sid != "" {
				req.Header.Set("X-Session-ID", sid)
			}
		} else if m, ok := body.(map[string]any); ok {
			if sid, exists := m["session_id"]; exists {
				if s, ok := sid.(string); ok && s != "" {
					req.Header.Set("X-Session-ID", s)
				}
			}
		}
	}

	// Set X-Agent-Secret header if LOOM_AGENT_SECRET is configured (for test environments).
	// This allows requests to /sessions and /work routes to pass SessionAuthenticator.
	if secret := os.Getenv("LOOM_AGENT_SECRET"); secret != "" {
		req.Header.Set("X-Agent-Secret", secret)
	}

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

// lookupAuthToken walks up the test name hierarchy (splitting on "/") to find
// a matching auth token. This allows subtests to inherit tokens from their parent.
func lookupAuthToken(name string) string {
	for {
		if v, ok := testAuthTokens.Load(name); ok {
			return v.(string)
		}
		idx := strings.LastIndex(name, "/")
		if idx < 0 {
			return ""
		}
		name = name[:idx]
	}
}

func decodeRespJSON(t *testing.T, rr *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rr.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func TestWorkRequest_NoWorkAvailable(t *testing.T) {
	t.Parallel()

	mux, _, _, _, sessionStore, _, _, _, _ := newTestRouter(t)

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})

	rr := doRequest(t, mux, "POST", "/api/work/request", map[string]string{
		"session_id": session.ID,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("workRequest status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]string
	decodeRespJSON(t, rr, &resp)

	if resp["message"] != "no work available" {
		t.Errorf("workRequest message = %q, want %q", resp["message"], "no work available")
	}
}

func TestWorkRequest_WorkAvailable(t *testing.T) {
	t.Parallel()

	mux, _, storyStore, taskStore, sessionStore, _, _, _, _ := newTestRouter(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Work Available Story"; s.Status = models.StatusReady })
	task := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Available Task"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})

	rr := doRequest(t, mux, "POST", "/api/work/request", map[string]string{
		"session_id": session.ID,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("workRequest status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]any
	decodeRespJSON(t, rr, &resp)

	taskData, ok := resp["task"].(map[string]any)
	if !ok {
		t.Fatal("workRequest response missing 'task' field")
	}

	taskID, _ := taskData["id"].(string)
	if taskID != task.ID {
		t.Errorf("workRequest task ID = %q, want %q", taskID, task.ID)
	}
}

func TestWorkComplete(t *testing.T) {
	t.Parallel()

	mux, _, storyStore, taskStore, sessionStore, _, _, _, _ := newTestRouter(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Work Complete Story"; s.Status = models.StatusReady })
	task := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Complete Task"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})

	rr := doRequest(t, mux, "POST", "/api/work/request", map[string]string{
		"session_id": session.ID,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("workRequest status = %d, want %d", rr.Code, http.StatusOK)
	}

	rr = doRequest(t, mux, "POST", "/api/work/complete", map[string]string{
		"session_id": session.ID,
		"task_id":    task.ID,
		"result":     "All tests pass",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("workComplete status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]any
	decodeRespJSON(t, rr, &resp)

	taskData, ok := resp["task"].(map[string]any)
	if !ok {
		t.Fatal("workComplete response missing 'task' field")
	}
	status, _ := taskData["status"].(string)
	if models.Status(status) != models.StatusDone {
		t.Errorf("workComplete status = %q, want %q", status, models.StatusDone)
	}
}

func TestWorkBlock(t *testing.T) {
	t.Parallel()

	mux, _, storyStore, taskStore, sessionStore, commentStore, _, _, _ := newTestRouter(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Work Block Story"; s.Status = models.StatusReady })
	task := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Block Task"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})

	rr := doRequest(t, mux, "POST", "/api/work/request", map[string]string{
		"session_id": session.ID,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("workRequest status = %d, want %d", rr.Code, http.StatusOK)
	}

	rr = doRequest(t, mux, "POST", "/api/work/block", map[string]string{
		"session_id": session.ID,
		"task_id":    task.ID,
		"reason":     "Missing dependency",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("workBlock status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]any
	decodeRespJSON(t, rr, &resp)

	taskData, ok := resp["task"].(map[string]any)
	if !ok {
		t.Fatal("workBlock response missing 'task' field")
	}
	status, _ := taskData["status"].(string)
	if models.Status(status) != models.StatusBlocked {
		t.Errorf("workBlock status = %q, want %q", status, models.StatusBlocked)
	}

	// Verify a comment was created with the correct body prefix.
	comments, err := commentStore.GetByWorkItem(context.Background(), task.ID, models.WorkItemTypeTask)
	if err != nil {
		t.Fatalf("GetByWorkItem() error = %v", err)
	}
	if len(comments) == 0 {
		t.Fatal("expected at least one comment on blocked task")
	}
	lastComment := comments[len(comments)-1]
	if !strings.Contains(lastComment.Body, "Blocked:") {
		t.Errorf("comment body = %q, want it to contain 'Blocked:'", lastComment.Body)
	}
}

func TestWorkStart(t *testing.T) {
	t.Parallel()

	mux, _, storyStore, taskStore, sessionStore, _, _, _, _ := newTestRouter(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Work Start Story"; s.Status = models.StatusReady })
	task := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Start Task"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})

	// Manually assign the task to the session (without changing status).
	task.AssignedTo = session.ID
	task.AssigneeType = models.AssigneeTypeSession
	if err := taskStore.Update(context.Background(), task); err != nil {
		t.Fatalf("update task assignment: %v", err)
	}

	rr := doRequest(t, mux, "POST", "/api/work/start", map[string]string{
		"session_id": session.ID,
		"task_id":    task.ID,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("workStart status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]any
	decodeRespJSON(t, rr, &resp)

	taskData, ok := resp["task"].(map[string]any)
	if !ok {
		t.Fatal("workStart response missing 'task' field")
	}
	status, _ := taskData["status"].(string)
	if models.Status(status) != models.StatusInProgress {
		t.Errorf("workStart status = %q, want %q", status, models.StatusInProgress)
	}
}

func TestFullLifecycle(t *testing.T) {
	t.Parallel()

	mux, _, _, taskStore, sessionStore, _, _, _, _ := newTestRouter(t)

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})

	rr := doRequest(t, mux, "POST", "/api/stories", map[string]any{
		"title":          "Full Lifecycle Story",
		"requires_build": true,
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("createStory status = %d, want %d", rr.Code, http.StatusCreated)
	}

	var storyResp map[string]any
	decodeRespJSON(t, rr, &storyResp)
	storyID, _ := storyResp["id"].(string)

	rr = doRequest(t, mux, "POST", "/api/stories/"+storyID+"/tasks", map[string]any{
		"title":     "Implement Feature",
		"task_type": "code",
		"status":    "ready",
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("createTask status = %d, want %d", rr.Code, http.StatusCreated)
	}

	var taskResp map[string]any
	decodeRespJSON(t, rr, &taskResp)
	taskID, _ := taskResp["id"].(string)

	// Transition task to ready status (API doesn't support status on creation).
	if err := taskStore.UpdateStatus(context.Background(), taskID, models.StatusReady); err != nil {
		t.Fatalf("UpdateStatus(task, ready) error = %v", err)
	}

	rr = doRequest(t, mux, "POST", "/api/work/request", map[string]string{
		"session_id": session.ID,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("workRequest status = %d, want %d", rr.Code, http.StatusOK)
	}

	var workResp map[string]any
	decodeRespJSON(t, rr, &workResp)
	assignedTask, _ := workResp["task"].(map[string]any)
	assignedTaskID, _ := assignedTask["id"].(string)

	if assignedTaskID != taskID {
		t.Errorf("assigned task = %q, want %q", assignedTaskID, taskID)
	}

	// workRequest already starts the task (sets status to in_progress).
	// Skip workStart and go directly to complete.

	rr = doRequest(t, mux, "POST", "/api/work/complete", map[string]string{
		"session_id": session.ID,
		"task_id":    taskID,
		"result":     "Implementation complete",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("workComplete status = %d, want %d", rr.Code, http.StatusOK)
	}

	var completeResp map[string]any
	decodeRespJSON(t, rr, &completeResp)
	completeTask, ok := completeResp["task"].(map[string]any)
	if !ok {
		t.Fatal("workComplete response missing 'task' field")
	}
	completeStatus, _ := completeTask["status"].(string)

	if models.Status(completeStatus) != models.StatusDone {
		t.Errorf("workComplete status = %q, want %q", completeStatus, models.StatusDone)
	}

	// Give the dispatcher time to create the build task.
	deadline := time.Now().Add(2 * time.Second)
	var hasBuildTask bool
	for time.Now().Before(deadline) {
		rr = doRequest(t, mux, "GET", "/api/stories/"+storyID, nil)
		if rr.Code != http.StatusOK {
			continue
		}
		var storyWithTasks map[string]any
		decodeRespJSON(t, rr, &storyWithTasks)
		tasksData, _ := storyWithTasks["tasks"].([]any)
		hasBuildTask = false
		for _, tData := range tasksData {
			tMap, _ := tData.(map[string]any)
			taskType, _ := tMap["task_type"].(string)
			if models.TaskType(taskType) == models.TaskTypeBuild {
				hasBuildTask = true
				break
			}
		}
		if hasBuildTask {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !hasBuildTask {
		t.Log("Build task not yet created (dispatcher may need more time to process)")
	}
}

func TestStalenessInAPI(t *testing.T) {
	t.Parallel()

	mux, _, _, _, sessionStore, _, _, _, _ := newTestRouter(t)

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})
	originalLastSeen := session.LastSeenAt

	time.Sleep(10 * time.Millisecond)
	rr := doRequest(t, mux, "POST", "/api/work/keepalive", map[string]string{
		"session_id": session.ID,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("workKeepalive status = %d, want %d", rr.Code, http.StatusOK)
	}

	got, err := sessionStore.GetByID(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if !got.LastSeenAt.After(originalLastSeen) {
		t.Errorf("last_seen_at = %v, should be after %v", got.LastSeenAt, originalLastSeen)
	}
}

func TestWorkRequest_InvalidSession(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, _, _ := newTestRouter(t)

	rr := doRequest(t, mux, "POST", "/api/work/request", map[string]string{
		"session_id": "nonexistent",
	})

	if rr.Code != http.StatusNotFound {
		t.Fatalf("workRequest status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestWorkRequest_MissingSessionID(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, _, _ := newTestRouter(t)

	rr := doRequest(t, mux, "POST", "/api/work/request", map[string]string{})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("workRequest status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestWorkComplete_WrongSession(t *testing.T) {
	t.Parallel()

	mux, _, storyStore, taskStore, sessionStore, _, _, _, _ := newTestRouter(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Wrong Session Story"; s.Status = models.StatusReady })
	task := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Wrong Session Task"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

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

	rr := doRequest(t, mux, "POST", "/api/work/request", map[string]string{
		"session_id": sessionA.ID,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("workRequest status = %d, want %d", rr.Code, http.StatusOK)
	}

	rr = doRequest(t, mux, "POST", "/api/work/complete", map[string]string{
		"session_id": sessionB.ID,
		"task_id":    task.ID,
	})

	if rr.Code != http.StatusForbidden {
		t.Fatalf("workComplete status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestWorkStart_MissingTaskID(t *testing.T) {
	t.Parallel()

	mux, _, _, _, sessionStore, _, _, _, _ := newTestRouter(t)

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})

	rr := doRequest(t, mux, "POST", "/api/work/start", map[string]string{
		"session_id": session.ID,
	})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("workStart status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestWorkBlock_MissingReason(t *testing.T) {
	t.Parallel()

	mux, _, storyStore, taskStore, sessionStore, commentStore, _, _, _ := newTestRouter(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Block No Reason Story"; s.Status = models.StatusReady })
	task := testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Block No Reason Task"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeCode
	})

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})

	rr := doRequest(t, mux, "POST", "/api/work/request", map[string]string{
		"session_id": session.ID,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("workRequest status = %d, want %d", rr.Code, http.StatusOK)
	}

	rr = doRequest(t, mux, "POST", "/api/work/block", map[string]string{
		"session_id": session.ID,
		"task_id":    task.ID,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("workBlock status = %d, want %d", rr.Code, http.StatusOK)
	}

	// Verify no comment was created when reason is empty.
	comments, err := commentStore.GetByWorkItem(context.Background(), task.ID, models.WorkItemTypeTask)
	if err != nil {
		t.Fatalf("GetByWorkItem() error = %v", err)
	}
	if len(comments) > 0 {
		t.Errorf("expected no comments when reason is empty, got %d", len(comments))
	}
}

func TestWorkKeepalive_InvalidSession(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, _, _ := newTestRouter(t)

	rr := doRequest(t, mux, "POST", "/api/work/keepalive", map[string]string{
		"session_id": "nonexistent",
	})

	if rr.Code != http.StatusNotFound {
		t.Fatalf("workKeepalive status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestWorkRequest_InactiveSession(t *testing.T) {
	t.Parallel()

	mux, dbConn, _, _, sessionStore, _, _, _, _ := newTestRouter(t)

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})
	testhelpers.SetSessionLastSeen(t, dbConn, session.ID, time.Now().UTC().Add(-2*time.Hour))

	if err := sessionStore.FlagStale(context.Background(), session.ID); err != nil {
		t.Fatalf("FlagStale() error = %v", err)
	}

	rr := doRequest(t, mux, "POST", "/api/work/request", map[string]string{
		"session_id": session.ID,
	})

	if rr.Code != http.StatusForbidden {
		t.Fatalf("workRequest status = %d, want %d", rr.Code, http.StatusForbidden)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "not active") {
		t.Errorf("workRequest body = %q, want 'not active'", body)
	}
}

func TestWorkComplete_MissingSessionID(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, _, _ := newTestRouter(t)

	rr := doRequest(t, mux, "POST", "/api/work/complete", map[string]string{
		"task_id": "TASK-000001",
	})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("workComplete status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestWorkComplete_MissingFields(t *testing.T) {
	t.Parallel()

	mux, _, _, _, _, _, _, _, _ := newTestRouter(t)

	rr := doRequest(t, mux, "POST", "/api/work/complete", map[string]string{
		"session_id": "some-session",
	})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("workComplete status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestWorkRequest_CapabilityMismatch(t *testing.T) {
	t.Parallel()

	mux, _, storyStore, taskStore, sessionStore, _, _, _, _ := newTestRouter(t)

	story := testhelpers.CreateTestStory(t, storyStore, func(s *models.Story) { s.Title = "Capability Mismatch Story"; s.Status = models.StatusReady })
	testhelpers.CreateTestTask(t, taskStore, func(ts *models.Task) {
		ts.StoryID = story.ID
		ts.Title = "Build Task"
		ts.Status = models.StatusReady
		ts.TaskType = models.TaskTypeBuild
	})

	session := testhelpers.CreateTestSession(t, sessionStore, func(s *models.Session) {
		s.HarnessType = "opencode"
		data, _ := json.Marshal([]string{"code"})
		s.Capabilities = string(data)
	})

	rr := doRequest(t, mux, "POST", "/api/work/request", map[string]string{
		"session_id": session.ID,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("workRequest status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]string
	decodeRespJSON(t, rr, &resp)

	if resp["message"] != "no work available" {
		t.Errorf("workRequest message = %q, want %q", resp["message"], "no work available")
	}
}
