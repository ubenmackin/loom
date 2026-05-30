package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ubenmackin/loom/internal/models"
)

// SessionStore provides CRUD operations for sessions.
type SessionStore struct {
	db *sql.DB
}

// NewSessionStore creates a new SessionStore.
func NewSessionStore(db *sql.DB) *SessionStore {
	return &SessionStore{db: db}
}

// scanSessionRow is a helper to scan a session row from a *sql.Row or *sql.Rows.
func scanSessionRow(scanner interface{ Scan(...any) error }) (*models.Session, error) {
	session := &models.Session{}
	var capabilities, metadata sql.NullString
	var lastSeenAt, createdAt sql.NullTime

	err := scanner.Scan(
		&session.ID, &session.HarnessType, &capabilities, &metadata,
		&lastSeenAt, &session.Status, &createdAt,
	)
	if err != nil {
		return nil, err
	}

	session.Capabilities = stringOrZero(capabilities)
	session.Metadata = stringOrZero(metadata)
	session.LastSeenAt = timeOrZero(lastSeenAt)
	session.CreatedAt = timeOrZero(createdAt)

	return session, nil
}

// Register inserts a new session.
func (s *SessionStore) Register(ctx context.Context, session *models.Session) error {
	now := time.Now().UTC()
	session.CreatedAt = now
	session.LastSeenAt = now
	if session.Status == "" {
		session.Status = models.SessionStatusActive
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, harness_type, capabilities, metadata, last_seen_at, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.HarnessType, session.Capabilities, session.Metadata,
		session.LastSeenAt, session.Status, session.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

// GetByID retrieves a session by its ID.
func (s *SessionStore) GetByID(ctx context.Context, id string) (*models.Session, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, harness_type, capabilities, metadata, last_seen_at, status, created_at
		 FROM sessions WHERE id = ?`, id)

	session, err := scanSessionRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("session %q: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("query session %q: %w", id, err)
	}

	return session, nil
}

// UpdateLastSeen sets last_seen_at to the current time.
func (s *SessionStore) UpdateLastSeen(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET last_seen_at = ? WHERE id = ?`, time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("update last seen for session %q: %w", id, err)
	}
	return requireOneRow(result, nil, "session", id)
}

// Disconnect sets a session's status to "disconnected".
func (s *SessionStore) Disconnect(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET status = ? WHERE id = ?`, models.SessionStatusDisconnected, id,
	)
	if err != nil {
		return fmt.Errorf("disconnect session %q: %w", id, err)
	}
	return requireOneRow(result, nil, "session", id)
}

// GetStaleSessions returns active sessions whose last_seen_at is older than
// the given threshold from now.
func (s *SessionStore) GetStaleSessions(ctx context.Context, threshold time.Duration) ([]*models.Session, error) {
	cutoff := time.Now().UTC().Add(-threshold)

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, harness_type, capabilities, metadata, last_seen_at, status, created_at
		 FROM sessions
		 WHERE last_seen_at < ? AND status = ?
		 ORDER BY last_seen_at ASC`, cutoff, models.SessionStatusActive)
	if err != nil {
		return nil, fmt.Errorf("get stale sessions: %w", err)
	}
	defer closeRows(rows)

	return collectRows(rows, scanSessionRow)
}

// GetByCapabilities returns sessions whose capabilities JSON array contains
// the given capability string.
func (s *SessionStore) GetByCapabilities(ctx context.Context, capability string) ([]*models.Session, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, harness_type, capabilities, metadata, last_seen_at, status, created_at
		 FROM sessions
		 WHERE status = ?
		 ORDER BY created_at ASC`, models.SessionStatusActive)
	if err != nil {
		return nil, fmt.Errorf("get sessions by capability: %w", err)
	}
	defer closeRows(rows)

	all, err := collectRows(rows, scanSessionRow)
	if err != nil {
		return nil, err
	}

	return filterByCapability(all, capability), nil
}

// filterByCapability filters a session slice to only those whose capabilities
// JSON array contains the given capability string.
func filterByCapability(sessions []*models.Session, capability string) []*models.Session {
	var filtered []*models.Session
	for _, session := range sessions {
		caps, err := session.CapabilitiesSlice()
		if err != nil || len(caps) == 0 {
			continue
		}
		for _, c := range caps {
			if c == capability {
				filtered = append(filtered, session)
				break
			}
		}
	}
	return filtered
}

// SessionWithTaskCount pairs a session with its current number of assigned tasks.
type SessionWithTaskCount struct {
	Session   *models.Session
	TaskCount int
}

// GetByCapabilitiesWithTaskCount returns active sessions matching the given
// capability, each annotated with its current assigned-task count.
func (s *SessionStore) GetByCapabilitiesWithTaskCount(ctx context.Context, capability string) ([]SessionWithTaskCount, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.harness_type, s.capabilities, s.metadata, s.last_seen_at, s.status, s.created_at,
		       COALESCE(t.task_count, 0)
		FROM sessions s
		LEFT JOIN (
			SELECT assigned_to, COUNT(*) AS task_count
			FROM tasks
			WHERE assigned_to IS NOT NULL AND assigned_to != ''
			GROUP BY assigned_to
		) t ON t.assigned_to = s.id
		WHERE s.status = ?
		ORDER BY s.created_at ASC`, models.SessionStatusActive)
	if err != nil {
		return nil, fmt.Errorf("get sessions by capability with task count: %w", err)
	}
	defer closeRows(rows)

	var results []SessionWithTaskCount
	for rows.Next() {
		session := &models.Session{}
		var taskCount int
		var capabilities, metadata sql.NullString
		var lastSeenAt, createdAt sql.NullTime

		if err := rows.Scan(
			&session.ID, &session.HarnessType, &capabilities, &metadata,
			&lastSeenAt, &session.Status, &createdAt,
			&taskCount,
		); err != nil {
			return nil, fmt.Errorf("scan session with task count: %w", err)
		}

		session.Capabilities = stringOrZero(capabilities)
		session.Metadata = stringOrZero(metadata)
		session.LastSeenAt = timeOrZero(lastSeenAt)
		session.CreatedAt = timeOrZero(createdAt)

		results = append(results, SessionWithTaskCount{
			Session:   session,
			TaskCount: taskCount,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions with task count: %w", err)
	}

	// Apply in-memory capability filter.
	matched := make([]SessionWithTaskCount, 0, len(results))
	filtered := filterByCapability(sessionsFromResults(results), capability)
	filteredSet := make(map[string]bool, len(filtered))
	for _, s := range filtered {
		filteredSet[s.ID] = true
	}
	for _, r := range results {
		if filteredSet[r.Session.ID] {
			matched = append(matched, r)
		}
	}

	return matched, nil
}

// sessionsFromResults extracts the session pointers from a SessionWithTaskCount slice.
func sessionsFromResults(results []SessionWithTaskCount) []*models.Session {
	sessions := make([]*models.Session, len(results))
	for i, r := range results {
		sessions[i] = r.Session
	}
	return sessions
}

// GetTasksForSession returns all tasks assigned to the given session.
func (s *SessionStore) GetTasksForSession(ctx context.Context, sessionID string) ([]*models.Task, error) {
	ts := &TaskStore{db: s.db}
	return ts.List(ctx, TaskFilter{AssignedTo: sessionID})
}

// FlagStale sets a session's status to "stale".
func (s *SessionStore) FlagStale(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET status = ? WHERE id = ?`, models.SessionStatusStale, id,
	)
	if err != nil {
		return fmt.Errorf("flag stale session %q: %w", id, err)
	}
	return requireOneRow(result, nil, "session", id)
}

// ListActive returns all sessions with status "active".
func (s *SessionStore) ListActive(ctx context.Context) ([]*models.Session, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, harness_type, capabilities, metadata, last_seen_at, status, created_at
		 FROM sessions
		 WHERE status = ?
		 ORDER BY created_at ASC`, models.SessionStatusActive)
	if err != nil {
		return nil, fmt.Errorf("list active sessions: %w", err)
	}
	defer closeRows(rows)

	return collectRows(rows, scanSessionRow)
}

// ListAll returns all sessions regardless of status, ordered by created_at descending.
func (s *SessionStore) ListAll(ctx context.Context) ([]*models.Session, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, harness_type, capabilities, metadata, last_seen_at, status, created_at
		FROM sessions
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list all sessions: %w", err)
	}
	defer closeRows(rows)

	return collectRows(rows, scanSessionRow)
}
