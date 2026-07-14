package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/ubenmackin/loom/internal/models"
)

// checkGateConditions evaluates whether a story requires Build or Review
// gate tasks to be created, and creates them if appropriate.
//
// A Build task is created when:
//   - story.requires_build is true
//   - all non-gate (non-"build", non-"review") tasks for the story are Done
//   - no Build task already exists for the story
//
// A Review task is created when:
//   - story.requires_review is true
//   - the Build task (if one exists) is Done
//   - no Review task already exists for the story
func (d *Dispatcher) checkGateConditions(ctx context.Context, storyID string) {
	d.hub.Broadcast(EventDispatcherAction, map[string]string{
		"type":      EventGateCheck,
		"story_id":  storyID,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})

	story, err := d.stories.GetByID(ctx, storyID)
	if err != nil {
		slog.Error("dispatcher: failed to get story for gate check",
			"story_id", storyID, "error", err)
		return
	}

	// Circuit breaker: do not create new gates for failed stories.
	if story.Status == models.StatusFailed {
		slog.Warn("dispatcher: skipping gate check for failed story",
			"story_id", storyID)
		return
	}

	tasks, err := d.tasks.GetByStory(ctx, storyID)
	if err != nil {
		slog.Error("dispatcher: failed to get tasks for gate check",
			"story_id", storyID, "error", err)
		return
	}

	var hasBuildTask, hasReviewTask, hasSecurityTask, hasReleaseTask bool
	var buildTask, securityTask, reviewTask *models.Task
	var allCodeTasksDone = true

	for _, t := range tasks {
		switch t.TaskType {
		case models.TaskTypeBuild:
			hasBuildTask = true
			buildTask = t
		case models.TaskTypeReview:
			hasReviewTask = true
			reviewTask = t
		case models.TaskTypeSecurity:
			hasSecurityTask = true
			securityTask = t
		case models.TaskTypeRelease:
			hasReleaseTask = true
		default:
			// Non-gate tasks (code, custom, etc.)
			if t.Status != models.StatusDone {
				allCodeTasksDone = false
			}
		}
	}

	// Check if a Build gate task should be created.
	if story.RequiresBuild && !hasBuildTask && allCodeTasksDone {
		if err := d.createBuildTask(ctx, story, tasks); err != nil {
			slog.Error("dispatcher: failed to create build task",
				"story_id", story.ID, "error", err)
		}
	}

	// Check if a Security gate task should be created.
	if story.RequiresSecurity && !hasSecurityTask {
		// Security requires that the Build task (if exists) is Done
		if buildTask != nil && buildTask.Status == models.StatusDone {
			if err := d.createSecurityTask(ctx, story, buildTask); err != nil {
				slog.Error("dispatcher: failed to create security task",
					"story_id", story.ID, "error", err)
			}
		} else if !story.RequiresBuild && allCodeTasksDone {
			if err := d.createSecurityTask(ctx, story, nil); err != nil {
				slog.Error("dispatcher: failed to create security task",
					"story_id", story.ID, "error", err)
			}
		}
	}

	// Check if a Review gate task should be created.
	if story.RequiresReview && !hasReviewTask {
		// Review requires that the Build task (if exists) is Done, or that
		// all code tasks are Done if there is no Build task.
		// If we just created a build task above, it's in "ready" status,
		// so the review won't be created yet.
		// If RequiresSecurity is true, Review also waits for Security to be Done.
		if buildTask != nil && buildTask.Status == models.StatusDone {
			if !story.RequiresSecurity || (securityTask != nil && securityTask.Status == models.StatusDone) {
				d.createReviewTask(ctx, story, buildTask)
			}
		} else if !story.RequiresBuild && allCodeTasksDone {
			if !story.RequiresSecurity || (securityTask != nil && securityTask.Status == models.StatusDone) {
				d.createReviewTask(ctx, story, nil)
			}
		}
	}

	// Check if a Release gate task should be created.
	// Release requires Review to be Done.
	if !hasReleaseTask && hasReviewTask && reviewTask != nil && reviewTask.Status == models.StatusDone {
		// All non-release tasks should be done
		allOthersDone := true
		for _, t := range tasks {
			if t.TaskType != models.TaskTypeRelease && t.Status != models.StatusDone {
				allOthersDone = false
				break
			}
		}
		if allOthersDone {
			d.createReleaseTask(ctx, story, reviewTask)
		}
	}
}

// createGateTask is a shared helper that creates a gate task (build or review)
// with the given parameters, assembles prompt instructions, logs the activity,
// and broadcasts a WebSocket event.
func (d *Dispatcher) createGateTask(ctx context.Context, story *models.Story, taskType models.TaskType, titlePrefix string, sortOrder int, addDeps func(string) error) error {
	task := &models.Task{
		StoryID:   story.ID,
		Title:     fmt.Sprintf("%s: %s", titlePrefix, story.Title),
		Status:    models.StatusReady,
		TaskType:  taskType,
		SortOrder: sortOrder,
	}

	if err := d.tasks.Create(ctx, task); err != nil {
		return fmt.Errorf("create %s task: %w", taskType, err)
	}

	// Add dependencies if a callback is provided.
	if addDeps != nil {
		if err := addDeps(task.ID); err != nil {
			slog.Error("dispatcher: failed to add dependencies on gate task",
				"task_id", task.ID, "task_type", taskType, "error", err)
		}
	}

	// Assemble prompt instructions.
	instructions, err := d.assemblePrompt(ctx, task, story, "")
	if err != nil {
		slog.Error("dispatcher: failed to assemble prompt",
			"task_id", task.ID, "task_type", taskType, "error", err)
	} else {
		task.Instructions = instructions
		if err := d.tasks.Update(ctx, task); err != nil {
			slog.Error("dispatcher: failed to update task with instructions",
				"task_id", task.ID, "error", err)
		}
	}

	details, err := json.Marshal(map[string]string{"story_id": story.ID, "task_type": string(taskType)})
	if err != nil {
		slog.Error("dispatcher: failed to marshal gate task details", "error", err)
	} else {
		d.logActivity(ctx, task.ID, string(models.WorkItemTypeTask), "gate_created", string(details))
	}

	d.hub.Broadcast(EventGateTaskCreated, map[string]string{
		"task_id":   task.ID,
		"story_id":  story.ID,
		"task_type": string(taskType),
		"status":    string(models.StatusReady),
	})

	// Forward the gate task to the Gateway for ACP session creation.
	d.submitToGateway(Event{
		Type:   EventWorkRequested,
		TaskID: task.ID,
		Payload: map[string]string{
			"story_id":   story.ID,
			"project_id": story.ProjectID,
		},
	})

	slog.Info("dispatcher: created gate task",
		"task_id", task.ID, "story_id", story.ID, "task_type", taskType)

	return nil
}

// createBuildTask creates a new Build gate task for the given story, with
// dependencies on all Done code tasks. It returns an error if the task
// creation fails.
func (d *Dispatcher) createBuildTask(ctx context.Context, story *models.Story, existingTasks []*models.Task) error {
	return d.createGateTask(ctx, story, models.TaskTypeBuild, "Build", 9000, func(taskID string) error {
		for _, t := range existingTasks {
			if t.TaskType != models.TaskTypeBuild && t.TaskType != models.TaskTypeReview && t.Status == models.StatusDone {
				if err := d.tasks.AddDependency(ctx, taskID, t.ID); err != nil {
					slog.Error("dispatcher: failed to add dependency on build task",
						"build_task_id", taskID, "depends_on", t.ID, "error", err)
				}
			}
		}
		return nil
	})
}

// createReviewTask creates a new Review gate task for the given story,
// optionally depending on the Build task.
func (d *Dispatcher) createReviewTask(ctx context.Context, story *models.Story, buildTask *models.Task) {
	err := d.createGateTask(ctx, story, models.TaskTypeReview, "Review", 9100, func(taskID string) error {
		if buildTask != nil {
			if err := d.tasks.AddDependency(ctx, taskID, buildTask.ID); err != nil {
				return fmt.Errorf("add dependency on build task: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		slog.Error("dispatcher: failed to create review task",
			"story_id", story.ID, "error", err)
	}
}

// createSecurityTask creates a new Security gate task for the given story,
// optionally depending on the Build task.
func (d *Dispatcher) createSecurityTask(ctx context.Context, story *models.Story, buildTask *models.Task) error {
	return d.createGateTask(ctx, story, models.TaskTypeSecurity, "Security Audit", 9050, func(taskID string) error {
		if buildTask != nil {
			if err := d.tasks.AddDependency(ctx, taskID, buildTask.ID); err != nil {
				return fmt.Errorf("add dependency on build task: %w", err)
			}
		}
		return nil
	})
}

// createReleaseTask creates a new Release gate task for the given story,
// depending on the Review task.
func (d *Dispatcher) createReleaseTask(ctx context.Context, story *models.Story, reviewTask *models.Task) {
	err := d.createGateTask(ctx, story, models.TaskTypeRelease, "Release", 9200, func(taskID string) error {
		if reviewTask != nil {
			if err := d.tasks.AddDependency(ctx, taskID, reviewTask.ID); err != nil {
				return fmt.Errorf("add dependency on review task: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		slog.Error("dispatcher: failed to create release task",
			"story_id", story.ID, "error", err)
	}
}

// incrementFailureCount increments the story's failure_count and checks
// whether the circuit breaker should trip (≥3 failures).
// Returns true if the circuit breaker tripped (story moved to "failed").
func (d *Dispatcher) incrementFailureCount(ctx context.Context, storyID string) bool {
	story, err := d.stories.GetByID(ctx, storyID)
	if err != nil {
		slog.Error("dispatcher: failed to get story for failure count increment",
			"story_id", storyID, "error", err)
		return false
	}

	story.FailureCount++

	if err := d.stories.Update(ctx, story); err != nil {
		slog.Error("dispatcher: failed to update story failure_count",
			"story_id", storyID, "error", err)
		return false
	}

	slog.Warn("dispatcher: gate failure recorded",
		"story_id", storyID, "failure_count", story.FailureCount)

	if story.FailureCount >= 3 {
		// Circuit breaker tripped — transition story to "failed".
		if err := d.stories.UpdateStatus(ctx, storyID, models.StatusFailed); err != nil {
			slog.Error("dispatcher: failed to trip circuit breaker",
				"story_id", storyID, "error", err)
			return false
		}

		d.hub.Broadcast(EventStoryFailed, map[string]string{
			"story_id":      storyID,
			"failure_count": fmt.Sprintf("%d", story.FailureCount),
			"reason":        "Circuit breaker tripped: 3 consecutive gate failures",
		})

		slog.Error("dispatcher: circuit breaker tripped",
			"story_id", storyID,
			"failure_count", story.FailureCount,
			"reason", "3 consecutive gate failures")

		return true
	}

	return false
}

// resolveDependencies finds all tasks that depend on the just-completed task
// and attempts to unblock any that are no longer blocked.
func (d *Dispatcher) resolveDependencies(ctx context.Context, completedTaskID string) {
	dependents, err := d.tasks.GetDependents(ctx, completedTaskID)
	if err != nil {
		slog.Error("dispatcher: failed to get dependents for completed task",
			"task_id", completedTaskID, "error", err)
		return
	}

	for _, dep := range dependents {
		if dep.Status != models.StatusBlocked {
			continue
		}

		d.tryUnblockTask(ctx, dep.ID)

		details, err := json.Marshal(map[string]string{
			"resolved_by": completedTaskID,
		})
		if err != nil {
			slog.Error("dispatcher: failed to marshal resolution details", "error", err)
		} else {
			d.logActivity(ctx, dep.ID, string(models.WorkItemTypeTask), "unblocked", string(details))
		}

		slog.Info("dispatcher: resolved dependency, task unblocked",
			"task_id", dep.ID, "resolved_by", completedTaskID)
	}
}

// checkStoryCompletion evaluates whether all tasks for a story are
// done and all gates have passed. If so, transitions the story to "completed".
// Stories in "failed" status are skipped — they have tripped the circuit breaker.
func (d *Dispatcher) checkStoryCompletion(ctx context.Context, storyID string) {
	// Get the story first to check its status.
	story, err := d.stories.GetByID(ctx, storyID)
	if err != nil {
		slog.Error("dispatcher: failed to get story for completion check",
			"story_id", storyID, "error", err)
		return
	}

	// Circuit breaker: do not mark failed stories as completed.
	if story.Status == models.StatusFailed {
		return
	}

	// Get all tasks for this story
	tasks, err := d.tasks.GetByStory(ctx, storyID)
	if err != nil {
		slog.Error("dispatcher: failed to get tasks for story completion check",
			"story_id", storyID, "error", err)
		return
	}

	// Check if a Release task exists and is Done — that is the final gate.
	var hasReleaseTask bool
	var releaseTask *models.Task
	for _, t := range tasks {
		if t.TaskType == models.TaskTypeRelease {
			hasReleaseTask = true
			releaseTask = t
			break
		}
	}

	if hasReleaseTask {
		// If a Release task exists, only transition to completed when it is Done.
		if releaseTask.Status != models.StatusDone {
			return
		}
	} else {
		// No Release task exists — fall back to checking all tasks (backward compatibility).
		for _, task := range tasks {
			if task.Status != models.StatusDone && task.Status != models.StatusCancelled {
				return // Not all tasks are done
			}
		}
	}

	// All tasks are done/canceled — transition story to completed
	if err := d.stories.UpdateStatus(ctx, storyID, models.StatusCompleted); err != nil {
		slog.Error("dispatcher: failed to mark story completed",
			"story_id", storyID, "error", err)
		return
	}

	slog.Info("dispatcher: story auto-completed",
		"story_id", storyID)

	d.hub.Broadcast("story_completed", map[string]string{
		"story_id": storyID,
	})

	// Forward the completion to the Gateway so it can clean up the story's
	// git worktree. Unlike the websocket broadcast above, this event flows
	// through the gateway's event loop.
	d.submitToGateway(Event{
		Type: EventStoryCompleted,
		Payload: map[string]string{
			"story_id": storyID,
		},
	})
}
