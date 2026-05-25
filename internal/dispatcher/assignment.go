package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
)

// runAssignmentPass finds the best available session for each unassigned
// Ready task. Tasks are processed in the order returned by List,
// so that all eligible tasks are assigned in a single pass.
func (d *Dispatcher) runAssignmentPass(ctx context.Context) {
	d.hub.Broadcast("dispatcher_event", map[string]string{
		"type":      "assignment_pass_started",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})

	readyTasks, err := d.tasks.List(ctx, store.TaskFilter{Status: models.StatusReady})
	if err != nil {
		slog.Error("dispatcher: failed to list ready tasks", "error", err)
		return
	}

	// Batch fetch blockers for all unassigned tasks.
	taskIDs := make([]string, 0, len(readyTasks))
	for _, t := range readyTasks {
		if t.AssignedTo == "" {
			taskIDs = append(taskIDs, t.ID)
		}
	}

	blockerMap := make(map[string][]string)
	if len(taskIDs) > 0 {
		blockerMap, err = d.tasks.GetBlockersForTasks(ctx, taskIDs)
		if err != nil {
			slog.Error("dispatcher: failed to batch fetch blockers", "error", err)
			return
		}
	}

	for _, task := range readyTasks {
		// Skip tasks that are already assigned.
		if task.AssignedTo != "" {
			continue
		}

		// Check that all dependencies are satisfied.
		blockers := blockerMap[task.ID]
		if len(blockers) > 0 {
			// Task has unresolved blockers; transition to blocked if not already.
			if task.Status != models.StatusBlocked {
				if err := d.tasks.UpdateStatus(ctx, task.ID, models.StatusBlocked); err != nil {
					slog.Error("dispatcher: failed to block task",
						"task_id", task.ID, "error", err)
				}
			}
			continue
		}

		// Find the best available session for this task type.
		session, err := d.findBestSession(ctx, task.TaskType)
		if err != nil {
			slog.Error("dispatcher: failed to find best session",
				"task_id", task.ID, "task_type", task.TaskType, "error", err)
			continue
		}
		if session == nil {
			// No capable session available — skip for now.
			continue
		}

		if err := d.assignTaskToSession(ctx, task, session); err != nil {
			slog.Error("dispatcher: failed to assign task to session",
				"task_id", task.ID, "session_id", session.ID, "error", err)
		}
	}

	d.hub.Broadcast("dispatcher_event", map[string]string{
		"type":      "assignment_pass_finished",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// assignWorkToSession finds the best available Ready task for a specific
// session that has requested work (used by the async event path).
// It discards the result — the dispatcher event loop only needs side effects.
func (d *Dispatcher) assignWorkToSession(ctx context.Context, session *models.Session) {
	_, _ = d.findAndAssignTaskForSession(ctx, session)
}

// findAndAssignTaskForSession finds and assigns the best available Ready task
// for a specific session. Returns the assigned task or nil if no work is
// available. This is the shared implementation used by both the synchronous
// AssignWork API and the async event path.
func (d *Dispatcher) findAndAssignTaskForSession(ctx context.Context, session *models.Session) (*models.Task, error) {
	readyTasks, err := d.tasks.List(ctx, store.TaskFilter{Status: models.StatusReady})
	if err != nil {
		return nil, fmt.Errorf("list ready tasks: %w", err)
	}

	// Parse session capabilities.
	var caps []string
	if session.Capabilities != "" {
		if err := json.Unmarshal([]byte(session.Capabilities), &caps); err != nil {
			return nil, fmt.Errorf("parse session capabilities for %q: %w", session.ID, err)
		}
	}
	capSet := make(map[string]bool, len(caps))
	for _, c := range caps {
		capSet[c] = true
	}

	// Batch fetch blockers for all unassigned tasks.
	taskIDs := make([]string, 0, len(readyTasks))
	for _, t := range readyTasks {
		if t.AssignedTo == "" {
			taskIDs = append(taskIDs, t.ID)
		}
	}

	blockerMap := make(map[string][]string)
	if len(taskIDs) > 0 {
		blockerMap, err = d.tasks.GetBlockersForTasks(ctx, taskIDs)
		if err != nil {
			return nil, fmt.Errorf("batch fetch blockers: %w", err)
		}
	}

	for _, task := range readyTasks {
		// Skip already-assigned tasks.
		if task.AssignedTo != "" {
			continue
		}

		// Check capability match.
		if !capSet[string(task.TaskType)] {
			continue
		}

		// Check that all dependencies are satisfied.
		if len(blockerMap[task.ID]) > 0 {
			continue
		}

		if err := d.assignTaskToSession(ctx, task, session); err != nil {
			return nil, fmt.Errorf("assign task %q to session %q: %w", task.ID, session.ID, err)
		}

		// Return the freshly assigned task (re-read to get updated fields).
		assigned, err := d.tasks.GetByID(ctx, task.ID)
		if err != nil {
			return nil, fmt.Errorf("re-read assigned task %q: %w", task.ID, err)
		}
		return assigned, nil
	}

	return nil, nil
}

// findBestSession returns the active session with the matching capability
// that has the fewest currently assigned tasks (least loaded). Ties are
// broken deterministically by preferring the most recently registered
// session (latest created_at), so that new sessions are favored when load
// is equal.
func (d *Dispatcher) findBestSession(ctx context.Context, taskType models.TaskType) (*models.Session, error) {
	candidates, err := d.sessions.GetByCapabilitiesWithTaskCount(ctx, string(taskType))
	if err != nil {
		return nil, fmt.Errorf("get sessions by capability %q: %w", taskType, err)
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	var best *models.Session
	bestLoad := -1

	for _, c := range candidates {
		load := c.TaskCount
		// Use less-than-or-equal so that the last session with equal load
		// wins. Because candidates are ordered by created_at ASC, the last
		// one is the most recently registered — giving deterministic
		// last-registered-wins tie-breaking.
		if bestLoad == -1 || load <= bestLoad {
			best = c.Session
			bestLoad = load
		}
	}

	return best, nil
}

// assignTaskToSession updates a task with the session assignment, changes
// status to "in_progress", assembles prompt instructions, logs the activity,
// and broadcasts a WebSocket event.
func (d *Dispatcher) assignTaskToSession(ctx context.Context, task *models.Task, session *models.Session) error {
	task.AssignedTo = session.ID
	task.AssigneeType = models.AssigneeTypeSession
	task.Status = models.StatusInProgress

	// Assemble prompt instructions for the assigned session.
	story, err := d.stories.GetByID(ctx, task.StoryID)
	if err != nil {
		slog.Warn("dispatcher: assembling prompt with degraded content",
			"task_id", task.ID, "error", err)
		task.Instructions = defaultPrompt(task, nil)
	} else {
		instructions, err := d.assemblePrompt(ctx, task, story)
		if err != nil {
			slog.Warn("dispatcher: assembling prompt with degraded content",
				"task_id", task.ID, "error", err)
			task.Instructions = defaultPrompt(task, story)
		} else {
			task.Instructions = instructions
		}
	}

	if err := d.tasks.Update(ctx, task); err != nil {
		return fmt.Errorf("update task %q for assignment: %w", task.ID, err)
	}

	details, _ := json.Marshal(map[string]string{
		"session_id": session.ID,
		"task_type":  string(task.TaskType),
	})
	d.logActivity(ctx, task.ID, string(models.WorkItemTypeTask), "assigned", string(details))

	d.hub.Broadcast("task_assigned", map[string]string{
		"task_id":    task.ID,
		"session_id": session.ID,
		"status":     string(models.StatusInProgress),
	})

	slog.Info("dispatcher: assigned task to session",
		"task_id", task.ID, "session_id", session.ID)

	return nil
}
