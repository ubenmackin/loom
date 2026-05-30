package api

import (
	"net/http"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
)

// boardState is the response structure for the full board state.
type boardState struct {
	Stories               []*models.Story                      `json:"stories"`
	TasksByStatus         map[string][]*models.Task            `json:"tasks_by_status"`
	TasksByStoryAndStatus map[string]map[string][]*models.Task `json:"tasks_by_story_and_status,omitempty"`
	Stats                 boardStats                           `json:"stats"`
}

// boardStats holds aggregate counts for the board.
type boardStats struct {
	TotalStories    int `json:"total_stories"`
	TotalTasks      int `json:"total_tasks"`
	ReadyTasks      int `json:"ready_tasks"`
	InProgressTasks int `json:"in_progress_tasks"`
	BlockedTasks    int `json:"blocked_tasks"`
	DoneTasks       int `json:"done_tasks"`
	CancelledTasks  int `json:"canceled_tasks"`
	ArchivedTasks   int `json:"archived_tasks"`
	StaleTasks      int `json:"stale_tasks"`
}

// GetBoard handles GET /api/board and returns the full board state.
func (h *handlers) GetBoard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	p := parsePagination(r, 100, 500)
	offset := p.Offset
	limit := p.Limit

	// Fetch stories with pagination.
	stories, err := h.stories.List(ctx, store.StoryFilter{})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch stories: "+err.Error())
		return
	}
	if stories == nil {
		stories = []*models.Story{}
	}

	// Apply pagination to stories.
	if offset > 0 || limit < len(stories) {
		if offset >= len(stories) {
			stories = []*models.Story{}
		} else {
			end := offset + limit
			if end > len(stories) {
				end = len(stories)
			}
			stories = stories[offset:end]
		}
	}

	// Fetch all tasks.
	tasks, err := h.tasks.List(ctx, store.TaskFilter{})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch tasks: "+err.Error())
		return
	}
	if tasks == nil {
		tasks = []*models.Task{}
	}

	// Apply pagination to tasks.
	if offset > 0 || limit < len(tasks) {
		if offset >= len(tasks) {
			tasks = []*models.Task{}
		} else {
			end := offset + limit
			if end > len(tasks) {
				end = len(tasks)
			}
			tasks = tasks[offset:end]
		}
	}

	// Group tasks by status.
	tasksByStatus := make(map[string][]*models.Task)
	stats := boardStats{
		TotalStories: len(stories),
		TotalTasks:   len(tasks),
	}

	for _, task := range tasks {
		tasksByStatus[string(task.Status)] = append(tasksByStatus[string(task.Status)], task)

		switch task.Status {
		case models.StatusReady:
			stats.ReadyTasks++
		case models.StatusInProgress:
			stats.InProgressTasks++
		case models.StatusBlocked:
			stats.BlockedTasks++
		case models.StatusDone:
			stats.DoneTasks++
		case models.StatusCancelled:
			stats.CancelledTasks++
		case models.StatusArchived:
			stats.ArchivedTasks++
		}

		if task.IsStale {
			stats.StaleTasks++
		}
	}

	// Ensure all status keys exist in the map (even if empty).
	for _, status := range models.AllStatuses() {
		if _, ok := tasksByStatus[string(status)]; !ok {
			tasksByStatus[string(status)] = []*models.Task{}
		}
	}

	// Group tasks by story and status.
	tasksByStoryAndStatus := make(map[string]map[string][]*models.Task)
	for _, task := range tasks {
		if tasksByStoryAndStatus[task.StoryID] == nil {
			tasksByStoryAndStatus[task.StoryID] = make(map[string][]*models.Task)
		}
		tasksByStoryAndStatus[task.StoryID][string(task.Status)] = append(
			tasksByStoryAndStatus[task.StoryID][string(task.Status)], task)
	}

	respondJSON(w, http.StatusOK, boardState{
		Stories:               stories,
		TasksByStatus:         tasksByStatus,
		TasksByStoryAndStatus: tasksByStoryAndStatus,
		Stats:                 stats,
	})
}
