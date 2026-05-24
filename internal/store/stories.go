// Package store provides data access layer for Loom entities.
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

// StoryFilter holds optional criteria for listing stories.
type StoryFilter struct {
	Status     models.Status
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

// scanStoryRow is a helper to scan a story row from a *sql.Row or *sql.Rows.
func scanStoryRow(scanner interface{ Scan(...any) error }) (*models.Story, error) {
	story := &models.Story{}
	var desc, assignedTo, statusStr, assigneeTypeStr sql.NullString
	var createdAt, updatedAt sql.NullTime
	var numericID sql.NullInt64

	err := scanner.Scan(
		&story.ID, &numericID, &story.Title, &desc, &statusStr,
		&story.RequiresBuild, &story.RequiresReview, &assignedTo, &assigneeTypeStr,
		&story.SortOrder, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	story.Description = stringOrZero(desc)
	story.AssignedTo = stringOrZero(assignedTo)
	story.AssigneeType = models.AssigneeType(stringOrZero(assigneeTypeStr))
	story.Status = models.Status(stringOrZero(statusStr))
	story.NumericID = intOrZero(numericID)
	story.CreatedAt = timeOrZero(createdAt)
	story.UpdatedAt = timeOrZero(updatedAt)

	return story, nil
}

// Create inserts a new story. If the ID is empty, it is auto-generated.
// It mutates the pointer to set ID, NumericID, CreatedAt, and UpdatedAt.
func (s *StoryStore) Create(ctx context.Context, story *models.Story) error {
	res, err := s.db.ExecContext(ctx, "INSERT INTO work_item_sequence (type) VALUES ('story')")
	if err != nil {
		return fmt.Errorf("generate story id: %w", err)
	}
	seqID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id for story: %w", err)
	}
	if story.ID == "" {
		story.ID = fmt.Sprintf("STORY-%06d", seqID)
	}
	story.NumericID = int(seqID)

	now := time.Now().UTC()
	story.CreatedAt = now
	story.UpdatedAt = now
	if story.Status == "" {
		story.Status = models.StatusNew
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO stories (id, numeric_id, title, description, status, requires_build, requires_review, assigned_to, assignee_type, sort_order, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		story.ID, story.NumericID, story.Title, story.Description, story.Status,
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
		`SELECT id, numeric_id, title, description, status, requires_build, requires_review, assigned_to, assignee_type, sort_order, created_at, updated_at
		 FROM stories WHERE id = ?`, id)

	story, err := scanStoryRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("story %q: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("query story %q: %w", id, err)
	}

	return story, nil
}

// GetByNumericID retrieves a story by its numeric ID.
func (s *StoryStore) GetByNumericID(ctx context.Context, numID int) (*models.Story, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, numeric_id, title, description, status, requires_build, requires_review, assigned_to, assignee_type, sort_order, created_at, updated_at
		 FROM stories WHERE numeric_id = ?`, numID)

	story, err := scanStoryRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("story with numeric id %d: %w", numID, ErrNotFound)
		}
		return nil, fmt.Errorf("query story by numeric id %d: %w", numID, err)
	}

	return story, nil
}

// List returns stories matching the given filter. If filter fields are empty,
// all stories are returned.
func (s *StoryStore) List(ctx context.Context, filter StoryFilter) ([]*models.Story, error) {
	var conditions []string
	var args []any

	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.AssignedTo != "" {
		conditions = append(conditions, "assigned_to = ?")
		args = append(args, filter.AssignedTo)
	}

	query := `SELECT id, numeric_id, title, description, status, requires_build, requires_review, assigned_to, assignee_type, sort_order, created_at, updated_at
			  FROM stories`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY sort_order, created_at"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list stories: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("rows close error: %v", err)
		}
	}()

	var stories []*models.Story
	for rows.Next() {
		story, err := scanStoryRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan story: %w", err)
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
		`UPDATE stories SET title=?, description=?, status=?, requires_build=?, requires_review=?,
		 assigned_to=?, assignee_type=?, sort_order=?, updated_at=?
		 WHERE id=?`,
		story.Title, story.Description, story.Status,
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
		return fmt.Errorf("story %q: %w", story.ID, ErrNotFound)
	}

	return nil
}

// UpdateStatus changes a story's status, validating against the state machine.
func (s *StoryStore) UpdateStatus(ctx context.Context, id string, next models.Status) error {
	// First, get current status
	var currentStatus string
	err := s.db.QueryRowContext(ctx, "SELECT status FROM stories WHERE id = ?", id).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("story %q: %w", id, ErrNotFound)
		}
		return fmt.Errorf("query current status: %w", err)
	}

	// Validate transition
	if !models.IsValidTransition(models.Status(currentStatus), next) {
		return fmt.Errorf("story %q: %w (current=%q, next=%q)", id, ErrInvalidTransition, currentStatus, next)
	}

	// Atomic update with status check
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE stories SET status = ?, updated_at = ? WHERE id = ? AND status = ?`,
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

// Delete removes a story, but only if its status is "new".
func (s *StoryStore) Delete(ctx context.Context, id string) error {
	// First, check if the story exists.
	var status string
	err := s.db.QueryRowContext(ctx, `SELECT status FROM stories WHERE id=?`, id).Scan(&status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("delete story %q: %w", id, ErrNotFound)
		}
		return fmt.Errorf("delete story %q: %w", id, err)
	}

	if status != string(models.StatusNew) {
		return fmt.Errorf("delete story %q: status %s: %w", id, status, ErrInvalidTransition)
	}

	// Atomic DELETE — race-safe against concurrent deletes.
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM stories WHERE id=? AND status=?`, id, models.StatusNew)
	if err != nil {
		return fmt.Errorf("delete story %q: %w", id, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("delete story %q: %w", id, ErrNotFound)
	}

	return nil
}

// GetWithTasks retrieves a story along with its tasks using a single LEFT JOIN query.
func (s *StoryStore) GetWithTasks(ctx context.Context, id string) (*models.Story, []*models.Task, error) {
	query := `SELECT
		s.id, s.numeric_id, s.title, s.description, s.status,
		s.requires_build, s.requires_review, s.assigned_to, s.assignee_type,
		s.sort_order, s.created_at, s.updated_at,
		t.id, t.numeric_id, t.story_id, t.title, t.description, t.status,
		t.task_type, t.assigned_to, t.assignee_type,
		t.sort_order, t.instructions, t.is_stale, t.created_at, t.updated_at
		FROM stories s
		LEFT JOIN tasks t ON t.story_id = s.id
		WHERE s.id = ?
		ORDER BY t.sort_order`

	rows, err := s.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, nil, fmt.Errorf("query story with tasks %q: %w", id, err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("rows close error: %v", err)
		}
	}()

	var story *models.Story
	var tasks []*models.Task

	for rows.Next() {
		var (
			// Story columns
			sID, sTitle, sDesc, sStatusStr, sAssignedTo, sAssigneeTypeStr sql.NullString
			sNumID                                                        sql.NullInt64
			sSortOrder                                                    sql.NullInt64
			sRequiresBuild, sRequiresReview                               sql.NullBool
			sCreatedAt, sUpdatedAt                                        sql.NullTime

			// Task columns
			tID, tStoryID, tTitle, tDesc, tStatusStr, tAssignedTo, tAssigneeTypeStr sql.NullString
			tTaskTypeStr, tInstructions                                             sql.NullString
			tNumID, tSortOrder                                                      sql.NullInt64
			tCreatedAt, tUpdatedAt                                                  sql.NullTime
			tIsStale                                                                sql.NullBool
		)

		err := rows.Scan(
			&sID, &sNumID, &sTitle, &sDesc, &sStatusStr,
			&sRequiresBuild, &sRequiresReview, &sAssignedTo, &sAssigneeTypeStr,
			&sSortOrder, &sCreatedAt, &sUpdatedAt,
			&tID, &tNumID, &tStoryID, &tTitle, &tDesc, &tStatusStr,
			&tTaskTypeStr, &tAssignedTo, &tAssigneeTypeStr,
			&tSortOrder, &tInstructions, &tIsStale, &tCreatedAt, &tUpdatedAt,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("scan story with tasks: %w", err)
		}

		if story == nil {
			story = &models.Story{
				ID:             sID.String,
				NumericID:      intOrZero(sNumID),
				Title:          sTitle.String,
				Description:    sDesc.String,
				Status:         models.Status(sStatusStr.String),
				RequiresBuild:  sRequiresBuild.Bool,
				RequiresReview: sRequiresReview.Bool,
				AssignedTo:     sAssignedTo.String,
				AssigneeType:   models.AssigneeType(sAssigneeTypeStr.String),
				SortOrder:      int(sSortOrder.Int64),
				CreatedAt:      timeOrZero(sCreatedAt),
				UpdatedAt:      timeOrZero(sUpdatedAt),
			}
		}

		if tID.Valid {
			task := &models.Task{
				ID:           tID.String,
				NumericID:    intOrZero(tNumID),
				StoryID:      tStoryID.String,
				Title:        tTitle.String,
				Description:  tDesc.String,
				Status:       models.Status(tStatusStr.String),
				TaskType:     models.TaskType(tTaskTypeStr.String),
				AssignedTo:   tAssignedTo.String,
				AssigneeType: models.AssigneeType(tAssigneeTypeStr.String),
				SortOrder:    int(tSortOrder.Int64),
				Instructions: tInstructions.String,
				IsStale:      tIsStale.Bool,
				CreatedAt:    timeOrZero(tCreatedAt),
				UpdatedAt:    timeOrZero(tUpdatedAt),
			}
			tasks = append(tasks, task)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate story with tasks: %w", err)
	}

	if story == nil {
		return nil, nil, fmt.Errorf("story %q: %w", id, ErrNotFound)
	}

	if tasks == nil {
		tasks = []*models.Task{}
	}

	return story, tasks, nil
}
