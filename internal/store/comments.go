package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ubenmackin/loom/internal/models"
)

// CommentStore provides CRUD operations for comments.
type CommentStore struct {
	db *sql.DB
}

// NewCommentStore creates a new CommentStore.
func NewCommentStore(db *sql.DB) *CommentStore {
	return &CommentStore{db: db}
}

// nextCommentID generates the next comment ID in the format COMMENT-NNNNNN.
// It uses a BEGIN IMMEDIATE transaction to serialize the MAX+1 operation
// and prevent TOCTOU races between concurrent creates.
func (s *CommentStore) nextCommentID(ctx context.Context) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin transaction for comment id: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var maxID sql.NullString
	err = tx.QueryRowContext(ctx, "SELECT id FROM comments ORDER BY CAST(SUBSTR(id, 8) AS INTEGER) DESC LIMIT 1").Scan(&maxID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("query max comment id: %w", err)
	}

	var nextID string
	if !maxID.Valid || maxID.String == "" {
		nextID = "COMMENT-000001"
	} else {
		var n int
		if _, err := fmt.Sscanf(maxID.String, "COMMENT-%d", &n); err != nil {
			return "", fmt.Errorf("parse comment id %q: %w", maxID.String, err)
		}
		nextID = fmt.Sprintf("COMMENT-%06d", n+1)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit comment id transaction: %w", err)
	}

	return nextID, nil
}

// Create inserts a new comment. If the ID is empty, it is auto-generated.
func (s *CommentStore) Create(ctx context.Context, c *models.Comment) error {
	if c.ID == "" {
		id, err := s.nextCommentID(ctx)
		if err != nil {
			return fmt.Errorf("generate comment id: %w", err)
		}
		c.ID = id
	}

	now := time.Now().UTC()
	c.CreatedAt = now
	c.UpdatedAt = now

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO comments (id, work_item_id, work_item_type, author_id, author_type, body, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.WorkItemID, c.WorkItemType, c.AuthorID, c.AuthorType, c.Body,
		c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert comment: %w", err)
	}
	return nil
}

// GetByID retrieves a comment by its ID.
func (s *CommentStore) GetByID(ctx context.Context, id string) (*models.Comment, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, work_item_id, work_item_type, author_id, author_type, body, created_at, updated_at
		 FROM comments WHERE id = ?`, id)

	c := &models.Comment{}
	var body sql.NullString
	var createdAt, updatedAt sql.NullTime

	err := row.Scan(
		&c.ID, &c.WorkItemID, &c.WorkItemType, &c.AuthorID, &c.AuthorType,
		&body, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("comment %q: %w", id, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("query comment %q: %w", id, err)
	}

	c.Body = body.String
	if createdAt.Valid {
		c.CreatedAt = createdAt.Time
	}
	if updatedAt.Valid {
		c.UpdatedAt = updatedAt.Time
	}

	return c, nil
}

// GetByWorkItem returns comments for a given work item, ordered by created_at.
func (s *CommentStore) GetByWorkItem(ctx context.Context, workItemID string, workItemType string) ([]*models.Comment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, work_item_id, work_item_type, author_id, author_type, body, created_at, updated_at
		 FROM comments
		 WHERE work_item_id = ? AND work_item_type = ?
		 ORDER BY created_at ASC`, workItemID, workItemType)
	if err != nil {
		return nil, fmt.Errorf("get comments for %s %q: %w", workItemType, workItemID, err)
	}
	defer func() { _ = rows.Close() }()

	return scanComments(rows)
}

// Update modifies a comment. Only the original author may update.
func (s *CommentStore) Update(ctx context.Context, c *models.Comment) error {
	// Verify the author matches the existing comment.
	existing, err := s.GetByID(ctx, c.ID)
	if err != nil {
		return fmt.Errorf("get comment for update: %w", err)
	}
	if existing.AuthorID != c.AuthorID {
		return fmt.Errorf("only the author can update comment %q (author=%q, requester=%q)", c.ID, existing.AuthorID, c.AuthorID)
	}

	c.UpdatedAt = time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE comments SET body=?, updated_at=? WHERE id=?`,
		c.Body, c.UpdatedAt, c.ID,
	)
	if err != nil {
		return fmt.Errorf("update comment %q: %w", c.ID, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected comment %q: %w", c.ID, err)
	}
	if rows == 0 {
		return fmt.Errorf("comment %q: %w", c.ID, sql.ErrNoRows)
	}

	return nil
}

// Delete removes a comment. Only the original author may delete.
func (s *CommentStore) Delete(ctx context.Context, id, authorID string) error {
	// Verify the author matches the existing comment.
	existing, err := s.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get comment for delete: %w", err)
	}
	if existing.AuthorID != authorID {
		return fmt.Errorf("only the author can delete comment %q (author=%q, requester=%q)", id, existing.AuthorID, authorID)
	}

	result, err := s.db.ExecContext(ctx,
		`DELETE FROM comments WHERE id = ?`, id,
	)
	if err != nil {
		return fmt.Errorf("delete comment %q: %w", id, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected delete comment %q: %w", id, err)
	}
	if rows == 0 {
		return fmt.Errorf("comment %q: %w", id, sql.ErrNoRows)
	}

	// Also remove unread entries for this comment.
	_, _ = s.db.ExecContext(ctx, `DELETE FROM unread_comments WHERE comment_id = ?`, id)

	return nil
}

// MarkAsRead removes an unread entry for a session+comment pair.
func (s *CommentStore) MarkAsRead(ctx context.Context, sessionID, commentID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM unread_comments WHERE session_id = ? AND comment_id = ?`,
		sessionID, commentID,
	)
	if err != nil {
		return fmt.Errorf("mark comment %q as read for session %q: %w", commentID, sessionID, err)
	}
	return nil
}

// GetUnreadForSession returns unread comments for tasks assigned to the
// given session.
func (s *CommentStore) GetUnreadForSession(ctx context.Context, sessionID string) ([]*models.Comment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.work_item_id, c.work_item_type, c.author_id, c.author_type, c.body, c.created_at, c.updated_at
		 FROM comments c
		 JOIN unread_comments uc ON uc.comment_id = c.id
		 JOIN tasks t ON t.id = c.work_item_id AND c.work_item_type = 'task'
		 WHERE uc.session_id = ? AND t.assigned_to = ?`,
		sessionID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get unread comments for session %q: %w", sessionID, err)
	}
	defer func() { _ = rows.Close() }()

	return scanComments(rows)
}

// scanComments is a helper to scan multiple comment rows.
func scanComments(rows *sql.Rows) ([]*models.Comment, error) {
	var comments []*models.Comment
	for rows.Next() {
		c := &models.Comment{}
		var body sql.NullString
		var createdAt, updatedAt sql.NullTime

		if err := rows.Scan(
			&c.ID, &c.WorkItemID, &c.WorkItemType, &c.AuthorID, &c.AuthorType,
			&body, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}

		c.Body = body.String
		if createdAt.Valid {
			c.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			c.UpdatedAt = updatedAt.Time
		}

		comments = append(comments, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate comments: %w", err)
	}

	return comments, nil
}
