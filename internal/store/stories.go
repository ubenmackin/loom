// Package store provides data access layer for Loom entities.
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

// StoryFilter holds optional criteria for listing stories.
type StoryFilter struct {
	Status     string
	AssignedTo string
}

// StoryStore provides CRUD operations for stories.
type StoryStore struct {
	db *sql.DB
}

// NewStoryStore creates a new StoryStore.
func NewStoryStore(db *sql.DB) *StoryStore {
	return &StoryStore{db: db}
}

// validStoryTransitions defines the allowed status transitions.
var validStoryTransitions = map[string][]string{
	models.StatusNew:        {models.StatusReady, models.StatusInProgress},
	models.StatusReady:      {models.StatusInProgress, models.StatusBlocked},
	models.StatusInProgress: {models.StatusBlocked, models.StatusDone},
	models.StatusBlocked:    {models.StatusInProgress, models.StatusReady},
	models.StatusDone:       {},
}

// isValidTransition checks whether moving from current to next is allowed.
func isValidTransition(current, next string) bool {
	allowed, ok := validStoryTransitions[current]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == next {
			return true
		}
	}
	return false
}

// nextStoryID generates the next story ID in the format STORY-NNNNNN.
// It uses a BEGIN IMMEDIATE transaction to serialize the MAX+1 operation
// and prevent TOCTOU races between concurrent creates.
func (s *StoryStore) nextStoryID(ctx context.Context) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin transaction for story id: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var maxID sql.NullString
	err = tx.QueryRowContext(ctx, "SELECT id FROM stories ORDER BY CAST(SUBSTR(id, 7) AS INTEGER) DESC LIMIT 1").Scan(&maxID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("query max story id: %w", err)
	}

	var nextID string
	if !maxID.Valid || maxID.String == "" {
		nextID = "STORY-000001"
	} else {
		var n int
		if _, err := fmt.Sscanf(maxID.String, "STORY-%d", &n); err != nil {
			return "", fmt.Errorf("parse story id %q: %w", maxID.String, err)
		}
		nextID = fmt.Sprintf("STORY-%06d", n+1)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit story id transaction: %w", err)
	}

	return nextID, nil
}

// Create inserts a new story. If the ID is empty, it is auto-generated.
func (s *StoryStore) Create(ctx context.Context, story *models.Story) error {
	if story.ID == "" {
		id, err := s.nextStoryID(ctx)
		if err != nil {
			return fmt.Errorf("generate story id: %w", err)
		}
		story.ID = id
	}

	// Generate a global unique numeric ID across all work items
	res, err := s.db.ExecContext(ctx, "INSERT INTO work_item_sequence (type) VALUES ('story')")
	if err != nil {
		return fmt.Errorf("generate numeric id for story: %w", err)
	}
	numericID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("get numeric id for story: %w", err)
	}
	story.NumericID = int(numericID)

	now := time.Now().UTC()
	story.CreatedAt = now
	story.UpdatedAt = now
	if story.Status == "" {
		story.Status = models.StatusNew
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO stories (id, numeric_id, title, description, status, priority, requires_build, requires_review, assigned_to, assignee_type, sort_order, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		story.ID, story.NumericID, story.Title, story.Description, story.Status, story.Priority,
		story.RequiresBuild, story.RequiresReview, story.AssignedTo, story.AssigneeType,
		story.SortOrder, story.CreatedAt, story.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert story: %w", err)
	}
	return nil
}

// GetByID retrieves a story by its ID.
func (s *StoryStore) GetByID(ctx context.Context, id string) (*models.Story, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, numeric_id, title, description, status, priority, requires_build, requires_review, assigned_to, assignee_type, sort_order, created_at, updated_at
		 FROM stories WHERE id = ?`, id)

	story := &models.Story{}
	var desc, assignedTo, assigneeType sql.NullString
	var createdAt, updatedAt sql.NullTime
	var numericID sql.NullInt64

	err := row.Scan(
		&story.ID, &numericID, &story.Title, &desc, &story.Status, &story.Priority,
		&story.RequiresBuild, &story.RequiresReview, &assignedTo, &assigneeType,
		&story.SortOrder, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("story %q: %w", id, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("query story %q: %w", id, err)
	}

	story.Description = desc.String
	story.AssignedTo = assignedTo.String
	story.AssigneeType = assigneeType.String
	if numericID.Valid {
		story.NumericID = int(numericID.Int64)
	}
	if createdAt.Valid {
		story.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		story.UpdatedAt = updatedAt.Time
	}

	return story, nil
}

// GetByNumericID retrieves a story by its numeric ID.
func (s *StoryStore) GetByNumericID(ctx context.Context, numID int) (*models.Story, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, numeric_id, title, description, status, priority, requires_build, requires_review, assigned_to, assignee_type, sort_order, created_at, updated_at
		 FROM stories WHERE numeric_id = ?`, numID)

	story := &models.Story{}
	var desc, assignedTo, assigneeType sql.NullString
	var createdAt, updatedAt sql.NullTime
	var numericID sql.NullInt64

	err := row.Scan(
		&story.ID, &numericID, &story.Title, &desc, &story.Status, &story.Priority,
		&story.RequiresBuild, &story.RequiresReview, &assignedTo, &assigneeType,
		&story.SortOrder, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("story with numeric id %d: %w", numID, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("query story by numeric id %d: %w", numID, err)
	}

	story.Description = desc.String
	story.AssignedTo = assignedTo.String
	story.AssigneeType = assigneeType.String
	if numericID.Valid {
		story.NumericID = int(numericID.Int64)
	}
	if createdAt.Valid {
		story.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		story.UpdatedAt = updatedAt.Time
	}

	return story, nil
}

// List returns stories matching the given filter. If filter fields are empty,
// all stories are returned.
func (s *StoryStore) List(ctx context.Context, filter StoryFilter) ([]*models.Story, error) {
	var conditions []string
	var args []interface{}

	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.AssignedTo != "" {
		conditions = append(conditions, "assigned_to = ?")
		args = append(args, filter.AssignedTo)
	}

	query := `SELECT id, numeric_id, title, description, status, priority, requires_build, requires_review, assigned_to, assignee_type, sort_order, created_at, updated_at
			  FROM stories`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY sort_order, priority, created_at"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list stories: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var stories []*models.Story
	for rows.Next() {
		story := &models.Story{}
		var desc, assignedTo, assigneeType sql.NullString
		var createdAt, updatedAt sql.NullTime
		var numericID sql.NullInt64

		if err := rows.Scan(
			&story.ID, &numericID, &story.Title, &desc, &story.Status, &story.Priority,
			&story.RequiresBuild, &story.RequiresReview, &assignedTo, &assigneeType,
			&story.SortOrder, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan story: %w", err)
		}

		story.Description = desc.String
		story.AssignedTo = assignedTo.String
		story.AssigneeType = assigneeType.String
		if numericID.Valid {
			story.NumericID = int(numericID.Int64)
		}
		if createdAt.Valid {
			story.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			story.UpdatedAt = updatedAt.Time
		}

		stories = append(stories, story)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stories: %w", err)
	}

	return stories, nil
}

// Update saves all mutable fields of a story.
func (s *StoryStore) Update(ctx context.Context, story *models.Story) error {
	story.UpdatedAt = time.Now().UTC()

	result, err := s.db.ExecContext(ctx,
		`UPDATE stories SET title=?, description=?, status=?, priority=?, requires_build=?, requires_review=?,
		 assigned_to=?, assignee_type=?, sort_order=?, updated_at=?
		 WHERE id=?`,
		story.Title, story.Description, story.Status, story.Priority,
		story.RequiresBuild, story.RequiresReview, story.AssignedTo, story.AssigneeType,
		story.SortOrder, story.UpdatedAt, story.ID,
	)
	if err != nil {
		return fmt.Errorf("update story %q: %w", story.ID, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected story %q: %w", story.ID, err)
	}
	if rows == 0 {
		return fmt.Errorf("story %q: %w", story.ID, sql.ErrNoRows)
	}

	return nil
}

// UpdateStatus changes a story's status, validating against the state machine.
func (s *StoryStore) UpdateStatus(ctx context.Context, id string, status string) error {
	current, err := s.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get story for status update: %w", err)
	}

	if !isValidTransition(current.Status, status) {
		return fmt.Errorf("invalid transition %q -> %q for story %q", current.Status, status, id)
	}

	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE stories SET status=?, updated_at=? WHERE id=?`,
		status, now, id,
	)
	if err != nil {
		return fmt.Errorf("update story status %q: %w", id, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected story %q: %w", id, err)
	}
	if rows == 0 {
		return fmt.Errorf("story %q: %w", id, sql.ErrNoRows)
	}

	return nil
}

// Delete removes a story, but only if its status is "new".
func (s *StoryStore) Delete(ctx context.Context, id string) error {
	current, err := s.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get story for delete: %w", err)
	}

	if current.Status != models.StatusNew {
		return fmt.Errorf("cannot delete story %q with status %q (only %q allowed)", id, current.Status, models.StatusNew)
	}

	result, err := s.db.ExecContext(ctx, `DELETE FROM stories WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete story %q: %w", id, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected delete story %q: %w", id, err)
	}
	if rows == 0 {
		return fmt.Errorf("story %q: %w", id, sql.ErrNoRows)
	}

	return nil
}

// GetWithTasks retrieves a story along with its tasks.
func (s *StoryStore) GetWithTasks(ctx context.Context, id string) (*models.Story, []*models.Task, error) {
	story, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("get story with tasks: %w", err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, numeric_id, story_id, title, description, status, priority, task_type, estimate,
		        assigned_to, assignee_type, sort_order, context, instructions, is_stale, created_at, updated_at
		 FROM tasks WHERE story_id = ? ORDER BY sort_order, priority`, id)
	if err != nil {
		return nil, nil, fmt.Errorf("query tasks for story %q: %w", id, err)
	}
	defer func() { _ = rows.Close() }()

	var tasks []*models.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, nil, fmt.Errorf("scan task for story %q: %w", id, err)
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate tasks for story %q: %w", id, err)
	}

	return story, tasks, nil
}
