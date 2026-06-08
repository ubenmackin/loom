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

// ProjectStore provides CRUD operations for projects.
type ProjectStore struct {
	db *sql.DB
}

// NewProjectStore creates a new ProjectStore.
func NewProjectStore(db *sql.DB) *ProjectStore {
	return &ProjectStore{db: db}
}

// Create inserts a new project.
func (s *ProjectStore) Create(ctx context.Context, project *models.Project) error {
	if project.ID == "" {
		project.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	project.CreatedAt = now
	project.UpdatedAt = now

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO projects (id, name, description, repo_path, language, build_command, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		project.ID, project.Name, project.Description, project.RepoPath,
		project.Language, project.BuildCommand, project.CreatedAt, project.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert project: %w", err)
	}
	return nil
}

// GetByID retrieves a project by its ID.
func (s *ProjectStore) GetByID(ctx context.Context, id string) (*models.Project, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, repo_path, language, build_command, created_at, updated_at
		 FROM projects WHERE id = ?`, id)

	return scanProjectRow(row)
}

// List returns all projects ordered by name.
func (s *ProjectStore) List(ctx context.Context) ([]*models.Project, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, repo_path, language, build_command, created_at, updated_at
		 FROM projects ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer closeRows(rows)
	return collectRows(rows, scanProjectRow)
}

// Update saves all mutable fields of a project.
func (s *ProjectStore) Update(ctx context.Context, project *models.Project) error {
	project.UpdatedAt = time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE projects SET name=?, description=?, repo_path=?, language=?, build_command=?, updated_at=?
		 WHERE id=?`,
		project.Name, project.Description, project.RepoPath,
		project.Language, project.BuildCommand, project.UpdatedAt, project.ID,
	)
	if err != nil {
		return fmt.Errorf("update project %q: %w", project.ID, err)
	}
	return requireOneRow(result, nil, "project", project.ID)
}

// Delete removes a project by ID. Stories referencing this project will
// have their project_id set to NULL due to the ON DELETE SET NULL FK constraint.
func (s *ProjectStore) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete project %q: %w", id, err)
	}
	return requireOneRow(result, nil, "project", id)
}

// scanProjectRow scans a project row from a *sql.Row or *sql.Rows.
func scanProjectRow(scanner interface{ Scan(...any) error }) (*models.Project, error) {
	p := &models.Project{}
	var desc, repoPath, language, buildCommand sql.NullString
	var createdAt, updatedAt sql.NullTime

	err := scanner.Scan(
		&p.ID, &p.Name, &desc, &repoPath, &language, &buildCommand,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("project: %w", ErrNotFound)
		}
		return nil, err
	}

	p.Description = stringOrZero(desc)
	p.RepoPath = stringOrZero(repoPath)
	p.Language = stringOrZero(language)
	p.BuildCommand = stringOrZero(buildCommand)
	p.CreatedAt = timeOrZero(createdAt)
	p.UpdatedAt = timeOrZero(updatedAt)

	return p, nil
}
