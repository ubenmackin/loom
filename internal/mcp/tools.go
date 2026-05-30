package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/ubenmackin/loom/internal/dispatcher"
	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
)

// registerTools registers all 14 MCP tools with their handlers and schemas.
func (s *Server) registerTools() {
	toolList := []struct {
		name        string
		description string
		schema      map[string]any
		handler     ToolHandler
	}{
		{
			name:        "register_session",
			description: "Register an agent session with the Loom board. Returns a session ID for use in subsequent calls.",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"harness_type": map[string]any{"type": "string", "description": "Type of agent harness (e.g. opencode, claude, codex)"},
					"capabilities": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "List of capabilities this session supports (e.g. code, build, review)",
					},
					"metadata": map[string]any{
						"type":        "object",
						"description": "Optional metadata about the session",
					},
				},
				"required": []string{"harness_type"},
			},
			handler: s.handleRegisterSession,
		},
		{
			name:        "request_work",
			description: "Request the next available task assignment. Finds the best Ready task matching session capabilities and returns it with instructions.",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"session_id": map[string]any{"type": "string", "description": "The session ID requesting work"},
				},
				"required": []string{"session_id"},
			},
			handler: s.handleRequestWork,
		},
		{
			name:        "start_work",
			description: "Begin work on an assigned task. Transitions the task to in_progress.",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"session_id": map[string]any{"type": "string", "description": "The session ID starting work"},
					"task_id":    map[string]any{"type": "string", "description": "The task ID to start working on"},
				},
				"required": []string{"session_id", "task_id"},
			},
			handler: s.handleStartWork,
		},
		{
			name:        "complete_work",
			description: "Mark a task as Done with a result. Adds a result comment and fires a status changed event.",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"session_id": map[string]any{"type": "string", "description": "The session ID completing work"},
					"task_id":    map[string]any{"type": "string", "description": "The task ID to mark as done"},
					"result":     map[string]any{"type": "string", "description": "The result or summary of the completed work"},
				},
				"required": []string{"session_id", "task_id", "result"},
			},
			handler: s.handleCompleteWork,
		},
		{
			name:        "report_blocked",
			description: "Report that a task is blocked with a reason. Transitions the task to blocked and adds a comment.",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"session_id": map[string]any{"type": "string", "description": "The session ID reporting the block"},
					"task_id":    map[string]any{"type": "string", "description": "The task ID that is blocked"},
					"reason":     map[string]any{"type": "string", "description": "The reason the task is blocked"},
				},
				"required": []string{"session_id", "task_id", "reason"},
			},
			handler: s.handleReportBlocked,
		},
		{
			name:        "keep_alive",
			description: "Send a keepalive to update the session's last-seen timestamp.",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"session_id": map[string]any{"type": "string", "description": "The session ID to keep alive"},
				},
				"required": []string{"session_id"},
			},
			handler: s.handleKeepAlive,
		},
		{
			name:        "get_task",
			description: "Get full details and instructions for a specific task.",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "string", "description": "The task ID to retrieve"},
				},
				"required": []string{"task_id"},
			},
			handler: s.handleGetTask,
		},
		{
			name:        "get_blockers",
			description: "Get the unmet dependencies (blocking tasks) for a task.",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id": map[string]any{"type": "string", "description": "The task ID to check blockers for"},
				},
				"required": []string{"task_id"},
			},
			handler: s.handleGetBlockers,
		},
		{
			name:        "add_comment",
			description: "Add a comment to a work item (story or task).",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"work_item_id":   map[string]any{"type": "string", "description": "The work item ID to comment on"},
					"work_item_type": map[string]any{"type": "string", "description": "The type of work item (story or task)", "enum": []string{"story", "task"}},
					"author_id":      map[string]any{"type": "string", "description": "The ID of the author (session ID or user ID)"},
					"author_type":    map[string]any{"type": "string", "description": "The type of author (session or human)", "enum": []string{"session", "human"}},
					"body":           map[string]any{"type": "string", "description": "The comment body"},
				},
				"required": []string{"work_item_id", "work_item_type", "author_id", "author_type", "body"},
			},
			handler: s.handleAddComment,
		},
		{
			name:        "get_comments",
			description: "Get all comments for a work item (story or task).",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"work_item_id":   map[string]any{"type": "string", "description": "The work item ID"},
					"work_item_type": map[string]any{"type": "string", "description": "The type of work item (story or task)", "enum": []string{"story", "task"}},
				},
				"required": []string{"work_item_id", "work_item_type"},
			},
			handler: s.handleGetComments,
		},
		{
			name:        "get_unread_comments",
			description: "Get unread comments for the session's assigned tasks.",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"session_id": map[string]any{"type": "string", "description": "The session ID to check unread comments for"},
				},
				"required": []string{"session_id"},
			},
			handler: s.handleGetUnreadComments,
		},
		{
			name:        "get_my_tasks",
			description: "Get all tasks assigned to the calling session.",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"session_id": map[string]any{"type": "string", "description": "The session ID to list tasks for"},
				},
				"required": []string{"session_id"},
			},
			handler: s.handleGetMyTasks,
		},
		{
			name:        "add_dependency",
			description: "Add a finish-to-start dependency between two tasks.",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id":            map[string]any{"type": "string", "description": "The task that will depend on the other"},
					"depends_on_task_id": map[string]any{"type": "string", "description": "The task that must finish first"},
				},
				"required": []string{"task_id", "depends_on_task_id"},
			},
			handler: s.handleAddDependency,
		},
		{
			name:        "create_task",
			description: "Create a new task under a story. Optionally specify task IDs this task depends on.",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"story_id":    map[string]any{"type": "string", "description": "The parent story ID"},
					"title":       map[string]any{"type": "string", "description": "The task title"},
					"description": map[string]any{"type": "string", "description": "The task description"},
					"task_type":   map[string]any{"type": "string", "description": "The task type (code, build, review)", "enum": []string{"code", "build", "review"}},
					"depends_on": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Optional list of task IDs this task depends on",
					},
				},
				"required": []string{"story_id", "title"},
			},
			handler: s.handleCreateTask,
		},
	}

	for _, t := range toolList {
		s.tools[t.name] = t.handler
		s.toolDefs = append(s.toolDefs, ToolDef{
			Name:        t.name,
			Description: t.description,
			InputSchema: t.schema,
		})
	}
}

// handleRegisterSession registers a new agent session.
func (s *Server) handleRegisterSession(ctx context.Context, params map[string]any) (*ToolResult, error) {
	harnessType, err := getRequiredString(params, "harness_type")
	if err != nil {
		return nil, err
	}

	session := &models.Session{
		ID:          uuid.New().String(),
		HarnessType: harnessType,
		Status:      models.SessionStatusActive,
	}

	// Marshal capabilities to JSON if provided.
	if caps, ok := params["capabilities"]; ok {
		capsJSON, err := json.Marshal(caps)
		if err != nil {
			return nil, fmt.Errorf("marshal capabilities: %w", err)
		}
		session.Capabilities = string(capsJSON)
	}

	// Marshal metadata to JSON if provided.
	if meta, ok := params["metadata"]; ok {
		metaJSON, err := json.Marshal(meta)
		if err != nil {
			return nil, fmt.Errorf("marshal metadata: %w", err)
		}
		session.Metadata = string(metaJSON)
	}

	if err := s.sessions.Register(ctx, session); err != nil {
		return nil, fmt.Errorf("register session: %w", err)
	}

	// Auto-set the server's session ID if not already set.
	s.setSessionID.Do(func() {
		s.sessionID = session.ID
	})

	// Submit a session_registered event.
	s.submitEvent(ctx, dispatcher.Event{
		Type:      dispatcher.EventSessionRegistered,
		SessionID: session.ID,
	})

	return jsonTextResult(map[string]string{
		"session_id": session.ID,
		"status":     string(session.Status),
	})
}

// handleRequestWork finds the best available Ready task for the session
// and returns it with assembled instructions.
func (s *Server) handleRequestWork(ctx context.Context, params map[string]any) (*ToolResult, error) {
	sessionID, err := getRequiredString(params, "session_id")
	if err != nil {
		return nil, err
	}

	// Update last seen.
	if err := s.sessions.UpdateLastSeen(ctx, sessionID); err != nil {
		return nil, fmt.Errorf("update last seen: %w", err)
	}

	// Submit a WorkRequested event.
	s.submitEvent(ctx, dispatcher.Event{
		Type:      dispatcher.EventWorkRequested,
		SessionID: sessionID,
	})

	// Find Ready tasks that are unassigned or assigned to this session.
	readyTasks, err := s.tasks.List(ctx, store.TaskFilter{Status: models.StatusReady})
	if err != nil {
		return nil, fmt.Errorf("list ready tasks: %w", err)
	}

	// Prefer tasks already assigned to this session, then unassigned.
	var bestTask *models.Task
	for _, t := range readyTasks {
		if t.AssignedTo == sessionID {
			bestTask = t
			break
		}
		if t.AssignedTo == "" && bestTask == nil {
			bestTask = t
		}
	}

	if bestTask == nil {
		return textResult("No available tasks found. Check back later or ask a human for assignments."), nil
	}

	// Assign the task to this session if unassigned.
	if bestTask.AssignedTo == "" {
		bestTask.AssignedTo = sessionID
		bestTask.AssigneeType = models.AssigneeTypeSession
		if err := s.tasks.Update(ctx, bestTask); err != nil {
			return nil, fmt.Errorf("assign task %q: %w", bestTask.ID, err)
		}
	}

	// Assemble prompt with task details and instructions.
	result := map[string]any{
		"task_id":      bestTask.ID,
		"story_id":     bestTask.StoryID,
		"title":        bestTask.Title,
		"description":  bestTask.Description,
		"task_type":    bestTask.TaskType,
		"status":       bestTask.Status,
		"instructions": bestTask.Instructions,
	}

	// Try to get a prompt template for this task type.
	if bestTask.TaskType != "" && s.templates != nil {
		tmpl, err := s.templates.GetByTaskType(ctx, bestTask.TaskType)
		if err == nil && tmpl != nil {
			result["prompt_template"] = tmpl.Template
		}
	}

	// Check for blockers.
	blockers, err := s.tasks.GetBlockers(ctx, bestTask.ID)
	if err == nil && len(blockers) > 0 {
		var blockerIDs []string
		for _, b := range blockers {
			blockerIDs = append(blockerIDs, b.ID)
		}
		result["blockers"] = blockerIDs
	}

	return jsonTextResult(result)
}

// handleStartWork transitions a task to in_progress.
func (s *Server) handleStartWork(ctx context.Context, params map[string]any) (*ToolResult, error) {
	sessionID, err := getRequiredString(params, "session_id")
	if err != nil {
		return nil, err
	}
	taskID, err := getRequiredString(params, "task_id")
	if err != nil {
		return nil, err
	}

	// Verify the task exists and is assigned to this session.
	task, err := s.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task %q: %w", taskID, err)
	}
	if task.AssignedTo != sessionID {
		return nil, fmt.Errorf("task %q is not assigned to session %q", taskID, sessionID)
	}

	// Update task status.
	if err := s.tasks.UpdateStatus(ctx, taskID, models.StatusInProgress); err != nil {
		return nil, fmt.Errorf("start work on task %q: %w", taskID, err)
	}

	// Log activity.
	if s.activities != nil {
		if err := s.activities.Log(ctx, &models.ActivityLogEntry{
			WorkItemID:   taskID,
			WorkItemType: models.WorkItemTypeTask,
			Action:       "started",
			Details:      fmt.Sprintf(`{"session_id":%q}`, sessionID),
		}); err != nil {
			slog.Error("mcp: failed to log activity", "error", err, "task_id", taskID)
		}
	}

	// Update session last seen.
	if err := s.sessions.UpdateLastSeen(ctx, sessionID); err != nil {
		slog.Error("mcp: failed to update session last seen", "error", err, "session_id", sessionID)
	}

	return textResult(fmt.Sprintf("Task %s is now in_progress. Session %s has started work.", taskID, sessionID)), nil
}

// handleCompleteWork marks a task as done and adds a result comment.
func (s *Server) handleCompleteWork(ctx context.Context, params map[string]any) (*ToolResult, error) {
	sessionID, err := getRequiredString(params, "session_id")
	if err != nil {
		return nil, err
	}
	taskID, err := getRequiredString(params, "task_id")
	if err != nil {
		return nil, err
	}
	resultStr, err := getRequiredString(params, "result")
	if err != nil {
		return nil, err
	}

	// Verify the task exists and is assigned to this session.
	task, err := s.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task %q: %w", taskID, err)
	}
	if task.AssignedTo != sessionID {
		return nil, fmt.Errorf("task %q is not assigned to session %q", taskID, sessionID)
	}

	// Update task status.
	if err := s.tasks.UpdateStatus(ctx, taskID, models.StatusDone); err != nil {
		return nil, fmt.Errorf("complete task %q: %w", taskID, err)
	}

	// Add result comment.
	if s.comments != nil {
		if err := s.comments.Create(ctx, &models.Comment{
			WorkItemID:   taskID,
			WorkItemType: models.WorkItemTypeTask,
			AuthorID:     sessionID,
			AuthorType:   models.AuthorType(models.AssigneeTypeSession),
			Body:         fmt.Sprintf("Work completed. Result: %s", resultStr),
		}); err != nil {
			slog.Error("mcp: failed to create comment", "error", err, "task_id", taskID)
		}
	}

	// Submit event.
	s.submitEvent(ctx, dispatcher.Event{
		Type:   dispatcher.EventTaskCompleted,
		TaskID: taskID,
	})

	return textResult(fmt.Sprintf("Task %s is now done. Result has been recorded.", taskID)), nil
}

// handleReportBlocked transitions a task to blocked and adds a reason comment.
func (s *Server) handleReportBlocked(ctx context.Context, params map[string]any) (*ToolResult, error) {
	sessionID, err := getRequiredString(params, "session_id")
	if err != nil {
		return nil, err
	}
	taskID, err := getRequiredString(params, "task_id")
	if err != nil {
		return nil, err
	}
	reason, err := getRequiredString(params, "reason")
	if err != nil {
		return nil, err
	}

	// Verify the task exists and is assigned to this session.
	task, err := s.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task %q: %w", taskID, err)
	}
	if task.AssignedTo != sessionID {
		return nil, fmt.Errorf("task %q is not assigned to session %q", taskID, sessionID)
	}

	// Update task status.
	if err := s.tasks.UpdateStatus(ctx, taskID, models.StatusBlocked); err != nil {
		return nil, fmt.Errorf("block task %q: %w", taskID, err)
	}

	// Add reason comment.
	if s.comments != nil {
		if err := s.comments.Create(ctx, &models.Comment{
			WorkItemID:   taskID,
			WorkItemType: models.WorkItemTypeTask,
			AuthorID:     sessionID,
			AuthorType:   models.AuthorType(models.AssigneeTypeSession),
			Body:         fmt.Sprintf("Blocked: %s", reason),
		}); err != nil {
			slog.Error("mcp: failed to create comment", "error", err, "task_id", taskID)
		}
	}

	// Submit event.
	s.submitEvent(ctx, dispatcher.Event{
		Type:      dispatcher.EventTaskBlocked,
		TaskID:    taskID,
		SessionID: sessionID,
	})

	return textResult(fmt.Sprintf("Task %s is now blocked. Reason: %s", taskID, reason)), nil
}

// handleKeepAlive updates the session's last-seen timestamp.
func (s *Server) handleKeepAlive(ctx context.Context, params map[string]any) (*ToolResult, error) {
	sessionID, err := getRequiredString(params, "session_id")
	if err != nil {
		return nil, err
	}

	if err := s.sessions.UpdateLastSeen(ctx, sessionID); err != nil {
		return nil, fmt.Errorf("keep alive for session %q: %w", sessionID, err)
	}

	return textResult(fmt.Sprintf("Session %s keepalive acknowledged.", sessionID)), nil
}

// handleGetTask returns the details of a specific task.
func (s *Server) handleGetTask(ctx context.Context, params map[string]any) (*ToolResult, error) {
	taskID, err := getRequiredString(params, "task_id")
	if err != nil {
		return nil, err
	}

	task, err := s.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task %q: %w", taskID, err)
	}

	return jsonTextResult(task)
}

// handleGetBlockers returns the unmet dependencies for a task.
func (s *Server) handleGetBlockers(ctx context.Context, params map[string]any) (*ToolResult, error) {
	taskID, err := getRequiredString(params, "task_id")
	if err != nil {
		return nil, err
	}

	blockers, err := s.tasks.GetBlockers(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get blockers for task %q: %w", taskID, err)
	}

	if len(blockers) == 0 {
		return textResult("No blockers found."), nil
	}

	return jsonTextResult(blockers)
}

// handleAddComment adds a comment to a work item.
func (s *Server) handleAddComment(ctx context.Context, params map[string]any) (*ToolResult, error) {
	workItemID, err := getRequiredString(params, "work_item_id")
	if err != nil {
		return nil, err
	}
	workItemType, err := getRequiredString(params, "work_item_type")
	if err != nil {
		return nil, err
	}
	authorID, err := getRequiredString(params, "author_id")
	if err != nil {
		return nil, err
	}
	authorType, err := getRequiredString(params, "author_type")
	if err != nil {
		return nil, err
	}
	body, err := getRequiredString(params, "body")
	if err != nil {
		return nil, err
	}

	comment := &models.Comment{
		WorkItemID:   workItemID,
		WorkItemType: models.WorkItemType(workItemType),
		AuthorID:     authorID,
		AuthorType:   models.AuthorType(authorType),
		Body:         body,
	}

	if err := s.comments.Create(ctx, comment); err != nil {
		return nil, fmt.Errorf("create comment: %w", err)
	}

	return jsonTextResult(comment)
}

// handleGetComments returns all comments for a work item.
func (s *Server) handleGetComments(ctx context.Context, params map[string]any) (*ToolResult, error) {
	workItemID, err := getRequiredString(params, "work_item_id")
	if err != nil {
		return nil, err
	}
	workItemType, err := getRequiredString(params, "work_item_type")
	if err != nil {
		return nil, err
	}

	comments, err := s.comments.GetByWorkItem(ctx, workItemID, models.WorkItemType(workItemType))
	if err != nil {
		return nil, fmt.Errorf("get comments for %s %q: %w", workItemType, workItemID, err)
	}

	if len(comments) == 0 {
		return textResult("No comments found."), nil
	}

	return jsonTextResult(comments)
}

// handleGetUnreadComments returns unread comments for the session's assigned tasks.
func (s *Server) handleGetUnreadComments(ctx context.Context, params map[string]any) (*ToolResult, error) {
	sessionID, err := getRequiredString(params, "session_id")
	if err != nil {
		return nil, err
	}

	comments, err := s.comments.GetUnreadForSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get unread comments for session %q: %w", sessionID, err)
	}

	if len(comments) == 0 {
		return textResult("No unread comments."), nil
	}

	return jsonTextResult(comments)
}

// handleGetMyTasks returns all tasks assigned to the calling session.
func (s *Server) handleGetMyTasks(ctx context.Context, params map[string]any) (*ToolResult, error) {
	sessionID, err := getRequiredString(params, "session_id")
	if err != nil {
		return nil, err
	}

	tasks, err := s.sessions.GetTasksForSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get tasks for session %q: %w", sessionID, err)
	}

	if len(tasks) == 0 {
		return textResult("No tasks assigned to this session."), nil
	}

	return jsonTextResult(tasks)
}

// handleAddDependency adds a finish-to-start dependency between two tasks.
func (s *Server) handleAddDependency(ctx context.Context, params map[string]any) (*ToolResult, error) {
	taskID, err := getRequiredString(params, "task_id")
	if err != nil {
		return nil, err
	}
	dependsOnTaskID, err := getRequiredString(params, "depends_on_task_id")
	if err != nil {
		return nil, err
	}

	if err := s.tasks.AddDependency(ctx, taskID, dependsOnTaskID); err != nil {
		return nil, fmt.Errorf("add dependency %q -> %q: %w", taskID, dependsOnTaskID, err)
	}

	// Submit event.
	s.submitEvent(ctx, dispatcher.Event{
		Type:   dispatcher.EventDependencyAdded,
		TaskID: taskID,
	})

	return textResult(fmt.Sprintf("Dependency added: %s depends on %s", taskID, dependsOnTaskID)), nil
}

// handleCreateTask creates a new task under a story.
func (s *Server) handleCreateTask(ctx context.Context, params map[string]any) (*ToolResult, error) {
	storyID, err := getRequiredString(params, "story_id")
	if err != nil {
		return nil, err
	}
	title, err := getRequiredString(params, "title")
	if err != nil {
		return nil, err
	}

	description := getOptionalString(params, "description")
	taskType := getOptionalString(params, "task_type")

	task := &models.Task{
		StoryID:     storyID,
		Title:       title,
		Description: description,
		TaskType:    models.TaskType(taskType),
		Status:      models.StatusNew,
	}

	if err := s.tasks.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	dependsOn := getOptionalStringSlice(params, "depends_on")
	for _, depID := range dependsOn {
		if err := s.tasks.AddDependency(ctx, task.ID, depID); err != nil {
			return nil, fmt.Errorf("create task: failed to add dependency on %q: %w", depID, err)
		}
	}

	return jsonTextResult(task)
}
