package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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

	// Evaluate the rules engine to determine the appropriate action.
	action := g.rules.Evaluate(event.Type, agentType)
	if action == ActionNoOp {
		// Also try a wildcard match if we had a specific agent type.
		if agentType != "*" {
			action = g.rules.Evaluate(event.Type, "*")
		}
	}

	slog.Debug("gateway: event evaluated",
		"event_type", event.Type,
		"agent_type", agentType,
		"action", action)

	switch action {
	case ActionCreateSession, ActionResumeSession:
		g.processCreateSession(ctx, event, agentType)
	case ActionAssignTask:
		g.processAssignTask(ctx, event, agentType)
	case ActionNoOp:
		// Nothing to do.
	default:
		slog.Warn("gateway: unknown action", "action", action)
	}
}

// resolveAgentType extracts the agent type from the event by inspecting the
// referenced task (if available), the event payload, or the session.
func (g *Gateway) resolveAgentType(ctx context.Context, event dispatcher.Event) string {
	// If the event has a task ID, look up the task to get its agent_type.
	if event.TaskID != "" {
		task, err := g.taskStore.GetByID(ctx, event.TaskID)
		if err == nil && task != nil && task.AgentType != "" {
			return task.AgentType
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

	if err := g.createACPSession(ctx, projectID, agentType); err != nil {
		slog.Error("gateway: failed to create session",
			"project_id", projectID,
			"agent_type", agentType,
			"error", err)
		g.logActivity(ctx, projectID, "project", "gateway_session_create_failed",
			fmt.Sprintf("agent_type=%s error=%v", agentType, err))
		return
	}

	g.logActivity(ctx, projectID, "project", "gateway_session_created",
		fmt.Sprintf("agent_type=%s", agentType))
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

// ---------------------------------------------------------------------------
// ACP session management
// ---------------------------------------------------------------------------

// createACPSession creates a new ACP session for the given project and agent
// type. It dials the ACP WebSocket endpoint, sends a create_session message,
// and registers the session in the tracker.
func (g *Gateway) createACPSession(ctx context.Context, projectID, agentType string) error {
	client, err := g.getOrCreateACPClient(ctx, projectID, agentType)
	if err != nil {
		return fmt.Errorf("get or create acp client: %w", err)
	}

	// Send a create_session message.
	sessionMsg := acp.SessionMessage{
		Type:      "create_session",
		ProjectID: projectID,
		AgentType: agentType,
	}

	if err := client.Send(sessionMsg); err != nil {
		return fmt.Errorf("send create_session: %w", err)
	}

	// Register the session in the tracker with a temporary session ID.
	// The actual ACP session ID will be received asynchronously via the
	// ACP response channel. We use a placeholder until the response arrives.
	placeholderID := fmt.Sprintf("pending-%s-%s-%d", projectID, agentType, time.Now().UnixNano())
	session := g.tracker.RegisterSession(projectID, agentType, placeholderID)

	slog.Info("gateway: acp session creation requested",
		"project_id", projectID,
		"agent_type", agentType,
		"session_id", session.SessionID)

	return nil
}

// resumeACPSession sends a resume_session message for an existing session
// to the ACP server. This re-establishes the session after a disconnect.
func (g *Gateway) resumeACPSession(ctx context.Context, session *GatewaySession) error {
	client, err := g.getOrCreateACPClient(ctx, session.ProjectID, session.AgentType)
	if err != nil {
		return fmt.Errorf("get or create acp client for resume: %w", err)
	}

	resumeMsg := acp.SessionMessage{
		Type:      "resume_session",
		SessionID: session.SessionID,
		ProjectID: session.ProjectID,
		AgentType: session.AgentType,
	}

	if err := client.Send(resumeMsg); err != nil {
		return fmt.Errorf("send resume_session: %w", err)
	}

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
// assigns the task, and sends a get_task message via ACP.
func (g *Gateway) assignTaskToSession(ctx context.Context, projectID, agentType, taskID string) error {
	// Get or create an ACP client.
	client, err := g.getOrCreateACPClient(ctx, projectID, agentType)
	if err != nil {
		return fmt.Errorf("get or create acp client for task assignment: %w", err)
	}

	// Get or register a session in the tracker.
	gs, ok := g.tracker.GetSession(projectID, agentType)
	if !ok {
		// No session yet — create one on the fly.
		placeholderID := fmt.Sprintf("pending-%s-%s-%d", projectID, agentType, time.Now().UnixNano())
		gs = g.tracker.RegisterSession(projectID, agentType, placeholderID)

		// Also send a create_session to the ACP server.
		sessionMsg := acp.SessionMessage{
			Type:      "create_session",
			ProjectID: projectID,
			AgentType: agentType,
		}
		if err := client.Send(sessionMsg); err != nil {
			return fmt.Errorf("send create_session before task assignment: %w", err)
		}
	}

	// Look up the task for details.
	task, err := g.taskStore.GetByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("get task %q: %w", taskID, err)
	}

	// Build and send the task message.
	taskMsg := acp.TaskMessage{
		Type:        "get_task",
		TaskID:      task.ID,
		SessionID:   gs.SessionID,
		Title:       task.Title,
		Description: task.Description,
		Status:      string(task.Status),
	}

	if task.Instructions != "" {
		taskMsg.Instructions = task.Instructions
	}

	if err := client.Send(taskMsg); err != nil {
		return fmt.Errorf("send get_task: %w", err)
	}

	// Mark the session as busy with the assigned task.
	if _, err := g.tracker.AssignTask(projectID, agentType, taskID); err != nil {
		return fmt.Errorf("tracker assign task: %w", err)
	}

	// Increment the active count for capacity tracking.
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

// handleACPMessage processes an incoming ACP response message. It parses the
// raw JSON bytes into an ACPResponse and dispatches accordingly.
func (g *Gateway) handleACPMessage(ctx context.Context, msg []byte, projectID, agentType string) {
	var resp acp.ACPResponse
	if err := json.Unmarshal(msg, &resp); err != nil {
		slog.Warn("gateway: failed to unmarshal acp response",
			"project_id", projectID,
			"agent_type", agentType,
			"error", err)
		return
	}

	slog.Debug("gateway: acp response received",
		"project_id", projectID,
		"agent_type", agentType,
		"success", resp.Success,
		"session_id", resp.SessionID,
		"task_id", resp.TaskID)

	if !resp.Success {
		slog.Warn("gateway: acp response indicated failure",
			"project_id", projectID,
			"agent_type", agentType,
			"session_id", resp.SessionID,
			"error", resp.Error)

		// If there was an error, mark the session as errored and attempt
		// to dequeue the next job.
		if resp.SessionID != "" {
			if gs, ok := g.tracker.GetBySessionID(resp.SessionID); ok {
				_, _ = g.tracker.UpdateStatus(gs.ProjectID, gs.AgentType, SessionError)
				// Decrement active count and try next job.
				g.queue.DecrementActive(gs.ProjectID, gs.AgentType)
				if gs.AssignedTaskID != "" {
					g.queue.Remove(gs.AssignedTaskID)
					g.tryDequeueNextJob(ctx, gs.ProjectID, gs.AgentType)
				}
			}
		}
		return
	}

	// Update the session ID in the tracker if this is a session creation response.
	if resp.SessionID != "" {
		if gs, ok := g.tracker.GetSession(projectID, agentType); ok {
			// Only update if the current session ID starts with "pending-".
			if len(gs.SessionID) > 8 && gs.SessionID[:8] == "pending-" {
				if _, err := g.tracker.UpdateSessionID(projectID, agentType, resp.SessionID); err != nil {
					slog.Warn("gateway: failed to update session id in tracker",
						"project_id", projectID,
						"agent_type", agentType,
						"error", err)
				} else {
					slog.Info("gateway: session id updated from acp response",
						"project_id", projectID,
						"agent_type", agentType,
						"session_id", resp.SessionID)
				}
			}
		}
	}

	// Handle task completion.
	if resp.TaskID != "" {
		g.handleTaskCompleted(ctx, resp, projectID, agentType)
	}

	// Update heartbeat for the session.
	if gs, ok := g.tracker.GetSession(projectID, agentType); ok {
		_ = g.tracker.Heartbeat(gs.ProjectID, gs.AgentType)
	}
}

// handleTaskCompleted processes a task completion ACP response. It updates
// the task status in the store, logs activity, broadcasts the event through
// the dispatcher, and dequeues the next job.
func (g *Gateway) handleTaskCompleted(ctx context.Context, resp acp.ACPResponse, projectID, agentType string) {
	// Update the task status to done in the store.
	if err := g.taskStore.UpdateStatus(ctx, resp.TaskID, models.StatusDone); err != nil {
		slog.Error("gateway: failed to update task status to done",
			"task_id", resp.TaskID, "error", err)
	}

	// Log the activity.
	g.logActivity(ctx, resp.TaskID, string(models.WorkItemTypeTask),
		"gateway_task_completed",
		fmt.Sprintf("agent_type=%s session_id=%s", agentType, resp.SessionID))

	// Broadcast a task completed event through the dispatcher so the
	// dispatcher can resolve dependencies and check gates.
	g.dispatcher.Submit(ctx, dispatcher.Event{
		Type:      dispatcher.EventTaskCompleted,
		TaskID:    resp.TaskID,
		SessionID: resp.SessionID,
	})

	slog.Info("gateway: task completed via acp",
		"task_id", resp.TaskID,
		"project_id", projectID,
		"agent_type", agentType)

	// Mark the session as idle and clear the assigned task.
	if _, err := g.tracker.CompleteTask(projectID, agentType); err != nil {
		slog.Warn("gateway: failed to complete task in tracker",
			"project_id", projectID, "agent_type", agentType, "error", err)
	}

	// Decrement the active count and try to dequeue the next job.
	g.queue.DecrementActive(projectID, agentType)
	g.tryDequeueNextJob(ctx, projectID, agentType)
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
