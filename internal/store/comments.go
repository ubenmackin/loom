package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/ubenmackin/loom/internal/models"
)

// CommentStore provides CRUD operations for comments.
type CommentStore struct {
	db *sql.DB
}

// nextCommentID generates the next comment ID in the format COMMENT-NNNNNN.
func (s *CommentStore) nextCommentID(ctx context.Context) (string, error) {
	res, err := s.db.ExecContext(ctx, "INSERT INTO work_item_sequence (type) VALUES ('comment')")
	if err != nil {
		return "", fmt.Errorf("generate comment id: %w", err)
	}
	seqID, err := res.LastInsertId()
	if err != nil {
		return "", fmt.Errorf("get last insert id for comment: %w", err)
	}
	return fmt.Sprintf("COMMENT-%06d", seqID), nil
}

// NewCommentStore creates a new CommentStore.
func NewCommentStore(db *sql.DB) *CommentStore {
	return &CommentStore{db: db}
}

// scanCommentRow is a helper to scan a comment row from a *sql.Row or *sql.Rows.
func scanCommentRow(scanner interface{ Scan(...any) error }) (*models.Comment, error) {
	c := &models.Comment{}
	var body sql.NullString
	var createdAt, updatedAt sql.NullTime

	err := scanner.Scan(
		&c.ID, &c.WorkItemID, &c.WorkItemType, &c.AuthorID, &c.AuthorType,
		&body, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	c.Body = stringOrZero(body)
	c.CreatedAt = timeOrZero(createdAt)
	c.UpdatedAt = timeOrZero(updatedAt)

	return c, nil
}

// Create inserts a new comment. If the ID is empty, it is auto-generated.
// It mutates the pointer to set ID, CreatedAt, and UpdatedAt.
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

	c, err := scanCommentRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("comment %q: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("query comment %q: %w", id, err)
	}

	return c, nil
}

// GetByWorkItem returns comments for a given work item, ordered by created_at.
func (s *CommentStore) GetByWorkItem(ctx context.Context, workItemID string, workItemType models.WorkItemType) ([]*models.Comment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, work_item_id, work_item_type, author_id, author_type, body, created_at, updated_at
		 FROM comments
		 WHERE work_item_id = ? AND work_item_type = ?
		 ORDER BY created_at ASC`, workItemID, workItemType)
	if err != nil {
		return nil, fmt.Errorf("get comments for %s %q: %w", workItemType, workItemID, err)
	}
	defer closeRows(rows)

	return collectRows(rows, scanCommentRow)
}

// Update modifies a comment. Only the original author may update.
func (s *CommentStore) Update(ctx context.Context, c *models.Comment) error {
	existing, err := s.GetByID(ctx, c.ID)
	if err != nil {
		return fmt.Errorf("get comment for update: %w", err)
	}
	if existing.AuthorID != c.AuthorID {
		return fmt.Errorf("%w: only the author can update comment %q (author=%q, requester=%q)", ErrUnauthorizedAuthor, c.ID, existing.AuthorID, c.AuthorID)
	}

	c.UpdatedAt = time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE comments SET body=?, updated_at=? WHERE id=?`,
		c.Body, c.UpdatedAt, c.ID,
	)
	if err != nil {
		return fmt.Errorf("update comment %q: %w", c.ID, err)
	}

	return requireOneRow(result, nil, "comment", c.ID)
}

// Delete removes a comment. Only the original author may delete.
func (s *CommentStore) Delete(ctx context.Context, id, authorID string) error {
	existing, err := s.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get comment for delete: %w", err)
	}
	if existing.AuthorID != authorID {
		return fmt.Errorf("%w: only the author can delete comment %q (author=%q, requester=%q)", ErrUnauthorizedAuthor, id, existing.AuthorID, authorID)
	}

	result, err := s.db.ExecContext(ctx,
		`DELETE FROM comments WHERE id = ?`, id,
	)
	if err != nil {
		return fmt.Errorf("delete comment %q: %w", id, err)
	}

	if err := requireOneRow(result, nil, "comment", id); err != nil {
		return err
	}

	if _, err := s.db.ExecContext(ctx, `DELETE FROM unread_comments WHERE comment_id = ?`, id); err != nil {
		log.Printf("cleanup unread for comment %q: %v", id, err)
	}

	return nil
}

// MarkAsRead removes an unread entry for a session+comment pair.
func (s *CommentStore) MarkAsRead(ctx context.Context, sessionID, commentID string) error {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM unread_comments WHERE session_id = ? AND comment_id = ?`,
		sessionID, commentID,
	)
	if err != nil {
		return fmt.Errorf("mark comment %q as read for session %q: %w", commentID, sessionID, err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		log.Printf("MarkAsRead: no unread entry for session %q comment %q", sessionID, commentID)
	}
	return nil
}

// GetUnreadForSession returns unread comments for tasks assigned to the given session.
func (s *CommentStore) GetUnreadForSession(ctx context.Context, sessionID string) ([]*models.Comment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.work_item_id, c.work_item_type, c.author_id, c.author_type, c.body, c.created_at, c.updated_at
		 FROM comments c
		 JOIN unread_comments uc ON uc.comment_id = c.id
		 JOIN tasks t ON t.id = c.work_item_id AND c.work_item_type = ?
		 WHERE uc.session_id = ? AND t.assigned_to = ?`,
		models.WorkItemTypeTask, sessionID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get unread comments for session %q: %w", sessionID, err)
	}
	defer closeRows(rows)

	return collectRows(rows, scanCommentRow)
}

// GetUnreadForSessionByWorkItem returns unread comments for a session on a specific work item.
func (s *CommentStore) GetUnreadForSessionByWorkItem(ctx context.Context, sessionID, workItemID string, workItemType models.WorkItemType) ([]*models.Comment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c.id, c.work_item_id, c.work_item_type, c.author_id, c.author_type, c.body, c.created_at, c.updated_at
		 FROM comments c
		 JOIN unread_comments uc ON uc.comment_id = c.id
		 WHERE uc.session_id = ? AND c.work_item_id = ? AND c.work_item_type = ?`,
		sessionID, workItemID, workItemType)
	if err != nil {
		return nil, fmt.Errorf("get unread comments for session %q on %s %q: %w", sessionID, workItemType, workItemID, err)
	}
	defer closeRows(rows)
	return collectRows(rows, scanCommentRow)
}
