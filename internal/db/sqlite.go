// Package db provides SQLite database connectivity and migration for Loom.
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"strings"

	_ "modernc.org/sqlite"
)

// migrationsFS embeds the SQL migration files.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open opens a SQLite database at the given path with pragmas configured for
// concurrent access: WAL journal mode, busy timeout, and foreign keys enabled.
// The _txlock=immediate DSN parameter ensures all transactions use BEGIN
// IMMEDIATE by default, which acquires write locks at transaction start rather
// than on the first write. This prevents TOCTOU races in MAX+1 ID generation.
func Open(dbPath string) (*sql.DB, error) {
	dsn := dbPath + "?_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", dbPath, err)
	}

	// Configure pragmas for safe concurrent access.
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return nil, fmt.Errorf("set pragma %q: %w", p, err)
		}
	}

	return db, nil
}

// runMigration applies a single migration file within its own transaction.
// This function is called from the migration loop so that each transaction's
// defer runs before the next iteration, avoiding the defer-in-loop resource leak.
func runMigration(db *sql.DB, entry fs.DirEntry) error {
	// Check if this migration has already been applied.
	var exists int
	err := db.QueryRow("SELECT 1 FROM schema_migrations WHERE version = ?", entry.Name()).Scan(&exists)
	if err == nil {
		log.Printf("Skipping already-applied migration: %s", entry.Name())
		return nil
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("check migration %s: %w", entry.Name(), err)
	}

	data, err := migrationsFS.ReadFile("migrations/" + entry.Name())
	if err != nil {
		return fmt.Errorf("read migration %s: %w", entry.Name(), err)
	}

	log.Printf("Running migration: %s", entry.Name())

	// Execute the migration and tracking insert in a single transaction
	// so a failed migration does not get recorded as applied.
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration transaction %s: %w", entry.Name(), err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(string(data)); err != nil {
		return fmt.Errorf("exec migration %s: %w", entry.Name(), err)
	}

	if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", entry.Name()); err != nil {
		return fmt.Errorf("record migration %s: %w", entry.Name(), err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", entry.Name(), err)
	}

	return nil
}

// Migrate reads and executes all embedded SQL migration files in order.
// Each migration is tracked in a schema_migrations table so it only runs once.
func Migrate(db *sql.DB) error {
	// Bootstrap the tracking table before any migrations run.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version   TEXT PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		if err := runMigration(db, entry); err != nil {
			return err
		}
	}

	return nil
}

// BackfillNumericIDs checks if there are any stories or tasks with NULL or 0 numeric_id,
// and populates them sequentially from the work_item_sequence table.
func BackfillNumericIDs(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin backfill transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 1. Find all stories with NULL or 0 numeric_id, ordered by creation date or string id
	rows, err := tx.Query("SELECT id FROM stories WHERE numeric_id IS NULL OR numeric_id = 0 ORDER BY created_at, id")
	if err != nil {
		return fmt.Errorf("query unassigned stories: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var storyIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan unassigned story id: %w", err)
		}
		storyIDs = append(storyIDs, id)
	}
	_ = rows.Close()

	// Update each story with a new sequence ID
	for _, id := range storyIDs {
		res, err := tx.Exec("INSERT INTO work_item_sequence (type) VALUES ('story')")
		if err != nil {
			return fmt.Errorf("insert work item sequence for story: %w", err)
		}
		numID, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("get last insert id for story: %w", err)
		}
		_, err = tx.Exec("UPDATE stories SET numeric_id = ? WHERE id = ?", numID, id)
		if err != nil {
			return fmt.Errorf("update story numeric id: %w", err)
		}
		log.Printf("Backfilled story %s with numeric_id %d", id, numID)
	}

	// 2. Find all tasks with NULL or 0 numeric_id, ordered by creation date or string id
	rows, err = tx.Query("SELECT id FROM tasks WHERE numeric_id IS NULL OR numeric_id = 0 ORDER BY created_at, id")
	if err != nil {
		return fmt.Errorf("query unassigned tasks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var taskIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan unassigned task id: %w", err)
		}
		taskIDs = append(taskIDs, id)
	}
	_ = rows.Close()

	// Update each task with a new sequence ID
	for _, id := range taskIDs {
		res, err := tx.Exec("INSERT INTO work_item_sequence (type) VALUES ('task')")
		if err != nil {
			return fmt.Errorf("insert work item sequence for task: %w", err)
		}
		numID, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("get last insert id for task: %w", err)
		}
		_, err = tx.Exec("UPDATE tasks SET numeric_id = ? WHERE id = ?", numID, id)
		if err != nil {
			return fmt.Errorf("update task numeric id: %w", err)
		}
		log.Printf("Backfilled task %s with numeric_id %d", id, numID)
	}

	return tx.Commit()
}
