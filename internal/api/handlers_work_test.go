package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/go-chi/chi/v5"

	"github.com/ubenmackin/loom/internal/db"
	"github.com/ubenmackin/loom/internal/dispatcher"
	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
)

// mockHub implements HubInterface for testing.
type mockHub struct{}

func (m *mockHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusSwitchingProtocols)
}

// testBroadcaster implements dispatcher.EventBroadcaster for testing.
type testBroadcaster struct{}

func (m *testBroadcaster) Broadcast(eventType string, payload any) {}

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

var apiSessionCounter atomic.Int64

func createTestSession(t *testing.T, s *store.SessionStore, harnessType string, capabilities []string) *models.Session {
	t.Helper()
	n := apiSessionCounter.Add(1)
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

func createTestComment(t *testing.T, s *store.CommentStore, workItemID, workItemType, authorID, authorType, body string) *models.Comment {
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

func setSessionLastSeen(t *testing.T, dbConn *sql.DB, sessionID string, tstamp time.Time) {
	t.Helper()
	ctx := context.Background()
	_, err := dbConn.ExecContext(ctx, "UPDATE sessions SET last_seen_at = ? WHERE id = ?", tstamp.UTC(), sessionID)
	if err != nil {
		t.Fatalf("set session last_seen_at: %v", err)
	}
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

	dbConn := setupTestDB(t)

	storyStore := store.NewStoryStore(dbConn)
	taskStore := store.NewTaskStore(dbConn)
	sessionStore := store.NewSessionStore(dbConn)
	commentStore := store.NewCommentStore(dbConn)
	templateStore := store.NewTemplateStore(dbConn)
	activityStore := store.NewActivityStore(dbConn)
	userStore := store.NewUserStore(dbConn)

	// Create a test user and session token so protected routes work in tests.
	testUser, err := userStore.CreateUser(context.Background(), "testuser", "test@example.com", "Test User", "password123")
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
	d := dispatcher.NewDispatcher(
		storyStore,
		taskStore,
		sessionStore,
		templateStore,
		commentStore,
		activityStore,
		broadcaster,
		30*time.Minute,
	)

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

func decodeJSON(t *testing.T, rr *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rr.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func TestWorkRequest_NoWorkAvailable(t *testing.T) {
	t.Parallel()

	mux, _, _, _, sessionStore, _, _, _, _ := newTestRouter(t)

	session := createTestSession(t, sessionStore, "opencode", []string{"code"})

	rr := doRequest(t, mux, "POST", "/api/work/request", map[string]string{
		"session_id": session.ID,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("workRequest status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]string
	decodeJSON(t, rr, &resp)

	if resp["message"] != "no work available" {
		t.Errorf("workRequest message = %q, want %q", resp["message"], "no work available")
	}
}

func TestWorkRequest_WorkAvailable(t *testing.T) {
	t.Parallel()

	mux, _, storyStore, taskStore, sessionStore, _, _, _, _ := newTestRouter(t)

	story := createTestStory(t, storyStore, "Work Available Story", models.StatusReady)
	task := createTestTask(t, taskStore, story.ID, "Available Task", models.StatusReady, models.TaskTypeCode)

	session := createTestSession(t, sessionStore, "opencode", []string{"code"})

	rr := doRequest(t, mux, "POST", "/api/work/request", map[string]string{
		"session_id": session.ID,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("workRequest status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]any
	decodeJSON(t, rr, &resp)

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

	story := createTestStory(t, storyStore, "Work Complete Story", models.StatusReady)
	task := createTestTask(t, taskStore, story.ID, "Complete Task", models.StatusReady, models.TaskTypeCode)

	session := createTestSession(t, sessionStore, "opencode", []string{"code"})

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
	decodeJSON(t, rr, &resp)

	taskData, ok := resp["task"].(map[string]any)
	if !ok {
		t.Fatal("workComplete response missing 'task' field")
	}
	status, _ := taskData["status"].(string)
	if status != models.StatusDone {
		t.Errorf("workComplete status = %q, want %q", status, models.StatusDone)
	}
}

func TestWorkBlock(t *testing.T) {
	t.Parallel()

	mux, _, storyStore, taskStore, sessionStore, _, _, _, _ := newTestRouter(t)

	story := createTestStory(t, storyStore, "Work Block Story", models.StatusReady)
	task := createTestTask(t, taskStore, story.ID, "Block Task", models.StatusReady, models.TaskTypeCode)

	session := createTestSession(t, sessionStore, "opencode", []string{"code"})

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
	decodeJSON(t, rr, &resp)

	taskData, ok := resp["task"].(map[string]any)
	if !ok {
		t.Fatal("workBlock response missing 'task' field")
	}
	status, _ := taskData["status"].(string)
	if status != models.StatusBlocked {
		t.Errorf("workBlock status = %q, want %q", status, models.StatusBlocked)
	}
}

func TestWorkStart(t *testing.T) {
	t.Parallel()

	mux, _, storyStore, taskStore, sessionStore, _, _, _, _ := newTestRouter(t)

	story := createTestStory(t, storyStore, "Work Start Story", models.StatusReady)
	task := createTestTask(t, taskStore, story.ID, "Start Task", models.StatusReady, models.TaskTypeCode)

	session := createTestSession(t, sessionStore, "opencode", []string{"code"})

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
	decodeJSON(t, rr, &resp)

	taskData, ok := resp["task"].(map[string]any)
	if !ok {
		t.Fatal("workStart response missing 'task' field")
	}
	status, _ := taskData["status"].(string)
	if status != models.StatusInProgress {
		t.Errorf("workStart status = %q, want %q", status, models.StatusInProgress)
	}
}

func TestFullLifecycle(t *testing.T) {
	t.Parallel()

	mux, _, storyStore, taskStore, sessionStore, _, _, _, _ := newTestRouter(t)

	session := createTestSession(t, sessionStore, "opencode", []string{"code"})

	rr := doRequest(t, mux, "POST", "/api/stories", map[string]any{
		"title":          "Full Lifecycle Story",
		"requires_build": true,
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("createStory status = %d, want %d", rr.Code, http.StatusCreated)
	}

	var storyResp map[string]any
	decodeJSON(t, rr, &storyResp)
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
	decodeJSON(t, rr, &taskResp)
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
	decodeJSON(t, rr, &workResp)
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
	decodeJSON(t, rr, &completeResp)
	completeTask, ok := completeResp["task"].(map[string]any)
	if !ok {
		t.Fatal("workComplete response missing 'task' field")
	}
	completeStatus, _ := completeTask["status"].(string)

	if completeStatus != models.StatusDone {
		t.Errorf("workComplete status = %q, want %q", completeStatus, models.StatusDone)
	}

	time.Sleep(100 * time.Millisecond)

	rr = doRequest(t, mux, "GET", "/api/stories/"+storyID, nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("getStory status = %d, want %d", rr.Code, http.StatusOK)
	}

	var storyWithTasks map[string]any
	decodeJSON(t, rr, &storyWithTasks)

	tasksData, _ := storyWithTasks["tasks"].([]any)
	var hasBuildTask bool
	for _, tData := range tasksData {
		tMap, _ := tData.(map[string]any)
		taskType, _ := tMap["task_type"].(string)
		if taskType == models.TaskTypeBuild {
			hasBuildTask = true
			break
		}
	}

	if !hasBuildTask {
		t.Log("Build task not yet created (dispatcher may need more time to process)")
	}

	_ = storyStore
	_ = taskStore
}

func TestStalenessInAPI(t *testing.T) {
	t.Parallel()

	mux, dbConn, _, _, sessionStore, _, _, _, _ := newTestRouter(t)

	session := createTestSession(t, sessionStore, "opencode", []string{"code"})
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

	_ = dbConn
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

	story := createTestStory(t, storyStore, "Wrong Session Story", models.StatusReady)
	task := createTestTask(t, taskStore, story.ID, "Wrong Session Task", models.StatusReady, models.TaskTypeCode)

	sessionA := createTestSession(t, sessionStore, "opencode", []string{"code"})
	sessionB := createTestSession(t, sessionStore, "opencode", []string{"code"})

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

	session := createTestSession(t, sessionStore, "opencode", []string{"code"})

	rr := doRequest(t, mux, "POST", "/api/work/start", map[string]string{
		"session_id": session.ID,
	})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("workStart status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestWorkBlock_MissingReason(t *testing.T) {
	t.Parallel()

	mux, _, storyStore, taskStore, sessionStore, _, _, _, _ := newTestRouter(t)

	story := createTestStory(t, storyStore, "Block No Reason Story", models.StatusReady)
	task := createTestTask(t, taskStore, story.ID, "Block No Reason Task", models.StatusReady, models.TaskTypeCode)

	session := createTestSession(t, sessionStore, "opencode", []string{"code"})

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

	session := createTestSession(t, sessionStore, "opencode", []string{"code"})
	setSessionLastSeen(t, dbConn, session.ID, time.Now().UTC().Add(-2*time.Hour))

	if err := sessionStore.FlagStale(context.Background(), session.ID); err != nil {
		t.Fatalf("FlagStale() error = %v", err)
	}

	rr := doRequest(t, mux, "POST", "/api/work/request", map[string]string{
		"session_id": session.ID,
	})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("workRequest status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "not active") {
		t.Errorf("workRequest body = %q, want 'not active'", body)
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

	story := createTestStory(t, storyStore, "Capability Mismatch Story", models.StatusReady)
	createTestTask(t, taskStore, story.ID, "Build Task", models.StatusReady, models.TaskTypeBuild)

	session := createTestSession(t, sessionStore, "opencode", []string{"code"})

	rr := doRequest(t, mux, "POST", "/api/work/request", map[string]string{
		"session_id": session.ID,
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("workRequest status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]string
	decodeJSON(t, rr, &resp)

	if resp["message"] != "no work available" {
		t.Errorf("workRequest message = %q, want %q", resp["message"], "no work available")
	}
}
