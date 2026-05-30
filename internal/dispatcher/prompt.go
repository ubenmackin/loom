package dispatcher

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/ubenmackin/loom/internal/models"
)

// mustachePattern matches {{key}} or {{key.nested}} placeholders.
var mustachePattern = regexp.MustCompile(`\{\{([\w.]+)\}\}`)

// assemblePrompt builds the instructions string for a task by loading the
// template for the task type and resolving mustache-style placeholders.
//
// Placeholders supported:
//   - {{story.title}}            → story.Title
//   - {{story.description}}      → story.Description
//   - {{task.title}}             → task.Title
//   - {{task.description}}       → task.Description
//   - {{last_build_comment}}     → most recent build-related comment on the task
//   - {{last_review_comment}}    → most recent review-related comment on the task
//   - {{unread_comments}}        → unread human-authored comments for the session
func (d *Dispatcher) assemblePrompt(ctx context.Context, task *models.Task, story *models.Story, sessionID string) (string, error) {
	// Compute unread comments early so we can pass them to defaultPrompt if needed.
	unreadSection := d.getUnreadHumanComments(ctx, task.ID, sessionID)

	tmpl, err := d.templates.GetByTaskType(ctx, task.TaskType)
	if err != nil {
		slog.Info("dispatcher: no template found for task type, using default prompt",
			"task_type", task.TaskType, "error", err)
		return defaultPrompt(task, story, unreadSection), nil
	}

	result := tmpl.Template

	// Build a lookup table for placeholder resolution.
	values := make(map[string]string)

	// Story fields.
	if story != nil {
		values["story.title"] = story.Title
		values["story.description"] = story.Description
	}

	// Task fields.
	values["task.title"] = task.Title
	values["task.description"] = task.Description

	// Last build comment.
	lastBuildComment, err := d.findLastComment(ctx, task.ID, "build")
	if err != nil {
		slog.Debug("dispatcher: could not find last build comment",
			"task_id", task.ID, "error", err)
	}
	values["last_build_comment"] = lastBuildComment

	// Last review comment.
	lastReviewComment, err := d.findLastComment(ctx, task.ID, "review")
	if err != nil {
		slog.Debug("dispatcher: could not find last review comment",
			"task_id", task.ID, "error", err)
	}
	values["last_review_comment"] = lastReviewComment

	// Unread human comments.
	values["unread_comments"] = unreadSection

	// Resolve all mustache placeholders.
	result = mustachePattern.ReplaceAllStringFunc(result, func(match string) string {
		key := mustachePattern.FindStringSubmatch(match)[1]
		if val, ok := values[key]; ok {
			return val
		}
		// Leave unresolved placeholders as-is.
		return match
	})

	return result, nil
}

// findLastComment retrieves the most recent comment on a task that contains
// the given keyword in its body (case-insensitive). Returns an empty string
// if no matching comment is found.
func (d *Dispatcher) findLastComment(ctx context.Context, taskID string, keyword string) (string, error) {
	comments, err := d.comments.GetByWorkItem(ctx, taskID, models.WorkItemTypeTask)
	if err != nil {
		return "", fmt.Errorf("get comments for task %q: %w", taskID, err)
	}

	// Comments are returned in ascending order by created_at, so iterate
	// in reverse to find the most recent match.
	lowerKeyword := strings.ToLower(keyword)
	for i := len(comments) - 1; i >= 0; i-- {
		if strings.Contains(strings.ToLower(comments[i].Body), lowerKeyword) {
			return comments[i].Body, nil
		}
	}

	return "", nil
}

// getUnreadHumanComments fetches unread human-authored comments for the given
// session and formats them as a markdown section. Returns an empty string if
// no unread human comments exist or if sessionID is empty (e.g. for gate tasks).
func (d *Dispatcher) getUnreadHumanComments(ctx context.Context, taskID, sessionID string) string {
	if sessionID == "" {
		return ""
	}

	// Fetch unread comments on tasks assigned to this session.
	unreads, err := d.comments.GetUnreadForSession(ctx, sessionID)
	if err != nil {
		slog.Debug("dispatcher: failed to get unread comments", "session_id", sessionID, "error", err)
	}

	// Also fetch unread comments on the parent story.
	// We need the story ID from the task.
	var storyUnreads []*models.Comment
	task, tErr := d.tasks.GetByID(ctx, taskID)
	if tErr == nil && task.StoryID != "" {
		su, sErr := d.comments.GetUnreadForSessionByWorkItem(ctx, sessionID, task.StoryID, models.WorkItemTypeStory)
		if sErr == nil {
			storyUnreads = su
		} else {
			slog.Debug("dispatcher: failed to get unread story comments", "session_id", sessionID, "story_id", task.StoryID, "error", sErr)
		}
	}

	// Combine both sources and filter to human-authored.
	var sb strings.Builder
	sb.WriteString("\n## Unread Comments\n\n")

	added := false
	for _, c := range unreads {
		if c.AuthorType == models.AuthorTypeHuman {
			fmt.Fprintf(&sb, "**%s**: %s\n\n", c.AuthorID, c.Body)
			added = true
		}
	}
	for _, c := range storyUnreads {
		if c.AuthorType == models.AuthorTypeHuman {
			fmt.Fprintf(&sb, "**%s**: %s\n\n", c.AuthorID, c.Body)
			added = true
		}
	}

	if !added {
		return ""
	}
	return sb.String()
}

// defaultPrompt returns a basic prompt when no template is found for the
// given task type.
func defaultPrompt(task *models.Task, story *models.Story, unreadSection string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "# Task: %s\n\n", task.Title)

	if task.Description != "" {
		sb.WriteString(task.Description)
		sb.WriteString("\n\n")
	}

	if story != nil {
		fmt.Fprintf(&sb, "## Story: %s\n\n", story.Title)
		if story.Description != "" {
			sb.WriteString(story.Description)
			sb.WriteString("\n")
		}
	}

	if unreadSection != "" {
		sb.WriteString(unreadSection)
	}

	return sb.String()
}
