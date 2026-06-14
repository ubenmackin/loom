package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// SettingKeyGlobalMaxConcurrency is the settings table key for the global
// max concurrency cap on the gateway's job queue.
const SettingKeyGlobalMaxConcurrency = "global_max_concurrency"

// SettingStore provides a generic key-value store for application settings.
type SettingStore struct {
	db *sql.DB
}

// NewSettingStore creates a new SettingStore.
func NewSettingStore(db *sql.DB) *SettingStore {
	return &SettingStore{db: db}
}

// Get retrieves the value for the given key. Returns ErrNotFound if the key
// does not exist.
func (s *SettingStore) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx,
		`SELECT value FROM settings WHERE key = ?`, key,
	).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("setting %q: %w", key, ErrNotFound)
		}
		return "", fmt.Errorf("get setting %q: %w", key, err)
	}
	return value, nil
}

// Set inserts or updates the value for the given key.
func (s *SettingStore) Set(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT (key) DO UPDATE SET value = ?`,
		key, value, value,
	)
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}

// Delete removes the setting for the given key. Returns ErrNotFound if the key
// does not exist.
func (s *SettingStore) Delete(ctx context.Context, key string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM settings WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("delete setting %q: %w", key, err)
	}
	return requireOneRow(result, nil, "setting", key)
}
