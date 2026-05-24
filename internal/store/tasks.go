package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ubenmackin/loom/internal/models"
)

// TaskFilter holds optional criteria for listing tasks.
type TaskFilter struct {
	StoryID    string
	Status     models.Status
	AssignedTo string
	TaskType   models.TaskType
}

// TaskStore provides CRUD operations for tasks.
type TaskStore struct {
	db *sql.DB
}

// NewTaskStore creates a new TaskStore.
func NewTaskStore(db *sql.DB) *TaskStore {
	return &TaskStore{db: db}
}

// scanTask is a helper to scan a task row from a *sql.Row or *sql.Rows.
func scanTask(scanner interface{ Scan(...any) error }) (*models.Task, error) {
	t := &models.Task{}
	var desc, assignedTo, instructions sql.NullString
	var statusStr, taskTypeStr, assigneeTypeStr sql.NullString
	var createdAt, updatedAt sql.NullTime
	var numericID sql.NullInt64

	err := scanner.Scan(
		&t.ID, &numericID, &t.StoryID, &t.Title, &desc, &statusStr, &taskTypeStr,
		&assignedTo, &assigneeTypeStr, &t.SortOrder,
		&instructions, &t.IsStale, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	t.Description = stringOrZero(desc)
	t.AssignedTo = stringOrZero(assignedTo)
	t.AssigneeType = models.AssigneeType(stringOrZero(assigneeTypeStr))
	t.Status = models.Status(stringOrZero(statusStr))
	t.TaskType = models.TaskType(stringOrZero(taskTypeStr))
	t.Instructions = stringOrZero(instructions)
	t.NumericID = intOrZero(numericID)
	t.CreatedAt = timeOrZero(createdAt)
	t.UpdatedAt = timeOrZero(updatedAt)

	return t, nil
}

// Create inserts a new task. If the ID is empty, it is auto-generated.
// It mutates the pointer to set ID, NumericID, CreatedAt, and UpdatedAt.
func (s *TaskStore) Create(ctx context.Context, t *models.Task) error {
	res, err := s.db.ExecContext(ctx, "INSERT INTO work_item_sequence (type) VALUES ('task')")
	if err != nil {
		return fmt.Errorf("generate task id: %w", err)
	}
	seqID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id for task: %w", err)
	}
	if t.ID == "" {
		t.ID = fmt.Sprintf("TASK-%06d", seqID)
	}
	t.NumericID = int(seqID)

	now := time.Now().UTC()
	t.CreatedAt = now
	t.UpdatedAt = now
	if t.Status == "" {
		t.Status = models.StatusNew
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO tasks (id, numeric_id, story_id, title, description, status, task_type,
		 assigned_to, assignee_type, sort_order, instructions, is_stale, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.NumericID, t.StoryID, t.Title, t.Description, t.Status, t.TaskType,
		t.AssignedTo, t.AssigneeType, t.SortOrder,
		t.Instructions, t.IsStale, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}
	return nil
}

// GetByID retrieves a task by its ID.
func (s *TaskStore) GetByID(ctx context.Context, id string) (*models.Task, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, numeric_id, story_id, title, description, status, task_type,
		        assigned_to, assignee_type, sort_order, instructions, is_stale, created_at, updated_at
		 FROM tasks WHERE id = ?`, id)

	t, err := scanTask(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("task %q: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("query task %q: %w", id, err)
	}
	return t, nil
}

// GetByNumericID retrieves a task by its numeric ID.
func (s *TaskStore) GetByNumericID(ctx context.Context, numID int) (*models.Task, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, numeric_id, story_id, title, description, status, task_type,
		        assigned_to, assignee_type, sort_order, instructions, is_stale, created_at, updated_at
		 FROM tasks WHERE numeric_id = ?`, numID)

	t, err := scanTask(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("task with numeric id %d: %w", numID, ErrNotFound)
		}
		return nil, fmt.Errorf("query task by numeric id %d: %w", numID, err)
	}
	return t, nil
}

// List returns tasks matching the given filter.
func (s *TaskStore) List(ctx context.Context, filter TaskFilter) ([]*models.Task, error) {
	var conditions []string
	var args []any

	if filter.StoryID != "" {
		conditions = append(conditions, "story_id = ?")
		args = append(args, filter.StoryID)
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.AssignedTo != "" {
		conditions = append(conditions, "assigned_to = ?")
		args = append(args, filter.AssignedTo)
	}
	if filter.TaskType != "" {
		conditions = append(conditions, "task_type = ?")
		args = append(args, filter.TaskType)
	}

	query := `SELECT id, numeric_id, story_id, title, description, status, task_type,
	                 assigned_to, assignee_type, sort_order, instructions, is_stale, created_at, updated_at
	          FROM tasks`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY sort_order, created_at"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("rows close error: %v", err)
		}
	}()

	var tasks []*models.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}

		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}

	return tasks, nil
}

// Update saves all mutable fields of a task.
func (s *TaskStore) Update(ctx context.Context, t *models.Task) error {
	t.UpdatedAt = time.Now().UTC()

	result, err := s.db.ExecContext(ctx,
		`UPDATE tasks SET story_id=?, title=?, description=?, status=?, task_type=?,
		 assigned_to=?, assignee_type=?, sort_order=?, instructions=?,
		 is_stale=?, updated_at=?
		 WHERE id=?`,
		t.StoryID, t.Title, t.Description, t.Status, t.TaskType,
		t.AssignedTo, t.AssigneeType, t.SortOrder, t.Instructions,
		t.IsStale, t.UpdatedAt, t.ID,
	)
	if err != nil {
		return fmt.Errorf("update task %q: %w", t.ID, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected task %q: %w", t.ID, err)
	}
	if rows == 0 {
		return fmt.Errorf("task %q: %w", t.ID, ErrNotFound)
	}

	return nil
}

// BatchUpdate applies updates to multiple tasks in a single transaction.
func (s *TaskStore) BatchUpdate(ctx context.Context, tasks []*models.Task) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	for _, t := range tasks {
		t.UpdatedAt = time.Now().UTC()

		result, execErr := tx.ExecContext(ctx,
			`UPDATE tasks SET story_id=?, title=?, description=?, status=?, task_type=?,
			 assigned_to=?, assignee_type=?, sort_order=?, instructions=?,
			 is_stale=?, updated_at=?
			 WHERE id=?`,
			t.StoryID, t.Title, t.Description, t.Status, t.TaskType,
			t.AssignedTo, t.AssigneeType, t.SortOrder, t.Instructions,
			t.IsStale, t.UpdatedAt, t.ID,
		)
		if execErr != nil {
			err = fmt.Errorf("update task %q in batch: %w", t.ID, execErr)
			return err
		}

		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			err = fmt.Errorf("rows affected task %q in batch: %w", t.ID, rowsErr)
			return err
		}
		if rows == 0 {
			err = fmt.Errorf("task %q: %w", t.ID, ErrNotFound)
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// UpdateStatus changes a task's status, validating against the state machine.
func (s *TaskStore) UpdateStatus(ctx context.Context, id string, next models.Status) error {
	var currentStatus string
	err := s.db.QueryRowContext(ctx, "SELECT status FROM tasks WHERE id = ?", id).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("task %q: %w", id, ErrNotFound)
		}
		return fmt.Errorf("query current status: %w", err)
	}

	if !models.IsValidTransition(models.Status(currentStatus), next) {
		return fmt.Errorf("task %q: %w (current=%q, next=%q)", id, ErrInvalidTransition, currentStatus, next)
	}

	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE tasks SET status = ?, updated_at = ? WHERE id = ? AND status = ?`,
		next, now, id, currentStatus)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("status was modified concurrently")
	}

	return nil
}

// AddDependency creates a finish-to-start dependency between two tasks.
// It checks for cycles before inserting.
func (s *TaskStore) AddDependency(ctx context.Context, taskID, dependsOnID string) error {
	if taskID == dependsOnID {
		return fmt.Errorf("%w: %q", ErrSelfDependency, taskID)
	}

	hasCycle, err := s.DetectCycle(ctx, taskID, dependsOnID)
	if err != nil {
		return fmt.Errorf("check cycle before add dependency: %w", err)
	}
	if hasCycle {
		return fmt.Errorf("%w: adding dependency %q -> %q", ErrCycleDetected, taskID, dependsOnID)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO task_dependencies (task_id, depends_on_task_id) VALUES (?, ?)`,
		taskID, dependsOnID,
	)
	if err != nil {
		return fmt.Errorf("add dependency %q -> %q: %w", taskID, dependsOnID, err)
	}
	return nil
}

// RemoveDependency removes a dependency between two tasks.
func (s *TaskStore) RemoveDependency(ctx context.Context, taskID, dependsOnID string) error {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM task_dependencies WHERE task_id = ? AND depends_on_task_id = ?`,
		taskID, dependsOnID,
	)
	if err != nil {
		return fmt.Errorf("remove dependency %q -> %q: %w", taskID, dependsOnID, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected remove dependency: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("dependency %q -> %q: %w", taskID, dependsOnID, ErrNotFound)
	}

	return nil
}

// GetDependencies returns all task IDs that the given task depends on.
func (s *TaskStore) GetDependencies(ctx context.Context, taskID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT depends_on_task_id FROM task_dependencies WHERE task_id = ?`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get dependencies for task %q: %w", taskID, err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("rows close error: %v", err)
		}
	}()

	var deps []string
	for rows.Next() {
		var depID string
		if err := rows.Scan(&depID); err != nil {
			return nil, fmt.Errorf("scan dependency: %w", err)
		}
		deps = append(deps, depID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dependencies: %w", err)
	}

	return deps, nil
}

// GetBlockers returns all tasks that the given task depends on and are not done.
func (s *TaskStore) GetBlockers(ctx context.Context, taskID string) ([]*models.Task, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.numeric_id, t.story_id, t.title, t.description, t.status, t.task_type,
		        t.assigned_to, t.assignee_type, t.sort_order, t.instructions, t.is_stale, t.created_at, t.updated_at
		 FROM tasks t
		 JOIN task_dependencies td ON td.depends_on_task_id = t.id
		 WHERE td.task_id = ? AND t.status != ?`, taskID, models.StatusDone)
	if err != nil {
		return nil, fmt.Errorf("get blockers for task %q: %w", taskID, err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("rows close error: %v", err)
		}
	}()

	var blockers []*models.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan blocker task: %w", err)
		}
		blockers = append(blockers, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate blockers: %w", err)
	}

	return blockers, nil
}

// GetByStory returns all tasks belonging to a story.
func (s *TaskStore) GetByStory(ctx context.Context, storyID string) ([]*models.Task, error) {
	return s.List(ctx, TaskFilter{StoryID: storyID})
}

// loadDependencyGraph preloads the entire dependency graph from the database.
// The returned map is adjacency list: depends_on_task_id -> []task_id (forward edges).
func (s *TaskStore) loadDependencyGraph(ctx context.Context) (map[string][]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT task_id, depends_on_task_id FROM task_dependencies`)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("rows close error: %v", err)
		}
	}()

	adj := make(map[string][]string)
	for rows.Next() {
		var taskID, depID string
		if err := rows.Scan(&taskID, &depID); err != nil {
			return nil, err
		}
		// Forward: from depends_on_task_id -> task_id
		adj[depID] = append(adj[depID], taskID)
	}
	return adj, rows.Err()
}

// DetectCycle checks whether adding a dependency from dependsOnID -> taskID
// (i.e., taskID depends on dependsOnID) would create a cycle in the dependency
// graph. It preloads the entire graph and does a DFS in memory.
func (s *TaskStore) DetectCycle(ctx context.Context, taskID, dependsOnID string) (bool, error) {
	adj, err := s.loadDependencyGraph(ctx)
	if err != nil {
		return false, fmt.Errorf("load dependency graph: %w", err)
	}

	visited := make(map[string]bool)
	return s.dfsInMemory(taskID, dependsOnID, adj, visited), nil
}

// dfsInMemory performs a depth-first search from currentID following
// forward dependencies in the preloaded graph. If we reach targetID, a cycle exists.
func (s *TaskStore) dfsInMemory(currentID, targetID string, adj map[string][]string, visited map[string]bool) bool {
	if currentID == targetID {
		return true
	}
	if visited[currentID] {
		return false
	}
	visited[currentID] = true

	for _, next := range adj[currentID] {
		if s.dfsInMemory(next, targetID, adj, visited) {
			return true
		}
	}
	return false
}

// GetDependents returns all tasks that depend on the given task (reverse lookup).
func (s *TaskStore) GetDependents(ctx context.Context, taskID string) ([]*models.Task, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.numeric_id, t.story_id, t.title, t.description, t.status, t.task_type,
			t.assigned_to, t.assignee_type, t.sort_order, t.instructions, t.is_stale, t.created_at, t.updated_at
		FROM tasks t
		JOIN task_dependencies td ON td.task_id = t.id
		WHERE td.depends_on_task_id = ?`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get dependents for task %q: %w", taskID, err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("rows close error: %v", err)
		}
	}()

	var dependents []*models.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan dependent task: %w", err)
		}
		dependents = append(dependents, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dependents: %w", err)
	}

	return dependents, nil
}

// Delete removes a task, but only if its status is "new".
func (s *TaskStore) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM tasks WHERE id=? AND status=?`, id, models.StatusNew)
	if err != nil {
		return fmt.Errorf("delete task %q: %w", id, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("delete task %q: %w", id, ErrNotFound)
	}

	return nil
}

// GetBlockersForTasks batch-fetches blockers for a set of tasks. Returns a
// map from task ID to its blocker task IDs that are not yet done.
func (s *TaskStore) GetBlockersForTasks(ctx context.Context, taskIDs []string) (map[string][]string, error) {
	result := make(map[string][]string, len(taskIDs))
	if len(taskIDs) == 0 {
		return result, nil
	}

	placeholders := make([]string, len(taskIDs))
	args := make([]any, len(taskIDs))
	for i, id := range taskIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(
		`SELECT td.task_id, td.depends_on_task_id
		 FROM task_dependencies td
		 JOIN tasks t ON td.depends_on_task_id = t.id
		 WHERE td.task_id IN (%s) AND t.status != ?`,
		strings.Join(placeholders, ","),
	)
	args = append(args, models.StatusDone)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("batch get blockers: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("rows close error: %v", err)
		}
	}()

	for rows.Next() {
		var taskID, blockerID string
		if err := rows.Scan(&taskID, &blockerID); err != nil {
			return nil, fmt.Errorf("scan blocker row: %w", err)
		}
		result[taskID] = append(result[taskID], blockerID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate blockers: %w", err)
	}

	return result, nil
}
