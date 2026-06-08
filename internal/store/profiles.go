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
	db      *sql.DB
	ptStore *ProfileTaskTypeStore
}

func NewAgentProfileStore(db *sql.DB) *AgentProfileStore {
	return &AgentProfileStore{
		db:      db,
		ptStore: NewProfileTaskTypeStore(db),
	}
}

func (s *AgentProfileStore) Create(ctx context.Context, profile *models.AgentProfile) error {
	if profile.ID == "" {
		profile.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	profile.CreatedAt = now
	profile.UpdatedAt = now

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx,
		`INSERT INTO agent_profiles (id, name, description, capabilities, max_concurrency, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		profile.ID, profile.Name, profile.Description, profile.Capabilities,
		profile.MaxConcurrency, profile.CreatedAt, profile.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert agent profile: %w", err)
	}

	for _, tt := range profile.TaskTypes {
		if tt == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO profile_task_types (profile_id, task_type) VALUES (?, ?)`,
			profile.ID, tt); err != nil {
			return fmt.Errorf("insert profile_task_type %q: %w", tt, err)
		}
	}

	return tx.Commit()
}

func (s *AgentProfileStore) GetByID(ctx context.Context, id string) (*models.AgentProfile, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, description, capabilities, max_concurrency, created_at, updated_at
		 FROM agent_profiles WHERE id = ?`, id)
	profile, err := scanAgentProfileRow(row)
	if err != nil {
		return nil, err
	}

	// Load task types
	taskTypes, err := s.ptStore.GetByProfileID(ctx, profile.ID)
	if err != nil {
		return nil, fmt.Errorf("load task types for profile %q: %w", profile.ID, err)
	}
	profile.TaskTypes = taskTypes

	return profile, nil
}

func (s *AgentProfileStore) List(ctx context.Context) ([]*models.AgentProfile, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, description, capabilities, max_concurrency, created_at, updated_at
		 FROM agent_profiles ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list agent profiles: %w", err)
	}
	defer closeRows(rows)

	profiles, err := collectRows(rows, scanAgentProfileRow)
	if err != nil {
		return nil, err
	}

	// Batch-load task types for all profiles to avoid N+1 queries.
	if len(profiles) > 0 {
		profileIDs := make([]string, len(profiles))
		for i, p := range profiles {
			profileIDs[i] = p.ID
		}

		taskTypesMap, err := s.ptStore.GetByProfileIDs(ctx, profileIDs)
		if err != nil {
			return nil, fmt.Errorf("batch load task types: %w", err)
		}

		for _, p := range profiles {
			if types, ok := taskTypesMap[p.ID]; ok {
				p.TaskTypes = types
			}
		}
	}

	return profiles, nil
}

func (s *AgentProfileStore) Update(ctx context.Context, profile *models.AgentProfile) error {
	profile.UpdatedAt = time.Now().UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx,
		`UPDATE agent_profiles SET name=?, description=?, capabilities=?, max_concurrency=?, updated_at=?
		 WHERE id=?`,
		profile.Name, profile.Description, profile.Capabilities,
		profile.MaxConcurrency, profile.UpdatedAt, profile.ID)
	if err != nil {
		return fmt.Errorf("update agent profile %q: %w", profile.ID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("agent_profile %q: %w", profile.ID, ErrNotFound)
	}

	// Delete existing and re-insert task types
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM profile_task_types WHERE profile_id = ?`, profile.ID); err != nil {
		return fmt.Errorf("delete profile_task_types: %w", err)
	}

	for _, tt := range profile.TaskTypes {
		if tt == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO profile_task_types (profile_id, task_type) VALUES (?, ?)`,
			profile.ID, tt); err != nil {
			return fmt.Errorf("insert profile_task_type %q: %w", tt, err)
		}
	}

	return tx.Commit()
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
