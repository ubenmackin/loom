package store

import (
	"context"
	"database/sql"
	"fmt"
)

// ProfileTaskTypeStore provides CRUD operations for the profile_task_types join table.
type ProfileTaskTypeStore struct {
	db *sql.DB
}

// NewProfileTaskTypeStore creates a new ProfileTaskTypeStore.
func NewProfileTaskTypeStore(db *sql.DB) *ProfileTaskTypeStore {
	return &ProfileTaskTypeStore{db: db}
}

// SetForProfile replaces all task types for the given profile in a single transaction.
// NOTE: This opens its own transaction and is intended for external use. For Create/Update
// operations on AgentProfileStore (which have their own parent transaction), the task type
// insert logic is inlined directly to share the parent transaction.
func (s *ProfileTaskTypeStore) SetForProfile(ctx context.Context, profileID string, taskTypes []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Delete existing
	if _, err := tx.ExecContext(ctx, `DELETE FROM profile_task_types WHERE profile_id = ?`, profileID); err != nil {
		return fmt.Errorf("delete profile_task_types: %w", err)
	}

	// Insert new ones
	for _, tt := range taskTypes {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO profile_task_types (profile_id, task_type) VALUES (?, ?)`,
			profileID, tt); err != nil {
			return fmt.Errorf("insert profile_task_type %q: %w", tt, err)
		}
	}

	return tx.Commit()
}

// GetByProfileIDs returns a map of profile_id -> task_types for all given profile IDs in a single batch query.
func (s *ProfileTaskTypeStore) GetByProfileIDs(ctx context.Context, profileIDs []string) (map[string][]string, error) {
	if len(profileIDs) == 0 {
		return make(map[string][]string), nil
	}

	// Build the query with IN clause.
	// Note: SQLite doesn't support array parameters, so we use one placeholder per ID.
	query := `SELECT profile_id, task_type FROM profile_task_types WHERE profile_id IN (`
	params := make([]interface{}, len(profileIDs))
	for i, id := range profileIDs {
		if i > 0 {
			query += ", "
		}
		query += "?"
		params[i] = id
	}
	query += ") ORDER BY profile_id, task_type"

	rows, err := s.db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("query profile_task_types batch: %w", err)
	}
	defer closeRows(rows)

	result := make(map[string][]string)
	for rows.Next() {
		var profileID, taskType string
		if err := rows.Scan(&profileID, &taskType); err != nil {
			return nil, fmt.Errorf("scan profile_task_type: %w", err)
		}
		result[profileID] = append(result[profileID], taskType)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	// Ensure all requested profile IDs have an entry (even if empty).
	for _, id := range profileIDs {
		if _, ok := result[id]; !ok {
			result[id] = []string{}
		}
	}

	return result, nil
}

// GetByProfileID returns all task types associated with the given profile.
func (s *ProfileTaskTypeStore) GetByProfileID(ctx context.Context, profileID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT task_type FROM profile_task_types WHERE profile_id = ? ORDER BY task_type`, profileID)
	if err != nil {
		return nil, fmt.Errorf("query profile_task_types: %w", err)
	}
	defer closeRows(rows)

	var taskTypes []string
	for rows.Next() {
		var tt string
		if err := rows.Scan(&tt); err != nil {
			return nil, fmt.Errorf("scan task_type: %w", err)
		}
		taskTypes = append(taskTypes, tt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return taskTypes, nil
}
