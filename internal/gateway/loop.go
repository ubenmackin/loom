package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/ubenmackin/loom/internal/acp"
	"github.com/ubenmackin/loom/internal/dispatcher"
	"github.com/ubenmackin/loom/internal/models"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

// stalenessThreshold is how long a session can go without a heartbeat before
// being marked as stale. The dispatcher also has its own staleness check at
// 30 minutes — this is the gateway-level check at a shorter interval.
const stalenessThreshold = 5 * time.Minute

// stalenessCheckInterval controls how often the gateway checks for stale
// sessions.
const stalenessCheckInterval = 30 * time.Second

// ---------------------------------------------------------------------------
// Gateway event loop
// ---------------------------------------------------------------------------

// run is the main gateway event loop. It processes events from the event
// channel and performs periodic staleness checks.
func (g *Gateway) run() {
	ticker := time.NewTicker(stalenessCheckInterval)
	defer ticker.Stop()

	slog.Info("gateway: event loop started")

	for {
		select {
		case <-g.done:
			slog.Info("gateway: event loop stopped")
			return

		case event := <-g.eventCh:
			g.processEvent(context.Background(), event)

		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			g.checkStaleness(ctx)
			cancel()
		}
	}
}

// ---------------------------------------------------------------------------
// Event processing
// ---------------------------------------------------------------------------

// processEvent evaluates a dispatcher event against the rules engine and
// dispatches the resulting action.
func (g *Gateway) processEvent(ctx context.Context, event dispatcher.Event) {
	g.eventsProcessed.Add(1)

	slog.Debug("gateway: processing event",
		"event_type", event.Type,
		"task_id", event.TaskID,
		"session_id", event.SessionID)

	// Determine the agent_type — look it up from the task if this event
	// references a task, otherwise try the payload or fall back to "*".
	agentType := g.resolveAgentType(ctx, event)
	if agentType == "" {
		agentType = "*"
	}

	switch event.Type {
	case dispatcher.EventWorkRequested:
		if agentType == "planner" {
			g.processCreateSession(ctx, event, agentType)
		} else {
			g.processAssignTask(ctx, event, agentType)
		}
	case dispatcher.EventTaskCompleted:
		if agentType == "planner" || agentType == "executor" || agentType == "reviewer" || agentType == "builder" {
			g.processCreateSession(ctx, event, agentType)
		}
	case dispatcher.EventGateTaskCreated:
		g.processCreateSession(ctx, event, agentType)
	default:
		// noop
	}
}

// resolveAgentType extracts the agent type from the event by inspecting the
// referenced task (if available), the event payload, or the session.
func (g *Gateway) resolveAgentType(ctx context.Context, event dispatcher.Event) string {
	// If the event has a task ID, look up the task.
	if event.TaskID != "" {
		task, err := g.taskStore.GetByID(ctx, event.TaskID)
		if err == nil && task != nil {
			// First, try capability-based matching using task_type.
			if task.TaskType != "" {
				g.mu.RLock()
				// Collect matching profile names for deterministic iteration.
				var candidates []string
				for name, taskTypes := range g.profileTaskTypes {
					for _, tt := range taskTypes {
						if tt == string(task.TaskType) {
							candidates = append(candidates, name)
							break
						}
					}
				}
				g.mu.RUnlock()

				if len(candidates) > 0 {
					// Sort for deterministic behavior (first alphabetically).
					sort.Strings(candidates)
					return candidates[0]
				}
			}

			// Fall back to the task's agent_type field.
			if task.AgentType != "" {
				return task.AgentType
			}
		}
	}

	// Try to extract agent_type from the payload (if it's a map).
	if event.Payload != nil {
		if m, ok := event.Payload.(map[string]interface{}); ok {
			if at, ok := m["agent_type"].(string); ok && at != "" {
				return at
			}
		}
	}

	// Try the session if we have a session ID.
	if event.SessionID != "" {
		session, err := g.sessionStore.GetByID(ctx, event.SessionID)
		if err == nil && session != nil {
			return session.HarnessType
		}
	}

	return ""
}

// ---------------------------------------------------------------------------
// Action handlers
// ---------------------------------------------------------------------------

// processCreateSession handles the create_session action. It looks up the
// project from the event's task, checks for an existing session, and either
// resumes or creates a new ACP session.
func (g *Gateway) processCreateSession(ctx context.Context, event dispatcher.Event, agentType string) {
	projectID := g.resolveProjectID(ctx, event)
	if projectID == "" {
		slog.Warn("gateway: cannot create session, no project_id",
			"event_type", event.Type, "task_id", event.TaskID)
		return
	}

	// Resolve story ID and task ID for session context building.
	storyID := g.resolveStoryID(ctx, event)
	taskID := event.TaskID

	// Check if a gateway session already exists for this (project, agentType).
	if existing, ok := g.tracker.GetSession(projectID, agentType); ok {
		slog.Info("gateway: session already exists, resuming",
			"project_id", projectID,
			"agent_type", agentType,
			"session_id", existing.SessionID)

		if err := g.resumeACPSession(ctx, existing); err != nil {
			slog.Error("gateway: failed to resume session",
				"project_id", projectID,
				"agent_type", agentType,
				"session_id", existing.SessionID,
				"error", err)
		}
		return
	}

	// No existing session — create a new one.
	slog.Info("gateway: creating new session",
		"project_id", projectID, "agent_type", agentType)

	projectName := projectID
	if p, err := g.projectStore.GetByID(ctx, projectID); err == nil && p != nil {
		projectName = p.Name
	}

	if err := g.createACPSession(ctx, projectID, agentType, storyID, taskID); err != nil {
		slog.Error("gateway: failed to create session",
			"project_id", projectID,
			"agent_type", agentType,
			"error", err)
		g.logActivity(ctx, projectID, "project", "gateway_session_create_failed",
			fmt.Sprintf("agent_type=%s project=%s error=%v", agentType, projectName, err))
		return
	}

	// Store the story ID on the session so that handleACPMessage can persist
	// the ACP session ID back to the story (planner sessions created via
	// processCreateSession do not go through assignTaskToSession, so
	// AssignedTaskID is never set).
	if gs, ok := g.tracker.GetSession(projectID, agentType); ok && storyID != "" {
		gs.StoryID = storyID
	}

	g.logActivity(ctx, projectID, "project", "gateway_session_created",
		fmt.Sprintf("agent_type=%s project=%s", agentType, projectName))
}

// processAssignTask handles the assign_task action. It finds an available
// session for the agent type and assigns the task, or queues the work if
// no capacity is available.
func (g *Gateway) processAssignTask(ctx context.Context, event dispatcher.Event, agentType string) {
	projectID := g.resolveProjectID(ctx, event)
	if projectID == "" {
		slog.Warn("gateway: cannot assign task, no project_id",
			"event_type", event.Type, "task_id", event.TaskID)
		return
	}

	taskID := event.TaskID
	if taskID == "" {
		slog.Warn("gateway: cannot assign task, no task_id",
			"event_type", event.Type)
		return
	}

	// Check if capacity exists for this agent type.
	if g.queue.HasCapacity(projectID, agentType) {
		slog.Info("gateway: assigning task to session",
			"task_id", taskID,
			"project_id", projectID,
			"agent_type", agentType)

		if err := g.assignTaskToSession(ctx, projectID, agentType, taskID); err != nil {
			slog.Error("gateway: failed to assign task, queuing",
				"task_id", taskID,
				"project_id", projectID,
				"agent_type", agentType,
				"error", err)

			// Queue the job for later assignment.
			g.queue.Enqueue(projectID, agentType, taskID, event.Type)
			g.logActivity(ctx, taskID, string(models.WorkItemTypeTask),
				"gateway_task_queued",
				fmt.Sprintf("agent_type=%s reason=%v", agentType, err))
		}
	} else {
		// No capacity — queue the job.
		slog.Info("gateway: no capacity, queuing task",
			"task_id", taskID,
			"project_id", projectID,
			"agent_type", agentType)

		g.queue.Enqueue(projectID, agentType, taskID, event.Type)
		g.logActivity(ctx, taskID, string(models.WorkItemTypeTask),
			"gateway_task_queued",
			fmt.Sprintf("agent_type=%s reason=no_capacity", agentType))
	}
}

// resolveProjectID extracts the project ID from the event by inspecting the
// referenced task, the event payload, or falling back to the session.
func (g *Gateway) resolveProjectID(ctx context.Context, event dispatcher.Event) string {
	// If the event has a task ID, look up the task to get its story, then project.
	if event.TaskID != "" {
		task, err := g.taskStore.GetByID(ctx, event.TaskID)
		if err == nil && task != nil {
			// Get the story to find the project ID.
			story, err := g.storyStore.GetByID(ctx, task.StoryID)
			if err == nil && story != nil && story.ProjectID != "" {
				return story.ProjectID
			}
		}
	}

	// Try to extract project_id from the payload.
	if event.Payload != nil {
		if m, ok := event.Payload.(map[string]interface{}); ok {
			if pid, ok := m["project_id"].(string); ok && pid != "" {
				return pid
			}
		}
	}

	return ""
}

// resolveStoryID extracts the story ID from the event by inspecting the
// referenced task, the event payload (for gate-style events), or falling
// back to the active planner session's assigned task.
func (g *Gateway) resolveStoryID(ctx context.Context, event dispatcher.Event) string {
	// If the event has a task ID, look up the task to get its story.
	if event.TaskID != "" {
		task, err := g.taskStore.GetByID(ctx, event.TaskID)
		if err == nil && task != nil && task.StoryID != "" {
			return task.StoryID
		}
	}

	// Try to extract story_id from the payload (e.g., gate_task_created events).
	if event.Payload != nil {
		if m, ok := event.Payload.(map[string]interface{}); ok {
			if sid, ok := m["story_id"].(string); ok && sid != "" {
				return sid
			}
		}
	}

	return ""
}

// ---------------------------------------------------------------------------
// ACP session management
// ---------------------------------------------------------------------------

// createACPSession creates a new ACP session for the given project and agent
// type. It creates the ACP session via the client, registers it in the
// tracker, and sends the initial context as a prompt. storyID and taskID are
// used to build session context (story details, tasks, comments) that is
// included as the initial prompt.
func (g *Gateway) createACPSession(ctx context.Context, projectID, agentType, storyID, taskID string) error {
	client, err := g.getOrCreateACPClient(ctx, projectID, agentType)
	if err != nil {
		return fmt.Errorf("get or create acp client: %w", err)
	}

	// Determine project path and build MCP server config.
	projectPath := projectID
	mcpServers := []acp.MCPServer{}
	if p, err := g.projectStore.GetByID(ctx, projectID); err == nil && p != nil && p.RepoPath != "" {
		projectPath = p.RepoPath
		mcpServers = append(mcpServers, acp.MCPServer{
			Name:    "loom",
			Command: p.RepoPath + "/dist/loom-server",
			Args:    []string{"--mcp"},
			Env:     []acp.EnvVar{},
		})
	}

	// Create the ACP session.
	sessionID, err := client.NewSession(ctx, acp.NewSessionRequest{
		Cwd:        projectPath,
		MCPServers: mcpServers,
	})
	if err != nil {
		return fmt.Errorf("new acp session: %w", err)
	}

	// Register the session in the tracker with the real session ID.
	_ = g.tracker.RegisterSession(projectID, agentType, sessionID)

	// Register the session client for future prompt sends.
	g.RegisterSessionClient(sessionID, client)

	// Build and send the initial context as a prompt.
	acpCtx, err := g.buildACPContext(ctx, storyID, taskID, agentType, false)
	if err != nil {
		slog.Warn("gateway: failed to build acp context, proceeding without it",
			"project_id", projectID,
			"story_id", storyID,
			"task_id", taskID,
			"agent_type", agentType,
			"error", err)
	} else {
		stopReason, promptErr := client.SendPrompt(ctx, sessionID, acpCtx)
		if promptErr != nil {
			slog.Warn("gateway: failed to send initial prompt, continuing",
				"project_id", projectID,
				"session_id", sessionID,
				"error", promptErr)
		} else {
			slog.Info("gateway: initial prompt sent to session",
				"project_id", projectID,
				"session_id", sessionID,
				"stop_reason", stopReason)
		}
	}

	slog.Info("gateway: acp session created",
		"project_id", projectID,
		"agent_type", agentType,
		"session_id", sessionID)

	return nil
}

// buildACPContext fetches story and/or task data and returns a formatted
// context string suitable for any ACP session. The context includes the
// system prompt for the given agent type, an MCP server hint, and optional
// story/task details. If isUpdate is true, a "[CONTEXT UPDATE]" prefix is
// prepended.
func (g *Gateway) buildACPContext(ctx context.Context, storyID, taskID, agentType string, isUpdate bool) (string, error) {
	var b strings.Builder

	// Prefix for context updates.
	if isUpdate {
		b.WriteString("[CONTEXT UPDATE] The user has answered your question.\n\n")
	}

	// System prompt.
	b.WriteString(SystemPrompt(agentType))
	b.WriteString("\n\n## CONTEXT\n\n")

	// MCP server hint.
	b.WriteString("Connect to Loom's MCP server (configured in your opencode.json's mcpServers as 'loom').\n\n")

	// Story data.
	if storyID != "" {
		story, err := g.storyStore.GetByID(ctx, storyID)
		if err != nil {
			return "", fmt.Errorf("get story %q: %w", storyID, err)
		}
		if story == nil {
			return "", fmt.Errorf("story %q not found", storyID)
		}

		fmt.Fprintf(&b, "Story ID: %s\n", story.ID)
		fmt.Fprintf(&b, "Story Title: %s\n", story.Title)
		if story.Description != "" {
			fmt.Fprintf(&b, "Story Description: %s\n", story.Description)
		}
		fmt.Fprintf(&b, "Story Status: %s\n", story.Status)

		// Tasks for the story.
		tasks, err := g.taskStore.GetByStory(ctx, storyID)
		if err != nil {
			return "", fmt.Errorf("get tasks for story %q: %w", storyID, err)
		}

		b.WriteString("\nCurrent Tasks:\n")
		if len(tasks) == 0 {
			b.WriteString("- (no tasks)\n")
		} else {
			for _, task := range tasks {
				if task == nil {
					continue
				}
				fmt.Fprintf(&b, "- %s: %s [%s]\n", task.ID, task.Title, task.Status)
			}
		}

		// Comments.
		comments, err := g.commentStore.GetByWorkItem(ctx, storyID, models.WorkItemTypeStory)
		if err != nil {
			return "", fmt.Errorf("get comments for story %q: %w", storyID, err)
		}

		b.WriteString("\nCurrent Comments (chronological):\n")
		if len(comments) == 0 {
			b.WriteString("- (no comments)\n")
		} else {
			for _, c := range comments {
				if c == nil {
					continue
				}
				fmt.Fprintf(&b, "- [%s] %s/%s: %s\n",
					c.CreatedAt.UTC().Format(time.RFC3339),
					c.AuthorType, c.AuthorID, c.Body)
			}
		}
	}

	// Task-specific data.
	if taskID != "" {
		task, err := g.taskStore.GetByID(ctx, taskID)
		if err != nil {
			return "", fmt.Errorf("get task %q: %w", taskID, err)
		}
		if task != nil {
			fmt.Fprintf(&b, "\nAssigned Task:\n")
			fmt.Fprintf(&b, "- ID: %s\n", task.ID)
			fmt.Fprintf(&b, "- Title: %s\n", task.Title)
			if task.Description != "" {
				fmt.Fprintf(&b, "- Description: %s\n", task.Description)
			}
			fmt.Fprintf(&b, "- Status: %s\n", task.Status)
			fmt.Fprintf(&b, "- Task Type: %s\n", task.TaskType)
			if task.Instructions != "" {
				fmt.Fprintf(&b, "- Instructions: %s\n", task.Instructions)
			}
		}
	}

	return b.String(), nil
}

// resumeACPSession re-establishes an existing session by re-registering the
// session client. The client's Connect() already auto-initializes, so no
// resume message is needed.
func (g *Gateway) resumeACPSession(ctx context.Context, session *GatewaySession) error {
	client, err := g.getOrCreateACPClient(ctx, session.ProjectID, session.AgentType)
	if err != nil {
		return fmt.Errorf("get or create acp client for resume: %w", err)
	}

	// Re-register the session client.
	g.RegisterSessionClient(session.SessionID, client)

	// Update the session status.
	if _, err := g.tracker.UpdateStatus(session.ProjectID, session.AgentType, SessionActive); err != nil {
		slog.Warn("gateway: failed to update session status on resume",
			"project_id", session.ProjectID,
			"agent_type", session.AgentType,
			"error", err)
	}

	slog.Info("gateway: acp session resumed",
		"project_id", session.ProjectID,
		"agent_type", session.AgentType,
		"session_id", session.SessionID)

	return nil
}

// assignTaskToSession finds an available session for the given agent type,
// assigns the task, and sends it as a prompt via ACP. If no session exists,
// one is created on the fly using NewSession.
func (g *Gateway) assignTaskToSession(ctx context.Context, projectID, agentType, taskID string) error {
	client, err := g.getOrCreateACPClient(ctx, projectID, agentType)
	if err != nil {
		return fmt.Errorf("get or create acp client for task assignment: %w", err)
	}

	// Get or register a session in the tracker.
	gs, ok := g.tracker.GetSession(projectID, agentType)
	if !ok {
		// No session yet — create one on the fly.
		projectPath := projectID
		mcpServers := []acp.MCPServer{}
		if p, err := g.projectStore.GetByID(ctx, projectID); err == nil && p != nil && p.RepoPath != "" {
			projectPath = p.RepoPath
			mcpServers = append(mcpServers, acp.MCPServer{
				Name:    "loom",
				Command: p.RepoPath + "/dist/loom-server",
				Args:    []string{"--mcp"},
				Env:     []acp.EnvVar{},
			})
		}

		sessionID, err := client.NewSession(ctx, acp.NewSessionRequest{
			Cwd:        projectPath,
			MCPServers: mcpServers,
		})
		if err != nil {
			return fmt.Errorf("new acp session for task assignment: %w", err)
		}

		gs = g.tracker.RegisterSession(projectID, agentType, sessionID)
		g.RegisterSessionClient(sessionID, client)
	}

	// Look up the task for details.
	task, err := g.taskStore.GetByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("get task %q: %w", taskID, err)
	}

	// Build task context and send as a prompt.
	var b strings.Builder
	fmt.Fprintf(&b, "Task ID: %s\n", task.ID)
	fmt.Fprintf(&b, "Task Title: %s\n", task.Title)
	if task.Description != "" {
		fmt.Fprintf(&b, "Task Description: %s\n", task.Description)
	}
	fmt.Fprintf(&b, "Task Status: %s\n", task.Status)
	if task.Instructions != "" {
		fmt.Fprintf(&b, "Task Instructions: %s\n", task.Instructions)
	}

	if _, err := client.SendPrompt(ctx, gs.SessionID, b.String()); err != nil {
		return fmt.Errorf("send task prompt: %w", err)
	}

	// Mark the session as busy with the assigned task.
	if _, err := g.tracker.AssignTask(projectID, agentType, taskID); err != nil {
		return fmt.Errorf("tracker assign task: %w", err)
	}

	g.queue.IncrementActive(projectID, agentType)

	slog.Info("gateway: task assigned to session",
		"task_id", taskID,
		"project_id", projectID,
		"agent_type", agentType,
		"session_id", gs.SessionID)

	return nil
}

// ---------------------------------------------------------------------------
// ACP message handling
// ---------------------------------------------------------------------------

// handleACPMessage processes an incoming ACP message from the receive channel.
// Since session creation is now synchronous (NewSession returns the session ID
// directly), this channel only receives notifications and unmatched messages.
func (g *Gateway) handleACPMessage(ctx context.Context, msg []byte, projectID, agentType string, client *acp.Client) {
	// Try to parse as a JSON-RPC response for logging.
	var generic map[string]any
	if err := json.Unmarshal(msg, &generic); err != nil {
		slog.Warn("gateway: failed to unmarshal acp message",
			"project_id", projectID,
			"agent_type", agentType,
			"error", err)
		return
	}

	slog.Debug("gateway: acp notification received",
		"project_id", projectID,
		"agent_type", agentType,
		"message", generic)

	// Check for error field.
	if errVal, ok := generic["error"]; ok && errVal != nil {
		slog.Warn("gateway: acp response indicated failure",
			"project_id", projectID,
			"agent_type", agentType,
			"error", errVal)

		if gs, ok := g.tracker.GetSession(projectID, agentType); ok {
			_, _ = g.tracker.UpdateStatus(gs.ProjectID, gs.AgentType, SessionError)
			g.queue.DecrementActive(gs.ProjectID, gs.AgentType)
			if gs.AssignedTaskID != "" {
				g.queue.Remove(gs.AssignedTaskID)
				g.tryDequeueNextJob(ctx, gs.ProjectID, gs.AgentType)
			}
		}
		return
	}

	// Update heartbeat for the session.
	if gs, ok := g.tracker.GetSession(projectID, agentType); ok {
		_ = g.tracker.Heartbeat(gs.ProjectID, gs.AgentType)
	}
}

// tryDequeueNextJob checks for queued jobs for the given (projectID, agentType)
// and assigns the next one if capacity allows.
func (g *Gateway) tryDequeueNextJob(ctx context.Context, projectID, agentType string) {
	if !g.queue.HasCapacity(projectID, agentType) {
		return
	}

	job := g.queue.Dequeue(projectID, agentType)
	if job == nil {
		return
	}

	slog.Info("gateway: dequeuing job for assignment",
		"task_id", job.TaskID,
		"project_id", job.ProjectID,
		"agent_type", job.AgentType)

	if err := g.assignTaskToSession(ctx, job.ProjectID, job.AgentType, job.TaskID); err != nil {
		slog.Error("gateway: failed to assign dequeued task, re-queuing",
			"task_id", job.TaskID,
			"project_id", job.ProjectID,
			"agent_type", job.AgentType,
			"error", err)
		// Re-queue the job at the front (push it back).
		// We use Enqueue which appends to the end, which is acceptable.
		g.queue.Enqueue(job.ProjectID, job.AgentType, job.TaskID, job.EventRef)
	}
}

// ---------------------------------------------------------------------------
// Staleness checking
// ---------------------------------------------------------------------------

// checkStaleness iterates all tracked sessions and marks any that have been
// silent for longer than stalenessThreshold as stale/error. It decrements
// active counts for stale sessions and drains the queue for freed capacity.
func (g *Gateway) checkStaleness(ctx context.Context) {
	now := time.Now().UTC()
	staleCutoff := now.Add(-stalenessThreshold)

	for _, s := range g.tracker.ListAll() {
		// Skip sessions that are already in an error state.
		if s.Status == SessionError {
			continue
		}

		if s.LastHeartbeat.Before(staleCutoff) {
			slog.Warn("gateway: session is stale",
				"project_id", s.ProjectID,
				"agent_type", s.AgentType,
				"session_id", s.SessionID,
				"last_heartbeat", s.LastHeartbeat,
				"status", s.Status)

			// Mark the session as error.
			if _, err := g.tracker.UpdateStatus(s.ProjectID, s.AgentType, SessionError); err != nil {
				slog.Warn("gateway: failed to mark session stale",
					"project_id", s.ProjectID, "agent_type", s.AgentType, "error", err)
				continue
			}

			// Disconnect the session in the persistent store so stale
			// sessions do not persist in the database.
			if g.sessionStore != nil && s.SessionID != "" {
				if err := g.sessionStore.Disconnect(ctx, s.SessionID); err != nil {
					slog.Warn("gateway: failed to disconnect stale session in db",
						"project_id", s.ProjectID,
						"session_id", s.SessionID,
						"error", err)
				}
			}

			// If this session had an assigned task, remove the job from the queue
			// and decrement the active count so another session can pick it up.
			if s.AssignedTaskID != "" {
				g.queue.Remove(s.AssignedTaskID)
			}

			// Decrement active count to free capacity.
			g.queue.DecrementActive(s.ProjectID, s.AgentType)

			// Try to dequeue the next job.
			g.tryDequeueNextJob(ctx, s.ProjectID, s.AgentType)

			// Log the staleness event.
			g.logActivity(ctx, s.SessionID, "session",
				"gateway_session_stale",
				fmt.Sprintf("project_id=%s agent_type=%s last_heartbeat=%s",
					s.ProjectID, s.AgentType, s.LastHeartbeat.Format(time.RFC3339)))

			// Broadcast a staleness event through the dispatcher.
			g.dispatcher.Submit(ctx, dispatcher.Event{
				Type:      dispatcher.EventSessionStale,
				SessionID: s.SessionID,
				Payload: map[string]string{
					"project_id": s.ProjectID,
					"agent_type": s.AgentType,
				},
			})
		}
	}
}
