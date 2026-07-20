package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ubenmackin/loom/internal/dispatcher"
	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
	"github.com/ubenmackin/loom/internal/testhelpers"
)

// capturingGatewaySubmitter is a minimal dispatcher.GatewaySubmitter
// implementation that records every event the MCP server submits to it
// (via Server.gateway.SubmitEvent). It lets tools_test.go assert that
// handleCreateTask emits an EventWorkRequested carrying the supplied
// agent_type without spinning up a full gateway.
type capturingGatewaySubmitter struct {
	events []dispatcher.Event
}

func (c *capturingGatewaySubmitter) SubmitEvent(event dispatcher.Event) {
	c.events = append(c.events, event)
}

// TestCreateTask_SetsAgentType verifies the TASK-006 behavior end-to-end:
// when the create_task MCP tool is invoked with agent_type="reviewer", (1)
// the persisted task row has AgentType=="reviewer", AND (2) the
// EventWorkRequested payload emitted to the gateway carries
// agent_type:"reviewer".
//
// Mock setup: a real in-memory sqlite store is wired through
// testhelpers.SetupTestDB (which applies all migrations, including
// migration 013 that adds the agent_type column to tasks). The MCP Server
// writes the task via the real TaskStore.Create and emits the
// EventWorkRequested event to a capturingGatewaySubmitter in place of the
// real gateway bridge. The dispatcher is left nil — Server.submitEvent
// no-ops on a nil dispatcher, which is fine for this test since we assert
// on the gateway submitter capture only.
func TestCreateTask_SetsAgentType(t *testing.T) {
	dbConn := testhelpers.SetupTestDB(t)
	tasks := store.NewTaskStore(dbConn)
	stories := store.NewStoryStore(dbConn)
	sessions := store.NewSessionStore(dbConn)
	comments := store.NewCommentStore(dbConn)

	// A ready story is required so handleCreateTask accepts status="ready"
	// for the new task (otherwise it errors with "parent story is not
	// ready or in_progress") — and the EventWorkRequested event is only
	// emitted when the task is created as "ready".
	//
	// Seed the projects row referenced by s.ProjectID below BEFORE creating
	// the story: migration 006 declares
	// `stories.project_id TEXT REFERENCES projects(id) ON DELETE SET NULL`,
	// and SetupTestDB enables PRAGMA foreign_keys=ON, so inserting a story
	// with a non-empty project_id without a matching projects row fails
	// with a FOREIGN KEY constraint violation before handleCreateTask runs.
	if _, err := dbConn.ExecContext(context.Background(),
		"INSERT INTO projects (id, name) VALUES (?, ?)",
		"p-test", "Test Project"); err != nil {
		t.Fatalf("failed to seed projects row: %v", err)
	}
	story := testhelpers.CreateTestStory(t, stories, func(s *models.Story) {
		s.Status = models.StatusReady
		s.ProjectID = "p-test"
	})

	gw := &capturingGatewaySubmitter{}
	server := NewServer(stories, tasks, sessions, comments, nil, nil, nil, gw)

	params := map[string]any{
		"story_id":   story.ID,
		"title":      "Reviewer task",
		"task_type":  "review",
		"status":     "ready",
		"agent_type": "reviewer",
	}

	result, err := server.handleCreateTask(context.Background(), params)
	if err != nil {
		t.Fatalf("handleCreateTask() returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("handleCreateTask() returned nil result")
	}

	// Extract the created task ID from the JSON-serialised ToolResult.
	// handleCreateTask returns jsonTextResult(task), which produces a
	// single text ContentBlock whose ``Text`` field is the JSON encoding
	// of the *models.Task.
	if len(result.Content) == 0 {
		t.Fatal("ToolResult.Content is empty, expected one text block")
	}
	var createdTask models.Task
	if err := json.Unmarshal([]byte(result.Content[0].Text), &createdTask); err != nil {
		t.Fatalf("failed to unmarshal ToolResult content: %v", err)
	}

	// (1) The persisted task row carries AgentType == "reviewer".
	persisted, err := tasks.GetByID(context.Background(), createdTask.ID)
	if err != nil {
		t.Fatalf("tasks.GetByID() unexpected error: %v", err)
	}
	if persisted.AgentType != "reviewer" {
		t.Errorf("persisted task.AgentType = %q, want %q", persisted.AgentType, "reviewer")
	}

	// (2) The EventWorkRequested payload emitted to the gateway contains
	// agent_type:"reviewer".
	var workReq *dispatcher.Event
	for i := range gw.events {
		if gw.events[i].Type == dispatcher.EventWorkRequested {
			workReq = &gw.events[i]
			break
		}
	}
	if workReq == nil {
		t.Fatalf("no EventWorkRequested found in captured gateway events (got %d events: %v)", len(gw.events), gw.events)
	}
	payload, ok := workReq.Payload.(map[string]interface{})
	if !ok {
		t.Fatalf("EventWorkRequested.Payload type = %T, want map[string]interface{}", workReq.Payload)
	}
	got, hasKey := payload["agent_type"]
	if !hasKey {
		t.Fatalf("EventWorkRequested.Payload missing agent_type key (payload: %v)", payload)
	}
	gotStr, _ := got.(string)
	if gotStr != "reviewer" {
		t.Errorf("EventWorkRequested.Payload[agent_type] = %q, want %q", gotStr, "reviewer")
	}

	// Sanity: the work_requested event must also echo the task_id and
	// project_id so the gateway can resume routing.
	if workReq.TaskID != createdTask.ID {
		t.Errorf("EventWorkRequested.TaskID = %q, want %q", workReq.TaskID, createdTask.ID)
	}
	if projID, _ := payload["project_id"].(string); projID != story.ProjectID {
		t.Errorf("EventWorkRequested.Payload[project_id] = %q, want %q", projID, story.ProjectID)
	}
}
