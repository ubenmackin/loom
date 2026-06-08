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

// TriggerRuleStore provides CRUD operations for trigger rules.
type TriggerRuleStore struct {
	db *sql.DB
}

func NewTriggerRuleStore(db *sql.DB) *TriggerRuleStore {
	return &TriggerRuleStore{db: db}
}

func (s *TriggerRuleStore) Create(ctx context.Context, rule *models.TriggerRule) error {
	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	rule.CreatedAt = now

	enabled := 0
	if rule.Enabled {
		enabled = 1
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO trigger_rules (id, agent_profile_id, event_type, action, priority, enabled, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rule.ID, rule.AgentProfileID, rule.EventType, rule.Action,
		rule.Priority, enabled, rule.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert trigger rule: %w", err)
	}
	return nil
}

func (s *TriggerRuleStore) GetByID(ctx context.Context, id string) (*models.TriggerRule, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, agent_profile_id, event_type, action, priority, enabled, created_at
		 FROM trigger_rules WHERE id = ?`, id)
	return scanTriggerRuleRow(row)
}

func (s *TriggerRuleStore) ListByProfile(ctx context.Context, profileID string) ([]*models.TriggerRule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, agent_profile_id, event_type, action, priority, enabled, created_at
		 FROM trigger_rules WHERE agent_profile_id = ? ORDER BY priority`, profileID)
	if err != nil {
		return nil, fmt.Errorf("list trigger rules by profile: %w", err)
	}
	defer closeRows(rows)
	return collectRows(rows, scanTriggerRuleRow)
}

func (s *TriggerRuleStore) List(ctx context.Context) ([]*models.TriggerRule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, agent_profile_id, event_type, action, priority, enabled, created_at
		 FROM trigger_rules ORDER BY agent_profile_id, priority`)
	if err != nil {
		return nil, fmt.Errorf("list trigger rules: %w", err)
	}
	defer closeRows(rows)
	return collectRows(rows, scanTriggerRuleRow)
}

func (s *TriggerRuleStore) Update(ctx context.Context, rule *models.TriggerRule) error {
	enabled := 0
	if rule.Enabled {
		enabled = 1
	}
	result, err := s.db.ExecContext(ctx,
		`UPDATE trigger_rules SET event_type=?, action=?, priority=?, enabled=?
		 WHERE id=?`,
		rule.EventType, rule.Action, rule.Priority, enabled, rule.ID)
	if err != nil {
		return fmt.Errorf("update trigger rule %q: %w", rule.ID, err)
	}
	return requireOneRow(result, nil, "trigger_rule", rule.ID)
}

func (s *TriggerRuleStore) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM trigger_rules WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete trigger rule %q: %w", id, err)
	}
	return requireOneRow(result, nil, "trigger_rule", id)
}

func scanTriggerRuleRow(scanner interface{ Scan(...any) error }) (*models.TriggerRule, error) {
	rule := &models.TriggerRule{}
	var createdAt sql.NullTime
	var enabled int

	err := scanner.Scan(
		&rule.ID, &rule.AgentProfileID, &rule.EventType, &rule.Action,
		&rule.Priority, &enabled, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("trigger_rule: %w", ErrNotFound)
		}
		return nil, err
	}

	rule.Enabled = enabled == 1
	rule.CreatedAt = timeOrZero(createdAt)
	return rule, nil
}
