package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
func (s *ActivityStore) GetByWorkItem(ctx context.Context, workItemID string, workItemType string, limit, offset int) ([]*models.ActivityLogEntry, error) {
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
	defer func() { _ = rows.Close() }()

	var entries []*models.ActivityLogEntry
	for rows.Next() {
		entry := &models.ActivityLogEntry{}
		var details sql.NullString
		var createdAt sql.NullTime

		if err := rows.Scan(
			&entry.ID, &entry.WorkItemID, &entry.WorkItemType,
			&entry.Action, &details, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan activity log: %w", err)
		}

		entry.Details = details.String
		if createdAt.Valid {
			entry.CreatedAt = createdAt.Time
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

	entry := &models.ActivityLogEntry{}
	var details sql.NullString
	var createdAt sql.NullTime

	err := row.Scan(
		&entry.ID, &entry.WorkItemID, &entry.WorkItemType,
		&entry.Action, &details, &createdAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("activity %q: %w", id, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("query activity %q: %w", id, err)
	}

	entry.Details = details.String
	if createdAt.Valid {
		entry.CreatedAt = createdAt.Time
	}

	return entry, nil
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
	defer func() { _ = rows.Close() }()

	var entries []*models.ActivityLogEntry
	for rows.Next() {
		entry := &models.ActivityLogEntry{}
		var details sql.NullString
		var createdAt sql.NullTime

		if err := rows.Scan(
			&entry.ID, &entry.WorkItemID, &entry.WorkItemType,
			&entry.Action, &details, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan activity log: %w", err)
		}

		entry.Details = details.String
		if createdAt.Valid {
			entry.CreatedAt = createdAt.Time
		}

		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate activity log: %w", err)
	}

	return entries, nil
}
