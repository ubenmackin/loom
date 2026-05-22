package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ubenmackin/loom/internal/models"
)

// TaskFilter holds optional criteria for listing tasks.
type TaskFilter struct {
	StoryID    string
	Status     string
	AssignedTo string
	TaskType   string
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
func scanTask(scanner interface{ Scan(...interface{}) error }) (*models.Task, error) {
	t := &models.Task{}
	var desc, assignedTo, assigneeType, contextJSON, instructions sql.NullString
	var estimate sql.NullInt64
	var createdAt, updatedAt sql.NullTime
	var numericID sql.NullInt64

	err := scanner.Scan(
		&t.ID, &numericID, &t.StoryID, &t.Title, &desc, &t.Status, &t.Priority, &t.TaskType,
		&estimate, &assignedTo, &assigneeType, &t.SortOrder,
		&contextJSON, &instructions, &t.IsStale, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	t.Description = desc.String
	t.AssignedTo = assignedTo.String
	t.AssigneeType = assigneeType.String
	t.Context = contextJSON.String
	t.Instructions = instructions.String
	if numericID.Valid {
		t.NumericID = int(numericID.Int64)
	}
	if estimate.Valid {
		e := int(estimate.Int64)
		t.Estimate = &e
	}
	if createdAt.Valid {
		t.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		t.UpdatedAt = updatedAt.Time
	}

	return t, nil
}

// Create inserts a new task. If the ID is empty, it is auto-generated.
func (s *TaskStore) Create(ctx context.Context, t *models.Task) error {
	// Generate both string ID and numeric ID from a single atomic sequence insert.
	// This eliminates the TOCTOU race that occurred when the string ID was generated
	// via a separate MAX+1 query committed before the actual INSERT.
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

	var estimate *int
	if t.Estimate != nil {
		estimate = t.Estimate
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO tasks (id, numeric_id, story_id, title, description, status, priority, task_type, estimate,
		 assigned_to, assignee_type, sort_order, context, instructions, is_stale, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.NumericID, t.StoryID, t.Title, t.Description, t.Status, t.Priority, t.TaskType,
		estimate, t.AssignedTo, t.AssigneeType, t.SortOrder,
		t.Context, t.Instructions, t.IsStale, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}
	return nil
}

// GetByID retrieves a task by its ID.
func (s *TaskStore) GetByID(ctx context.Context, id string) (*models.Task, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, numeric_id, story_id, title, description, status, priority, task_type, estimate,
		        assigned_to, assignee_type, sort_order, context, instructions, is_stale, created_at, updated_at
		 FROM tasks WHERE id = ?`, id)

	t, err := scanTask(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("task %q: %w", id, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("query task %q: %w", id, err)
	}
	return t, nil
}

// GetByNumericID retrieves a task by its numeric ID.
func (s *TaskStore) GetByNumericID(ctx context.Context, numID int) (*models.Task, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, numeric_id, story_id, title, description, status, priority, task_type, estimate,
		        assigned_to, assignee_type, sort_order, context, instructions, is_stale, created_at, updated_at
		 FROM tasks WHERE numeric_id = ?`, numID)

	t, err := scanTask(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("task with numeric id %d: %w", numID, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("query task by numeric id %d: %w", numID, err)
	}
	return t, nil
}

// List returns tasks matching the given filter.
func (s *TaskStore) List(ctx context.Context, filter TaskFilter) ([]*models.Task, error) {
	var conditions []string
	var args []interface{}

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

	query := `SELECT id, numeric_id, story_id, title, description, status, priority, task_type, estimate,
	                 assigned_to, assignee_type, sort_order, context, instructions, is_stale, created_at, updated_at
	          FROM tasks`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY sort_order, priority, created_at"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer func() { _ = rows.Close() }()

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

	var estimate *int
	if t.Estimate != nil {
		estimate = t.Estimate
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE tasks SET story_id=?, title=?, description=?, status=?, priority=?, task_type=?,
		 estimate=?, assigned_to=?, assignee_type=?, sort_order=?, context=?, instructions=?,
		 is_stale=?, updated_at=?
		 WHERE id=?`,
		t.StoryID, t.Title, t.Description, t.Status, t.Priority, t.TaskType,
		estimate, t.AssignedTo, t.AssigneeType, t.SortOrder, t.Context, t.Instructions,
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
		return fmt.Errorf("task %q: %w", t.ID, sql.ErrNoRows)
	}

	return nil
}

// UpdateStatus changes a task's status, validating against the state machine.
func (s *TaskStore) UpdateStatus(ctx context.Context, id string, status string) error {
	current, err := s.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get task for status update: %w", err)
	}

	if !isValidTransition(current.Status, status) {
		return fmt.Errorf("invalid transition %q -> %q for task %q", current.Status, status, id)
	}

	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE tasks SET status=?, updated_at=? WHERE id=?`,
		status, now, id,
	)
	if err != nil {
		return fmt.Errorf("update task status %q: %w", id, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected task %q: %w", id, err)
	}
	if rows == 0 {
		return fmt.Errorf("task %q: %w", id, sql.ErrNoRows)
	}

	return nil
}

// AddDependency creates a finish-to-start dependency between two tasks.
// It checks for cycles before inserting.
func (s *TaskStore) AddDependency(ctx context.Context, taskID, dependsOnID string) error {
	if taskID == dependsOnID {
		return fmt.Errorf("task cannot depend on itself: %q", taskID)
	}

	hasCycle, err := s.DetectCycle(ctx, taskID, dependsOnID)
	if err != nil {
		return fmt.Errorf("check cycle before add dependency: %w", err)
	}
	if hasCycle {
		return fmt.Errorf("adding dependency %q -> %q would create a cycle", dependsOnID, taskID)
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
		return fmt.Errorf("dependency %q -> %q not found", taskID, dependsOnID)
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
	defer func() { _ = rows.Close() }()

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
		`SELECT t.id, t.numeric_id, t.story_id, t.title, t.description, t.status, t.priority, t.task_type, t.estimate,
		        t.assigned_to, t.assignee_type, t.sort_order, t.context, t.instructions, t.is_stale, t.created_at, t.updated_at
		 FROM tasks t
		 JOIN task_dependencies td ON td.depends_on_task_id = t.id
		 WHERE td.task_id = ? AND t.status != ?`, taskID, models.StatusDone)
	if err != nil {
		return nil, fmt.Errorf("get blockers for task %q: %w", taskID, err)
	}
	defer func() { _ = rows.Close() }()

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

// DetectCycle checks whether adding a dependency from dependsOnID -> taskID
// (i.e., taskID depends on dependsOnID) would create a cycle in the dependency
// graph. It does a DFS starting from taskID, following the dependency chain,
// to see if we can reach dependsOnID.
func (s *TaskStore) DetectCycle(ctx context.Context, taskID, dependsOnID string) (bool, error) {
	visited := make(map[string]bool)
	return s.dfsDetectCycle(ctx, taskID, dependsOnID, visited)
}

// dfsDetectCycle performs a depth-first search from currentID following
// dependencies. If we reach targetID, a cycle exists.
func (s *TaskStore) dfsDetectCycle(ctx context.Context, currentID, targetID string, visited map[string]bool) (bool, error) {
	if currentID == targetID {
		return true, nil
	}
	if visited[currentID] {
		return false, nil
	}
	visited[currentID] = true

	// Get tasks that depend on currentID (i.e., currentID is in depends_on_task_id),
	// meaning we traverse forward in the dependency graph.
	rows, err := s.db.QueryContext(ctx,
		`SELECT task_id FROM task_dependencies WHERE depends_on_task_id = ?`, currentID)
	if err != nil {
		return false, fmt.Errorf("dfs query dependencies for %q: %w", currentID, err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var nextID string
		if err := rows.Scan(&nextID); err != nil {
			return false, fmt.Errorf("dfs scan dependency: %w", err)
		}
		found, err := s.dfsDetectCycle(ctx, nextID, targetID, visited)
		if err != nil {
			return false, err
		}
		if found {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("dfs iterate dependencies: %w", err)
	}

	return false, nil
}

// GetDependents returns all tasks that depend on the given task (reverse lookup).
// These are tasks whose depends_on_task_id matches the given taskID.
func (s *TaskStore) GetDependents(ctx context.Context, taskID string) ([]*models.Task, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.numeric_id, t.story_id, t.title, t.description, t.status, t.priority, t.task_type, t.estimate,
			t.assigned_to, t.assignee_type, t.sort_order, t.context, t.instructions, t.is_stale, t.created_at, t.updated_at
		FROM tasks t
		JOIN task_dependencies td ON td.task_id = t.id
		WHERE td.depends_on_task_id = ?`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get dependents for task %q: %w", taskID, err)
	}
	defer func() { _ = rows.Close() }()

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
