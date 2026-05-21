package api

import (
	"net/http"

	"github.com/ubenmackin/loom/internal/models"
	"github.com/ubenmackin/loom/internal/store"
	"github.com/ubenmackin/loom/internal/ws"
)

// BoardState is the response structure for the full board state.
type BoardState struct {
	Stories       []*models.Story           `json:"stories"`
	TasksByStatus map[string][]*models.Task `json:"tasks_by_status"`
	Stats         BoardStats                `json:"stats"`
}

// BoardStats holds aggregate counts for the board.
type BoardStats struct {
	TotalStories    int `json:"total_stories"`
	TotalTasks      int `json:"total_tasks"`
	ReadyTasks      int `json:"ready_tasks"`
	InProgressTasks int `json:"in_progress_tasks"`
	BlockedTasks    int `json:"blocked_tasks"`
	DoneTasks       int `json:"done_tasks"`
	StaleTasks      int `json:"stale_tasks"`
}

// GetBoard handles GET /api/board and returns the full board state.
func (h *handlers) GetBoard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Fetch all stories.
	stories, err := h.stories.List(ctx, store.StoryFilter{})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch stories: "+err.Error())
		return
	}
	if stories == nil {
		stories = []*models.Story{}
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

	// Group tasks by status.
	tasksByStatus := make(map[string][]*models.Task)
	stats := BoardStats{
		TotalStories: len(stories),
		TotalTasks:   len(tasks),
	}

	for _, task := range tasks {
		tasksByStatus[task.Status] = append(tasksByStatus[task.Status], task)

		switch task.Status {
		case models.StatusReady:
			stats.ReadyTasks++
		case models.StatusInProgress:
			stats.InProgressTasks++
		case models.StatusBlocked:
			stats.BlockedTasks++
		case models.StatusDone:
			stats.DoneTasks++
		}

		if task.IsStale {
			stats.StaleTasks++
		}
	}

	// Ensure all status keys exist in the map (even if empty).
	for _, status := range []string{
		models.StatusNew,
		models.StatusReady,
		models.StatusInProgress,
		models.StatusBlocked,
		models.StatusDone,
	} {
		if _, ok := tasksByStatus[status]; !ok {
			tasksByStatus[status] = []*models.Task{}
		}
	}

	respondJSON(w, http.StatusOK, BoardState{
		Stories:       stories,
		TasksByStatus: tasksByStatus,
		Stats:         stats,
	})
}

// HandleWebSocket returns an http.HandlerFunc that delegates to the WebSocket hub.
func (h *handlers) HandleWebSocket(hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hub.ServeHTTP(w, r)
	}
}
