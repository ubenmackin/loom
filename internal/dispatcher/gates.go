package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

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

// createBuildTask creates a new Build gate task for the given story, with
// dependencies on all Done code tasks. It returns an error if the task
// creation fails.
func (d *Dispatcher) createBuildTask(ctx context.Context, story *models.Story, existingTasks []*models.Task) error {
	buildTask := &models.Task{
		StoryID:   story.ID,
		Title:     fmt.Sprintf("Build: %s", story.Title),
		Status:    models.StatusReady,
		Priority:  0,
		TaskType:  models.TaskTypeBuild,
		SortOrder: 9000, // Gate tasks sort after regular tasks.
	}

	if err := d.tasks.Create(ctx, buildTask); err != nil {
		return fmt.Errorf("create build task: %w", err)
	}

	// Add dependencies on all Done code tasks.
	for _, t := range existingTasks {
		if t.TaskType != models.TaskTypeBuild && t.TaskType != models.TaskTypeReview && t.Status == models.StatusDone {
			if err := d.tasks.AddDependency(ctx, buildTask.ID, t.ID); err != nil {
				slog.Error("dispatcher: failed to add dependency on build task",
					"build_task_id", buildTask.ID, "depends_on", t.ID, "error", err)
			}
		}
	}

	// Assemble prompt instructions for the build task.
	instructions, err := d.assemblePrompt(ctx, buildTask, story)
	if err != nil {
		slog.Error("dispatcher: failed to assemble build prompt",
			"task_id", buildTask.ID, "error", err)
	} else {
		buildTask.Instructions = instructions
		if err := d.tasks.Update(ctx, buildTask); err != nil {
			slog.Error("dispatcher: failed to update build task with instructions",
				"task_id", buildTask.ID, "error", err)
		}
	}

	details, _ := json.Marshal(map[string]string{"story_id": story.ID, "task_type": "build"})
	d.logActivity(ctx, buildTask.ID, string(models.WorkItemTypeTask), "gate_created", string(details))

	d.hub.Broadcast("gate_task_created", map[string]string{
		"task_id":   buildTask.ID,
		"story_id":  story.ID,
		"task_type": string(models.TaskTypeBuild),
		"status":    string(models.StatusReady),
	})

	slog.Info("dispatcher: created build gate task",
		"task_id", buildTask.ID, "story_id", story.ID)
	return nil
}

// createReviewTask creates a new Review gate task for the given story,
// optionally depending on the Build task.
func (d *Dispatcher) createReviewTask(ctx context.Context, story *models.Story, buildTask *models.Task) {
	reviewTask := &models.Task{
		StoryID:   story.ID,
		Title:     fmt.Sprintf("Review: %s", story.Title),
		Status:    models.StatusReady,
		Priority:  0,
		TaskType:  models.TaskTypeReview,
		SortOrder: 9100, // Review tasks sort after build tasks.
	}

	if err := d.tasks.Create(ctx, reviewTask); err != nil {
		slog.Error("dispatcher: failed to create review task",
			"story_id", story.ID, "error", err)
		return
	}

	// Add dependency on the Build task if one exists.
	if buildTask != nil {
		if err := d.tasks.AddDependency(ctx, reviewTask.ID, buildTask.ID); err != nil {
			slog.Error("dispatcher: failed to add dependency on review task",
				"review_task_id", reviewTask.ID, "depends_on", buildTask.ID, "error", err)
		}
	}

	// Assemble prompt instructions for the review task.
	instructions, err := d.assemblePrompt(ctx, reviewTask, story)
	if err != nil {
		slog.Error("dispatcher: failed to assemble review prompt",
			"task_id", reviewTask.ID, "error", err)
	} else {
		reviewTask.Instructions = instructions
		if err := d.tasks.Update(ctx, reviewTask); err != nil {
			slog.Error("dispatcher: failed to update review task with instructions",
				"task_id", reviewTask.ID, "error", err)
		}
	}

	details, _ := json.Marshal(map[string]string{"story_id": story.ID, "task_type": "review"})
	d.logActivity(ctx, reviewTask.ID, string(models.WorkItemTypeTask), "gate_created", string(details))

	d.hub.Broadcast("gate_task_created", map[string]string{
		"task_id":   reviewTask.ID,
		"story_id":  story.ID,
		"task_type": string(models.TaskTypeReview),
		"status":    string(models.StatusReady),
	})

	slog.Info("dispatcher: created review gate task",
		"task_id", reviewTask.ID, "story_id", story.ID)
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

		details, _ := json.Marshal(map[string]string{
			"resolved_by": completedTaskID,
		})
		d.logActivity(ctx, dep.ID, string(models.WorkItemTypeTask), "unblocked", string(details))

		slog.Info("dispatcher: resolved dependency, task unblocked",
			"task_id", dep.ID, "resolved_by", completedTaskID)
	}
}
