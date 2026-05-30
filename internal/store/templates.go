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

// TemplateStore provides CRUD operations for prompt templates.
type TemplateStore struct {
	db *sql.DB
}

// NewTemplateStore creates a new TemplateStore.
func NewTemplateStore(db *sql.DB) *TemplateStore {
	return &TemplateStore{db: db}
}

// scanTemplateRow is a helper to scan a prompt template row from a *sql.Row or *sql.Rows.
func scanTemplateRow(scanner interface{ Scan(...any) error }) (*models.PromptTemplate, error) {
	t := &models.PromptTemplate{}
	var createdAt, updatedAt sql.NullTime

	err := scanner.Scan(&t.ID, &t.TaskType, &t.Template, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	t.CreatedAt = timeOrZero(createdAt)
	t.UpdatedAt = timeOrZero(updatedAt)

	return t, nil
}

// Create inserts a new prompt template. If the ID is empty, a UUID is generated.
func (s *TemplateStore) Create(ctx context.Context, t *models.PromptTemplate) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}

	now := time.Now().UTC()
	t.CreatedAt = now
	t.UpdatedAt = now

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO prompt_templates (id, task_type, template, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		t.ID, t.TaskType, t.Template, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert prompt template: %w", err)
	}
	return nil
}

// GetByTaskType retrieves a prompt template by its task type.
func (s *TemplateStore) GetByTaskType(ctx context.Context, taskType models.TaskType) (*models.PromptTemplate, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, task_type, template, created_at, updated_at
		 FROM prompt_templates WHERE task_type = ?`, taskType)

	t, err := scanTemplateRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("template for task_type %q: %w", taskType, ErrNotFound)
		}
		return nil, fmt.Errorf("query template by task_type %q: %w", taskType, err)
	}

	return t, nil
}

// List returns all prompt templates.
func (s *TemplateStore) List(ctx context.Context) ([]*models.PromptTemplate, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, task_type, template, created_at, updated_at
		 FROM prompt_templates ORDER BY task_type`)
	if err != nil {
		return nil, fmt.Errorf("list prompt templates: %w", err)
	}
	defer closeRows(rows)

	templates, err := collectRows(rows, scanTemplateRow)
	if err != nil {
		return nil, fmt.Errorf("scan prompt templates: %w", err)
	}
	return templates, nil
}

// Upsert creates or updates a prompt template by task_type.
func (s *TemplateStore) Upsert(ctx context.Context, t *models.PromptTemplate) error {
	now := time.Now().UTC()

	if t.ID == "" {
		// Check if a template already exists for this task type and use its ID.
		var existingID string
		err := s.db.QueryRowContext(ctx,
			`SELECT id FROM prompt_templates WHERE task_type = ?`, t.TaskType,
		).Scan(&existingID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("check existing template for task_type %q: %w", t.TaskType, err)
		}
		if existingID != "" {
			t.ID = existingID
		} else {
			t.ID = uuid.New().String()
		}
	}

	t.CreatedAt = now
	t.UpdatedAt = now

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO prompt_templates (id, task_type, template, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(task_type) DO UPDATE SET template=excluded.template, updated_at=excluded.updated_at`,
		t.ID, t.TaskType, t.Template, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert template for task_type %q: %w", t.TaskType, err)
	}

	return nil
}

// Delete removes a prompt template by ID.
func (s *TemplateStore) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM prompt_templates WHERE id = ?`, id,
	)
	if err != nil {
		return fmt.Errorf("delete template %q: %w", id, err)
	}
	return requireOneRow(result, nil, "template", id)
}
