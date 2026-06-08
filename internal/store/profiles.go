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

// AgentProfileStore provides CRUD operations for agent profiles.
type AgentProfileStore struct {
	db *sql.DB
}

func NewAgentProfileStore(db *sql.DB) *AgentProfileStore {
	return &AgentProfileStore{db: db}
}

func (s *AgentProfileStore) Create(ctx context.Context, profile *models.AgentProfile) error {
	if profile.ID == "" {
		profile.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	profile.CreatedAt = now
	profile.UpdatedAt = now

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agent_profiles (id, name, description, capabilities, max_concurrency, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		profile.ID, profile.Name, profile.Description, profile.Capabilities,
		profile.MaxConcurrency, profile.CreatedAt, profile.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert agent profile: %w", err)
	}
	return nil
}

func (s *AgentProfileStore) GetByID(ctx context.Context, id string) (*models.AgentProfile, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, capabilities, max_concurrency, created_at, updated_at
		 FROM agent_profiles WHERE id = ?`, id)
	return scanAgentProfileRow(row)
}

func (s *AgentProfileStore) List(ctx context.Context) ([]*models.AgentProfile, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, capabilities, max_concurrency, created_at, updated_at
		 FROM agent_profiles ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list agent profiles: %w", err)
	}
	defer closeRows(rows)
	return collectRows(rows, scanAgentProfileRow)
}

func (s *AgentProfileStore) Update(ctx context.Context, profile *models.AgentProfile) error {
	profile.UpdatedAt = time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE agent_profiles SET name=?, description=?, capabilities=?, max_concurrency=?, updated_at=?
		 WHERE id=?`,
		profile.Name, profile.Description, profile.Capabilities,
		profile.MaxConcurrency, profile.UpdatedAt, profile.ID)
	if err != nil {
		return fmt.Errorf("update agent profile %q: %w", profile.ID, err)
	}
	return requireOneRow(result, nil, "agent_profile", profile.ID)
}

func (s *AgentProfileStore) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM agent_profiles WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete agent profile %q: %w", id, err)
	}
	return requireOneRow(result, nil, "agent_profile", id)
}

func scanAgentProfileRow(scanner interface{ Scan(...any) error }) (*models.AgentProfile, error) {
	p := &models.AgentProfile{}
	var desc, capabilities sql.NullString
	var createdAt, updatedAt sql.NullTime

	err := scanner.Scan(
		&p.ID, &p.Name, &desc, &capabilities,
		&p.MaxConcurrency, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("agent_profile: %w", ErrNotFound)
		}
		return nil, err
	}

	p.Description = stringOrZero(desc)
	p.Capabilities = stringOrZero(capabilities)
	p.CreatedAt = timeOrZero(createdAt)
	p.UpdatedAt = timeOrZero(updatedAt)
	return p, nil
}
