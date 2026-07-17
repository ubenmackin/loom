// Package dispatcher implements the background event-processing loop that
// drives task assignment, gate creation, dependency resolution, and
// staleness detection for the Loom Kanban board.
package dispatcher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
)

// EventBroadcaster is a minimal interface for broadcasting WebSocket events.
// The concrete WebSocket hub will implement this interface.
type EventBroadcaster interface {
	Broadcast(eventType string, payload any)
}

// GatewaySubmitter allows the Dispatcher to submit events to the Gateway
// for ACP session creation. The Gateway implements this interface.
type GatewaySubmitter interface {
	SubmitEvent(event Event)
}

// Event represents a discrete event processed by the dispatcher loop.
// See events.go for the canonical list of event type constants.
type Event struct {
	Type      string
	TaskID    string
	SessionID string
	Payload   any
}

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

	gateway GatewaySubmitter // optional bridge to Gateway
	mu      sync.RWMutex     // protects gateway access

	eventCh            chan Event
	stalenessThreshold time.Duration
	done               chan struct{}

	wg      sync.WaitGroup
	stopped atomic.Bool
	started atomic.Bool

	startedAt          time.Time
	lastAssignPass     atomic.Pointer[time.Time]
	lastStalenessCheck atomic.Pointer[time.Time]
	eventsProcessed    map[string]*atomic.Int64
}

// DispatcherDeps groups all dependencies required by the Dispatcher.
type DispatcherDeps struct {
	StoryStore         *store.StoryStore
	TaskStore          *store.TaskStore
	SessionStore       *store.SessionStore
	TemplateStore      *store.TemplateStore
	CommentStore       *store.CommentStore
	ActivityStore      *store.ActivityStore
	Broadcaster        EventBroadcaster
	StalenessThreshold time.Duration
}

// DispatcherStatus is a snapshot of the dispatcher's runtime state,
// returned by Dispatcher.Status(). All fields are safe to read without
// blocking the event loop.
type DispatcherStatus struct {
	Running            bool
	StartedAt          time.Time
	Uptime             time.Duration
	EventQueueDepth    int
	EventsProcessed    map[string]int64
	ReadyTasks         int        // count of tasks in "ready" status
	ActiveSessions     int        // count of active sessions
	PendingBuildGates  int        // build gate tasks in ready or in_progress
	PendingReviewGates int        // review gate tasks in ready or in_progress
	StaleSessions      int        // count of stale sessions (last seen > threshold)
	LastAssignPass     *time.Time // timestamp of most recent assignment pass
	LastStalenessCheck *time.Time // timestamp of most recent staleness check
}

// NewDispatcher creates a new Dispatcher with the given dependencies.
// The stalenessThreshold controls how long a session can be silent before
// being flagged as stale (default 30 minutes if zero).
func NewDispatcher(deps DispatcherDeps) *Dispatcher {
	stalenessThreshold := deps.StalenessThreshold
	if stalenessThreshold <= 0 {
		stalenessThreshold = 30 * time.Minute
	}
	return &Dispatcher{
		stories:            deps.StoryStore,
		tasks:              deps.TaskStore,
		sessions:           deps.SessionStore,
		templates:          deps.TemplateStore,
		comments:           deps.CommentStore,
		activities:         deps.ActivityStore,
		hub:                deps.Broadcaster,
		eventCh:            make(chan Event, 256),
		stalenessThreshold: stalenessThreshold,
		done:               make(chan struct{}),
		eventsProcessed: map[string]*atomic.Int64{
			EventTaskCompleted:     new(atomic.Int64),
			EventTaskBlocked:       new(atomic.Int64),
			EventSessionRegistered: new(atomic.Int64),
			EventWorkRequested:     new(atomic.Int64),
			EventDependencyAdded:   new(atomic.Int64),
			EventPeriodicTick:      new(atomic.Int64),
			EventTasksGenerated:    new(atomic.Int64),
		},
	}
}

// Start launches the dispatcher goroutine loop and the periodic ticker.
// It is safe to call multiple times — subsequent calls are no-ops.
func (d *Dispatcher) Start() {
	if d.started.Swap(true) {
		return
	}
	d.startedAt = time.Now()
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.run()
	}()
}

// Stop signals the dispatcher to shut down gracefully. It is idempotent:
// subsequent calls are no-ops. Stop waits for the event loop to finish.
func (d *Dispatcher) Stop() {
	if d.stopped.Swap(true) {
		return
	}
	close(d.done)
	d.wg.Wait()
}

// Status returns a snapshot of the dispatcher's runtime state. It never
// blocks the event loop — all values are read atomically or from immutable
// fields. Count queries use a short-lived background context; if they fail,
// the corresponding fields are left at their zero values (0/nil) to keep the
// snapshot always available even when the database is temporarily
// unavailable.
func (d *Dispatcher) Status() DispatcherStatus {
	s := DispatcherStatus{
		Running:         !d.stopped.Load(),
		StartedAt:       d.startedAt,
		EventQueueDepth: len(d.eventCh),
		EventsProcessed: make(map[string]int64, len(d.eventsProcessed)),
	}
	if !d.startedAt.IsZero() {
		s.Uptime = time.Since(d.startedAt)
	}
	for typ, ctr := range d.eventsProcessed {
		s.EventsProcessed[typ] = ctr.Load()
	}

	// Pipeline metrics: populate counts via lightweight store queries.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if count, err := d.tasks.CountByFilter(ctx, store.TaskFilter{Status: models.StatusReady}); err == nil {
		s.ReadyTasks = count
	}

	if active, err := d.sessions.ListActive(ctx); err == nil {
		s.ActiveSessions = len(active)
	}

	if count, err := d.tasks.CountByTypesAndStatuses(ctx,
		[]models.TaskType{models.TaskTypeBuild},
		[]models.Status{models.StatusReady, models.StatusInProgress},
	); err == nil {
		s.PendingBuildGates = count
	}

	if count, err := d.tasks.CountByTypesAndStatuses(ctx,
		[]models.TaskType{models.TaskTypeReview},
		[]models.Status{models.StatusReady, models.StatusInProgress},
	); err == nil {
		s.PendingReviewGates = count
	}

	if count, err := d.sessions.CountStale(ctx, d.stalenessThreshold); err == nil {
		s.StaleSessions = count
	}

	if p := d.lastAssignPass.Load(); p != nil && !p.IsZero() {
		t := *p
		s.LastAssignPass = &t
	}
	if p := d.lastStalenessCheck.Load(); p != nil && !p.IsZero() {
		t := *p
		s.LastStalenessCheck = &t
	}

	return s
}

// Submit sends an event to the dispatcher channel. It prefers delivering the
// event, but if the channel is full it blocks until either the context is
// canceled, the event can be delivered, or the dispatcher is shutting down.
func (d *Dispatcher) Submit(ctx context.Context, event Event) {
	select {
	case d.eventCh <- event:
	case <-ctx.Done():
	case <-d.done:
		// Dispatcher is shutting down; discard event.
	}
}

// SetGateway sets the Gateway bridge. Thread-safe.
func (d *Dispatcher) SetGateway(g GatewaySubmitter) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.gateway = g
}

// submitToGateway sends an event to the Gateway if the bridge is configured.
func (d *Dispatcher) submitToGateway(event Event) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.gateway != nil {
		d.gateway.SubmitEvent(event)
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
	if session.Status != models.SessionStatusActive {
		return nil, fmt.Errorf("session %q is not active (status=%q)", sessionID, session.Status)
	}

	return d.findAndAssignTaskForSession(ctx, session)
}

// run is the main event loop. It processes events from eventCh and
// periodic ticks for staleness checks.
func (d *Dispatcher) run() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	statusTicker := time.NewTicker(3 * time.Second)
	defer statusTicker.Stop()

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

		case <-statusTicker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			d.broadcastDispatcherStatus(ctx)
			cancel()
		}
	}
}

// processEvent dispatches an event to the appropriate handler.
func (d *Dispatcher) processEvent(event Event) {
	// Increment the atomic counter for this event type (no-op if unknown).
	if ctr, ok := d.eventsProcessed[event.Type]; ok {
		ctr.Add(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch event.Type {
	case EventTaskCompleted:
		d.handleTaskStatusChanged(ctx, event)
	case EventTaskBlocked:
		d.handleTaskBlocked(ctx, event)
	case EventSessionRegistered:
		d.runAssignmentPass(ctx)
	case EventWorkRequested:
		d.handleWorkRequested(ctx, event)
	case EventDependencyAdded:
		d.handleDependencyAdded(ctx, event)
	case EventPeriodicTick:
		d.checkStaleness(ctx)
	case EventTasksGenerated:
		// Informational event — no action needed.
	default:
		slog.Warn("dispatcher: unknown event type", "event_type", event.Type)
	}
}

// handleTaskStatusChanged processes a task status change event. When a task
// transitions to "done", it resolves dependencies and checks gate conditions
// for the parent story. For gate task completions, it also detects failures
// and applies the circuit breaker.
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
		// When a gate task completes (build, review, security, release),
		// check if it was a failure by looking for remediation code tasks.
		if isGateTask(task) {
			if d.detectGateFailure(ctx, task) {
				if d.incrementFailureCount(ctx, task.StoryID) {
					// Circuit breaker tripped — story marked as failed.
					// Skip gate conditions and story completion processing.
					d.runAssignmentPass(ctx)
					return
				}
			}
		}

		// If a remediation code task just completed, re-open the highest-
		// priority Done gate task (build > security > review) so it can be
		// re-run. This is skipped when the circuit breaker has tripped
		// (story is "failed") — in that case the loop must stop. Re-opening
		// happens before the dependency/gate/completion checks below so that
		// the re-opened gate (now Ready) prevents premature story completion.
		if task.TaskType == models.TaskTypeCode {
			story, storyErr := d.stories.GetByID(ctx, task.StoryID)
			if storyErr != nil {
				slog.Error("dispatcher: failed to get story for gate re-open check",
					"story_id", task.StoryID, "error", storyErr)
			} else if story.Status != models.StatusFailed {
				d.reopenGateTask(ctx, task.StoryID)
			}
		}

		d.resolveDependencies(ctx, event.TaskID)
		d.checkGateConditions(ctx, task.StoryID)
		d.checkStoryCompletion(ctx, task.StoryID)
	}

	// Also attempt assignment in case a freed session can pick up new work.
	d.runAssignmentPass(ctx)
}

// isGateTask returns true if the task type is a gate task (build, review,
// security, or release).
func isGateTask(task *models.Task) bool {
	switch task.TaskType {
	case models.TaskTypeBuild, models.TaskTypeReview, models.TaskTypeSecurity, models.TaskTypeRelease:
		return true
	default:
		return false
	}
}

// detectGateFailure checks whether a gate task completion was a failure by
// looking for remediation code tasks (code-type tasks in a non-terminal
// state) for the same story. Returns true if a gate failure is detected.
func (d *Dispatcher) detectGateFailure(ctx context.Context, gateTask *models.Task) bool {
	tasks, err := d.tasks.GetByStory(ctx, gateTask.StoryID)
	if err != nil {
		slog.Error("dispatcher: failed to get story tasks for gate failure detection",
			"story_id", gateTask.StoryID, "error", err)
		return false
	}

	for _, t := range tasks {
		// A remediation task is a code-type task that is not Done or Canceled.
		if t.TaskType == models.TaskTypeCode &&
			t.Status != models.StatusDone &&
			t.Status != models.StatusCancelled {
			return true
		}
	}
	return false
}

// handleTaskBlocked processes a task-blocked event. When a task is blocked,
// we log the state change and re-evaluate dependencies to see if the task
// can be immediately unblocked.
func (d *Dispatcher) handleTaskBlocked(ctx context.Context, event Event) {
	if event.TaskID == "" {
		return
	}

	// Re-check whether all deps are now satisfied (task may already be unblocked).
	d.tryUnblockTask(ctx, event.TaskID)

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
		d.tryUnblockTask(ctx, event.TaskID)
	}
}

// tryUnblockTask checks whether all blockers for a task have been resolved,
// and if so, transitions the task to Ready.
func (d *Dispatcher) tryUnblockTask(ctx context.Context, taskID string) {
	blockers, err := d.tasks.GetBlockers(ctx, taskID)
	if err != nil {
		slog.Error("dispatcher: failed to get blockers", "task_id", taskID, "error", err)
		return
	}
	if len(blockers) > 0 {
		return
	}
	if err := d.tasks.UpdateStatus(ctx, taskID, models.StatusReady); err != nil {
		slog.Error("dispatcher: failed to unblock task", "task_id", taskID, "error", err)
		return
	}
	d.logActivity(ctx, taskID, string(models.WorkItemTypeTask), "unblocked", "", d.resolveProjectIDFromTask(ctx, taskID))
	d.hub.Broadcast(EventTaskCompleted, map[string]string{
		"task_id": taskID,
		"status":  string(models.StatusReady),
	})
}

// logActivity is a helper that logs an activity entry and logs any error.
// The projectID argument scopes the entry to a project so activity can be
// filtered by project without joining through the work item.
func (d *Dispatcher) logActivity(ctx context.Context, workItemID, workItemType, action, details, projectID string) {
	entry := &models.ActivityLogEntry{
		WorkItemID:   workItemID,
		WorkItemType: models.WorkItemType(workItemType),
		Action:       action,
		Details:      details,
		ProjectID:    projectID,
	}
	if err := d.activities.Log(ctx, entry); err != nil {
		slog.Error("dispatcher: failed to log activity",
			"work_item_id", workItemID,
			"action", action,
			"error", err)
	}
}

// broadcastDispatcherStatus gathers the current dispatcher runtime state and
// broadcasts it to all connected WebSocket clients via the hub. If the hub is
// nil (e.g., MCP-only mode), the method is a no-op.
func (d *Dispatcher) broadcastDispatcherStatus(ctx context.Context) {
	if d.hub == nil {
		return
	}

	s := d.Status()

	type dispatcherStatusBroadcast struct {
		Running            bool             `json:"running"`
		StartedAt          string           `json:"started_at"`
		UptimeSeconds      float64          `json:"uptime_seconds"`
		EventQueueDepth    int              `json:"event_queue_depth"`
		EventsProcessed    map[string]int64 `json:"events_processed"`
		ReadyTasks         int              `json:"ready_tasks"`
		ActiveSessions     int              `json:"active_sessions"`
		PendingBuildGates  int              `json:"pending_build_gates"`
		PendingReviewGates int              `json:"pending_review_gates"`
		StaleSessions      int              `json:"stale_sessions"`
		LastAssignPass     *string          `json:"last_assign_pass"`
		LastStalenessCheck *string          `json:"last_staleness_check"`
	}

	payload := dispatcherStatusBroadcast{
		Running:            s.Running,
		StartedAt:          s.StartedAt.Format(time.RFC3339),
		UptimeSeconds:      s.Uptime.Seconds(),
		EventQueueDepth:    s.EventQueueDepth,
		EventsProcessed:    s.EventsProcessed,
		ReadyTasks:         s.ReadyTasks,
		ActiveSessions:     s.ActiveSessions,
		PendingBuildGates:  s.PendingBuildGates,
		PendingReviewGates: s.PendingReviewGates,
		StaleSessions:      s.StaleSessions,
	}

	if s.LastAssignPass != nil {
		str := s.LastAssignPass.Format(time.RFC3339)
		payload.LastAssignPass = &str
	}
	if s.LastStalenessCheck != nil {
		str := s.LastStalenessCheck.Format(time.RFC3339)
		payload.LastStalenessCheck = &str
	}

	d.hub.Broadcast(EventDispatcherStatus, payload)
}

// resolveProjectIDFromTask performs the two-hop lookup (task → story →
// project) and returns the project_id for the given task. Returns an empty
// string if the task or its story cannot be found, or if the story has no
// project set. Errors from the underlying store calls are logged but
// suppressed so the caller can safely use the result for activity logging
// even when the lookup fails.
func (d *Dispatcher) resolveProjectIDFromTask(ctx context.Context, taskID string) string {
	if taskID == "" {
		return ""
	}
	task, err := d.tasks.GetByID(ctx, taskID)
	if err != nil {
		slog.Debug("dispatcher: failed to load task for project_id lookup",
			"task_id", taskID, "error", err)
		return ""
	}
	if task == nil || task.StoryID == "" {
		return ""
	}
	story, err := d.stories.GetByID(ctx, task.StoryID)
	if err != nil {
		slog.Debug("dispatcher: failed to load story for project_id lookup",
			"task_id", taskID, "story_id", task.StoryID, "error", err)
		return ""
	}
	if story == nil {
		return ""
	}
	return story.ProjectID
}
