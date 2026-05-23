package store

import (
	"database/sql"
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
