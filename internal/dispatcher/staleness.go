package dispatcher

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/ubenmackin/loom/internal/models"
)

// checkStaleness identifies sessions that have not been seen within the
// staleness threshold and flags them along with their assigned tasks.
func (d *Dispatcher) checkStaleness(ctx context.Context) {
	staleSessions, err := d.sessions.GetStaleSessions(ctx, d.stalenessThreshold)
	if err != nil {
		slog.Error("dispatcher: failed to get stale sessions", "error", err)
		return
	}

	if len(staleSessions) == 0 {
		return
	}

	slog.Info("dispatcher: detected stale sessions", "count", len(staleSessions))

	for _, session := range staleSessions {
		// Flag session as stale.
		if err := d.sessions.FlagStale(ctx, session.ID); err != nil {
			slog.Error("dispatcher: failed to flag session as stale",
				"session_id", session.ID, "error", err)
			continue
		}

		d.hub.Broadcast("session_stale", map[string]string{
			"session_id": session.ID,
			"status":     string(models.SessionStatusStale),
		})

		// Get all tasks assigned to this stale session and mark them stale.
		tasks, err := d.sessions.GetTasksForSession(ctx, session.ID)
		if err != nil {
			slog.Error("dispatcher: failed to get tasks for stale session",
				"session_id", session.ID, "error", err)
			continue
		}

		for _, task := range tasks {
			if task.IsStale {
				continue // Already flagged.
			}

			task.IsStale = true
			if err := d.tasks.Update(ctx, task); err != nil {
				slog.Error("dispatcher: failed to mark task as stale",
					"task_id", task.ID, "session_id", session.ID, "error", err)
				continue
			}

			taskDetails, _ := json.Marshal(map[string]string{
				"session_id": session.ID,
				"reason":     "session_stale",
			})
			d.logActivity(ctx, task.ID, string(models.WorkItemTypeTask), "marked_stale", string(taskDetails))

			d.hub.Broadcast("task_stale", map[string]string{
				"task_id":    task.ID,
				"session_id": session.ID,
				"is_stale":   "true",
			})
		}
	}
}
