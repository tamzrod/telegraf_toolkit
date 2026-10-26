package metadatastore

import (
	"context"
	"database/sql"
	"strings"
)

// execer is the subset of *sql.DB and *sql.Tx the repositories use. Holding
// the narrower interface lets one repository implementation run against the
// connection pool or inside a transaction, which is what makes an all-or-
// nothing registry write possible (P2C-002).
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

// isUniqueViolation reports whether an error is a SQLite uniqueness failure.
// The modernc driver reports it in the message rather than a typed code, so the
// message is inspected; the check is deliberately narrow.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToUpper(err.Error())
	return strings.Contains(msg, "UNIQUE CONSTRAINT FAILED") || strings.Contains(msg, "CONSTRAINT FAILED: UNIQUE")
}
