package store

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

// timeOrZero returns the time value from a sql.NullTime, or the zero time if not valid.
func timeOrZero(t sql.NullTime) time.Time {
	if t.Valid {
		return t.Time
	}
	return time.Time{}
}

// intOrZero returns the int value from a sql.NullInt64, or 0 if not valid.
func intOrZero(n sql.NullInt64) int {
	if n.Valid {
		return int(n.Int64)
	}
	return 0
}

// stringOrZero returns the string value from a sql.NullString, or "" if not valid.
func stringOrZero(s sql.NullString) string {
	if s.Valid {
		return s.String
	}
	return ""
}

// closeRows safely closes a sql.Rows result set and logs any error.
func closeRows(rows *sql.Rows) {
	if err := rows.Close(); err != nil {
		log.Printf("rows close error: %v", err)
	}
}

// requireOneRow checks the result of an Exec operation, verifying that exactly
// one row was affected. It returns ErrNotFound if no rows were affected.
func requireOneRow(result sql.Result, err error, entity, id string) error {
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected %s %q: %w", entity, id, err)
	}
	if rows == 0 {
		return fmt.Errorf("%s %q: %w", entity, id, ErrNotFound)
	}
	return nil
}

// collectRows scans all rows from a query result set using the given scan
// function and returns the collected slice. It handles rows.Next iteration,
// row scanning, and final rows.Err checking.
func collectRows[T any](rows *sql.Rows, scan func(scanner interface{ Scan(...any) error }) (*T, error)) ([]*T, error) {
	var results []*T
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// batchExecTx runs a batch of update operations within a single transaction.
// For each item in the slice, it calls the updateFn with the transaction and item.
// If any updateFn call fails or affects zero rows, the transaction is rolled back
// and the error is returned. On success, the transaction is committed.
func batchExecTx[T any](ctx context.Context, db *sql.DB, items []T, updateFn func(tx *sql.Tx, item T) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	for _, item := range items {
		if execErr := updateFn(tx, item); execErr != nil {
			err = execErr
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
