package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ubenmackin/loom/internal/models"
)

// ActivityStore provides append-only logging for activity entries.
type ActivityStore struct {
	db *sql.DB
}

// NewActivityStore creates a new ActivityStore.
func NewActivityStore(db *sql.DB) *ActivityStore {
	return &ActivityStore{db: db}
}

// scanActivityRow is a helper to scan an activity log row from a *sql.Row or *sql.Rows.
func scanActivityRow(scanner interface{ Scan(...any) error }) (*models.ActivityLogEntry, error) {
	entry := &models.ActivityLogEntry{}
	var details sql.NullString
	var createdAt sql.NullTime

	err := scanner.Scan(
		&entry.ID, &entry.WorkItemID, &entry.WorkItemType,
		&entry.Action, &details, &createdAt,
	)
	if err != nil {
		return nil, err
	}

	entry.Details = stringOrZero(details)
	entry.CreatedAt = timeOrZero(createdAt)

	return entry, nil
}

// Log inserts a new activity log entry. If the ID is empty, a UUID is generated.
func (s *ActivityStore) Log(ctx context.Context, entry *models.ActivityLogEntry) error {
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}

	entry.CreatedAt = time.Now().UTC()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO activity_log (id, work_item_id, work_item_type, action, details, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.WorkItemID, entry.WorkItemType, entry.Action,
		entry.Details, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert activity log: %w", err)
	}
	return nil
}

// GetByWorkItem retrieves activity log entries for a work item with pagination.
// Results are ordered by created_at descending (newest first).
func (s *ActivityStore) GetByWorkItem(ctx context.Context, workItemID string, workItemType models.WorkItemType, limit, offset int) ([]*models.ActivityLogEntry, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, work_item_id, work_item_type, action, details, created_at
		 FROM activity_log
		 WHERE work_item_id = ? AND work_item_type = ?
		 ORDER BY created_at DESC
		 LIMIT ? OFFSET ?`,
		workItemID, workItemType, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get activity for %s %q: %w", workItemType, workItemID, err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("rows close error: %v", err)
		}
	}()

	var entries []*models.ActivityLogEntry
	for rows.Next() {
		entry, err := scanActivityRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan activity log: %w", err)
		}

		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate activity log: %w", err)
	}

	return entries, nil
}

// GetByID retrieves a single activity log entry by ID.
func (s *ActivityStore) GetByID(ctx context.Context, id string) (*models.ActivityLogEntry, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, work_item_id, work_item_type, action, details, created_at
		 FROM activity_log WHERE id = ?`, id)

	entry, err := scanActivityRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("activity %q: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("query activity %q: %w", id, err)
	}

	return entry, nil
}

// GetByAction retrieves activity log entries where the action starts with one of the given prefixes.
// Supports pagination via limit and offset. Results are ordered by created_at DESC.
// When no prefixes are provided, all entries are returned (unfiltered).
func (s *ActivityStore) GetByAction(ctx context.Context, limit, offset int, actionPrefixes ...string) ([]*models.ActivityLogEntry, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `SELECT id, work_item_id, work_item_type, action, details, created_at
		 FROM activity_log`
	args := make([]interface{}, 0)

	if len(actionPrefixes) > 0 {
		conditions := make([]string, 0, len(actionPrefixes))
		for _, prefix := range actionPrefixes {
			conditions = append(conditions, "action LIKE ?")
			args = append(args, prefix+"%")
		}
		query += " WHERE " + strings.Join(conditions, " OR ")
	}

	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get activity by action: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("rows close error: %v", err)
		}
	}()

	var entries []*models.ActivityLogEntry
	for rows.Next() {
		entry, err := scanActivityRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan activity log: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate activity log: %w", err)
	}

	return entries, nil
}

// GetRecent retrieves the most recent activity log entries across all work items.
func (s *ActivityStore) GetRecent(ctx context.Context, limit int) ([]*models.ActivityLogEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, work_item_id, work_item_type, action, details, created_at
		FROM activity_log
		ORDER BY created_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("get recent activity: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("rows close error: %v", err)
		}
	}()

	var entries []*models.ActivityLogEntry
	for rows.Next() {
		entry, err := scanActivityRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan activity log: %w", err)
		}

		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate activity log: %w", err)
	}

	return entries, nil
}
