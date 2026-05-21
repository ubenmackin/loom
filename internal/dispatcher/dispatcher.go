// Package dispatcher implements the background event-processing loop that
// drives task assignment, gate creation, dependency resolution, and
// staleness detection for the Loom Kanban board.
package dispatcher

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
)

// EventBroadcaster is a minimal interface for broadcasting WebSocket events.
// The concrete WebSocket hub will implement this interface.
type EventBroadcaster interface {
	Broadcast(eventType string, payload any)
}

// Event represents a discrete event processed by the dispatcher loop.
type Event struct {
	Type      string // "task_status_changed", "task_blocked", "session_registered", "work_requested", "dependency_added", "periodic_tick"
	TaskID    string
	SessionID string
	Payload   any
}

// Event type constants for external consumers (e.g., API handlers).
const (
	EventTaskCompleted = "task_status_changed"
	EventTaskBlocked   = "task_blocked"
	EventWorkRequested = "work_requested"
)

// Dispatcher is the brain of the Loom system. It runs as a background
// goroutine with an event-driven loop that processes state changes and
// drives automated workflows (assignment, gates, staleness).
type Dispatcher struct {
	stories    *store.StoryStore
	tasks      *store.TaskStore
	sessions   *store.SessionStore
	templates  *store.TemplateStore
	comments   *store.CommentStore
	activities *store.ActivityStore

	hub EventBroadcaster

	eventCh            chan Event
	stalenessThreshold time.Duration
	done               chan struct{}
}

// NewDispatcher creates a new Dispatcher with the given stores and event hub.
// The stalenessThreshold controls how long a session can be silent before
// being flagged as stale (default 30 minutes if zero).
func NewDispatcher(
	stories *store.StoryStore,
	tasks *store.TaskStore,
	sessions *store.SessionStore,
	templates *store.TemplateStore,
	comments *store.CommentStore,
	activities *store.ActivityStore,
	hub EventBroadcaster,
	stalenessThreshold time.Duration,
) *Dispatcher {
	if stalenessThreshold <= 0 {
		stalenessThreshold = 30 * time.Minute
	}
	return &Dispatcher{
		stories:            stories,
		tasks:              tasks,
		sessions:           sessions,
		templates:          templates,
		comments:           comments,
		activities:         activities,
		hub:                hub,
		eventCh:            make(chan Event, 256),
		stalenessThreshold: stalenessThreshold,
		done:               make(chan struct{}),
	}
}

// Start launches the dispatcher goroutine loop and the periodic ticker.
func (d *Dispatcher) Start() {
	go d.run()
}

// Stop signals the dispatcher to shut down gracefully.
func (d *Dispatcher) Stop() {
	close(d.done)
}

// Submit sends an event to the dispatcher channel. It is non-blocking: if
// the channel is full, the event is dropped and a warning is logged.
func (d *Dispatcher) Submit(event Event) {
	select {
	case d.eventCh <- event:
	default:
		slog.Warn("dispatcher event channel full, dropping event",
			"event_type", event.Type,
			"task_id", event.TaskID,
			"session_id", event.SessionID,
		)
	}
}

// AssignWork finds and assigns the best available task for a specific session.
// This is the synchronous API for the work request flow. It returns the
// assigned task (or nil if no work is available) and any error encountered.
func (d *Dispatcher) AssignWork(ctx context.Context, sessionID string) (*models.Task, error) {
	session, err := d.sessions.GetByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session %q: %w", sessionID, err)
	}
	if session.Status != "active" {
		return nil, fmt.Errorf("session %q is not active (status=%q)", sessionID, session.Status)
	}

	return d.findAndAssignTaskForSession(ctx, session)
}

// run is the main event loop. It processes events from eventCh and
// periodic ticks for staleness checks.
func (d *Dispatcher) run() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	slog.Info("dispatcher started")

	for {
		select {
		case <-d.done:
			slog.Info("dispatcher stopped")
			return

		case event := <-d.eventCh:
			d.processEvent(event)

		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			d.checkStaleness(ctx)
			cancel()
		}
	}
}

// processEvent dispatches an event to the appropriate handler.
func (d *Dispatcher) processEvent(event Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch event.Type {
	case "task_status_changed":
		d.handleTaskStatusChanged(ctx, event)
	case "task_blocked":
		d.handleTaskBlocked(ctx, event)
	case "session_registered":
		d.runAssignmentPass(ctx)
	case "work_requested":
		d.handleWorkRequested(ctx, event)
	case "dependency_added":
		d.handleDependencyAdded(ctx, event)
	case "periodic_tick":
		d.checkStaleness(ctx)
	default:
		slog.Warn("dispatcher: unknown event type", "event_type", event.Type)
	}
}

// handleTaskStatusChanged processes a task status change event. When a task
// transitions to "done", it resolves dependencies and checks gate conditions
// for the parent story.
func (d *Dispatcher) handleTaskStatusChanged(ctx context.Context, event Event) {
	if event.TaskID == "" {
		return
	}

	task, err := d.tasks.GetByID(ctx, event.TaskID)
	if err != nil {
		slog.Error("dispatcher: failed to get task for status change",
			"task_id", event.TaskID, "error", err)
		return
	}

	if task.Status == models.StatusDone {
		d.resolveDependencies(ctx, event.TaskID)
		d.checkGateConditions(ctx, task.StoryID)
	}

	// Also attempt assignment in case a freed session can pick up new work.
	d.runAssignmentPass(ctx)
}

// handleTaskBlocked processes a task-blocked event. When a task is blocked,
// we log the state change and re-evaluate dependencies to see if the task
// can be immediately unblocked.
func (d *Dispatcher) handleTaskBlocked(ctx context.Context, event Event) {
	if event.TaskID == "" {
		return
	}

	// Re-check whether all deps are now satisfied (task may already be unblocked).
	blockers, err := d.tasks.GetBlockers(ctx, event.TaskID)
	if err != nil {
		slog.Error("dispatcher: failed to get blockers for blocked event",
			"task_id", event.TaskID, "error", err)
		return
	}
	if len(blockers) == 0 {
		// All dependencies resolved — transition to ready.
		if err := d.tasks.UpdateStatus(ctx, event.TaskID, models.StatusReady); err != nil {
			slog.Error("dispatcher: failed to unblock task",
				"task_id", event.TaskID, "error", err)
			return
		}
		d.logActivity(ctx, event.TaskID, models.WorkItemTypeTask, "unblocked", "")
		d.hub.Broadcast("task_status_changed", map[string]string{
			"task_id": event.TaskID,
			"status":  models.StatusReady,
		})
	}

	// Attempt assignment in case a session is available for other work.
	d.runAssignmentPass(ctx)
}

// handleWorkRequested finds the best task for a specific session that
// requested work.
func (d *Dispatcher) handleWorkRequested(ctx context.Context, event Event) {
	if event.SessionID == "" {
		slog.Warn("dispatcher: work_requested event missing session_id")
		return
	}

	session, err := d.sessions.GetByID(ctx, event.SessionID)
	if err != nil {
		slog.Error("dispatcher: failed to get session for work request",
			"session_id", event.SessionID, "error", err)
		return
	}

	if session.Status != models.SessionStatusActive {
		slog.Info("dispatcher: work requested by inactive session, ignoring",
			"session_id", event.SessionID, "status", session.Status)
		return
	}

	d.assignWorkToSession(ctx, session)
}

// handleDependencyAdded re-evaluates blockers for the task that received
// a new dependency.
func (d *Dispatcher) handleDependencyAdded(ctx context.Context, event Event) {
	if event.TaskID == "" {
		return
	}

	task, err := d.tasks.GetByID(ctx, event.TaskID)
	if err != nil {
		slog.Error("dispatcher: failed to get task for dependency added",
			"task_id", event.TaskID, "error", err)
		return
	}

	// If the task is blocked, re-check whether all deps are now satisfied.
	if task.Status == models.StatusBlocked {
		blockers, err := d.tasks.GetBlockers(ctx, event.TaskID)
		if err != nil {
			slog.Error("dispatcher: failed to get blockers",
				"task_id", event.TaskID, "error", err)
			return
		}
		if len(blockers) == 0 {
			// All dependencies resolved — transition to ready.
			if err := d.tasks.UpdateStatus(ctx, event.TaskID, models.StatusReady); err != nil {
				slog.Error("dispatcher: failed to unblock task",
					"task_id", event.TaskID, "error", err)
				return
			}
			d.logActivity(ctx, event.TaskID, models.WorkItemTypeTask, "unblocked", "")
			d.hub.Broadcast("task_status_changed", map[string]string{
				"task_id": event.TaskID,
				"status":  models.StatusReady,
			})
		}
	}
}

// logActivity is a helper that logs an activity entry and logs any error.
func (d *Dispatcher) logActivity(ctx context.Context, workItemID, workItemType, action, details string) {
	entry := &models.ActivityLogEntry{
		WorkItemID:   workItemID,
		WorkItemType: workItemType,
		Action:       action,
		Details:      details,
	}
	if err := d.activities.Log(ctx, entry); err != nil {
		slog.Error("dispatcher: failed to log activity",
			"work_item_id", workItemID,
			"action", action,
			"error", err)
	}
}
