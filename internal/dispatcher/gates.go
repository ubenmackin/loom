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

	tasks, err := d.tasks.GetByStory(ctx, storyID)
	if err != nil {
		slog.Error("dispatcher: failed to get tasks for gate check",
			"story_id", storyID, "error", err)
		return
	}

	var hasBuildTask, hasReviewTask bool
	var buildTask *models.Task
	var allCodeTasksDone = true

	for _, t := range tasks {
		switch t.TaskType {
		case models.TaskTypeBuild:
			hasBuildTask = true
			buildTask = t
		case models.TaskTypeReview:
			hasReviewTask = true
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

	// Check if a Review gate task should be created.
	if story.RequiresReview && !hasReviewTask {
		// Review requires that the Build task (if exists) is Done, or that
		// all code tasks are Done if there is no Build task.
		// If we just created a build task above, it's in "ready" status,
		// so the review won't be created yet.
		if buildTask != nil && buildTask.Status == models.StatusDone {
			d.createReviewTask(ctx, story, buildTask)
		} else if !story.RequiresBuild && allCodeTasksDone {
			d.createReviewTask(ctx, story, nil)
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
