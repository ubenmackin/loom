// Package testhelpers provides shared test database setup for the Loom test suite.
package testhelpers

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/ubenmackin/loom/internal/db"
)

// SetupTestDB creates an in-memory SQLite database with a unique name per test,
// runs migrations, and returns the database connection. The database is closed
// when the test completes.
func SetupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	// Use a unique name per test to avoid shared state between parallel tests.
	dbName := t.Name()
	dsn := "file:" + dbName + "?mode=memory&cache=shared"

	dbConn, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	// Enable foreign keys for the test connection.
	if _, err := dbConn.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}

	if err := db.Migrate(dbConn); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}

	t.Cleanup(func() {
		_ = dbConn.Close()
	})

	return dbConn
}
