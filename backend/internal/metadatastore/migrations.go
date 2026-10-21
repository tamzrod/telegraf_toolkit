// Package metadatastore implements the persistent Toolkit control-plane
// metadata store selected by ADR-0002 (Option A: SQLite) for the initial
// single-host deployment.
//
// Scope and limits (ADR-0002):
//   - The store holds credential *references* only. It never holds a secret
//     value; envelope encryption and key custody are ADR-0003 and are not
//     implemented here.
//   - It is reachable only from the backend process. No HTTP route exposes it.
//   - Multi-host management is out of scope; SQLite's single-writer limit is
//     documented in docs/METADATA_STORE.md.
//
// The storage boundary is kept behind small repository types so a future
// PostgreSQL adapter can replace the SQLite implementation without changing
// callers (ADR-0002 "Accepted decision and implementation limits").
package metadatastore

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// migration is one versioned, forward-only schema change. Migrations are
// applied in ascending version order, each inside its own transaction, so a
// partially applied migration cannot leave the schema half-changed.
type migration struct {
	version int
	name    string
	stmts   []string
}

// migrations is the committed migration set. It is append-only: an existing
// version must never be edited once released, because a deployed database has
// already recorded that version as applied. Add a new version instead.
var migrations = []migration{
	{
		version: 1,
		name:    "initial-control-plane-metadata",
		stmts: []string{
			// schema_migrations is created by ensureMigrationTable before any
			// migration runs, so it is not repeated here.
			// connection_profiles stores profile metadata plus a credential
			// *reference* (credential_ref_id). There is deliberately no column
			// for a credential value.
			`CREATE TABLE connection_profiles (
				connection_id          TEXT PRIMARY KEY,
				display_name           TEXT NOT NULL,
				family                 TEXT NOT NULL,
				major_version          INTEGER NOT NULL,
				scope_kind             TEXT NOT NULL,
				scope_database         TEXT NOT NULL DEFAULT '',
				scope_retention_policy TEXT NOT NULL DEFAULT '',
				scope_organization     TEXT NOT NULL DEFAULT '',
				scope_bucket           TEXT NOT NULL DEFAULT '',
				query_dialects         TEXT NOT NULL,
				write_protocols        TEXT NOT NULL,
				schema_discovery       INTEGER NOT NULL,
				credential_ref_id      TEXT NOT NULL,
				mock                   INTEGER NOT NULL,
				note                   TEXT NOT NULL DEFAULT ''
			)`,
			// credential_refs holds non-secret reference metadata only.
			`CREATE TABLE credential_refs (
				credential_ref_id TEXT PRIMARY KEY,
				alias             TEXT NOT NULL,
				role              TEXT NOT NULL,
				fingerprint       TEXT NOT NULL DEFAULT '',
				last_rotated      TEXT,
				expires_at        TEXT,
				mock              INTEGER NOT NULL
			)`,
			// audit_events is append-oriented. It records references, never
			// values.
			`CREATE TABLE audit_events (
				id             INTEGER PRIMARY KEY AUTOINCREMENT,
				actor          TEXT NOT NULL,
				action         TEXT NOT NULL,
				target_kind    TEXT NOT NULL,
				target_id      TEXT NOT NULL,
				outcome        TEXT NOT NULL,
				at             TEXT NOT NULL,
				correlation_id TEXT NOT NULL DEFAULT '',
				mock           INTEGER NOT NULL
			)`,
		},
	},
	{
		version: 2,
		name:    "audit-and-profile-lookup-indexes",
		stmts: []string{
			`CREATE INDEX idx_audit_events_target ON audit_events (target_kind, target_id)`,
			`CREATE INDEX idx_audit_events_at ON audit_events (at)`,
			`CREATE INDEX idx_connection_profiles_family ON connection_profiles (family)`,
		},
	},
	{
		version: 3,
		name:    "encrypted-secret-envelopes",
		stmts: []string{
			// secret_envelopes holds authenticated ciphertext envelopes only.
			// The encryption keys live in a separately provisioned key file
			// outside this database (ADR-0003), so a metadata backup can never
			// decrypt anything. The `key_id` column records which key wrapped
			// the envelope so rotation can find envelopes still on an old key.
			`CREATE TABLE secret_envelopes (
				credential_ref_id TEXT PRIMARY KEY,
				key_id            TEXT NOT NULL,
				envelope          TEXT NOT NULL,
				updated_at        TEXT NOT NULL
			)`,
			`CREATE INDEX idx_secret_envelopes_key_id ON secret_envelopes (key_id)`,
		},
	},
	{
		version: 4,
		name:    "profile-endpoint",
		stmts: []string{
			// endpoint is the connection target the profile identifies. Storing
			// it makes the endpoint an immutable part of the profile, so a probe
			// resolves its destination from the profile server-side instead of
			// accepting a caller-supplied host (P2L-006, F-02). Existing rows
			// default to an empty endpoint and are treated as unprobeable until
			// re-saved.
			`ALTER TABLE connection_profiles ADD COLUMN endpoint TEXT NOT NULL DEFAULT ''`,
		},
	},
	{
		version: 5,
		name:    "credential-bindings",
		stmts: []string{
			// credential_bindings records the server-authoritative approval that
			// a credential reference may be used with a specific profile
			// identity (connection ID, endpoint, canonical scope). A probe
			// resolves a credential only when a matching binding exists, so a
			// principal with profile.write cannot attach another principal's
			// stored secret to an endpoint of its choosing (P2L-007). No secret
			// value is stored here.
			`CREATE TABLE credential_bindings (
				credential_ref_id TEXT NOT NULL,
				connection_id     TEXT NOT NULL,
				endpoint          TEXT NOT NULL,
				scope             TEXT NOT NULL,
				approved_by       TEXT NOT NULL DEFAULT '',
				approved_at       TEXT NOT NULL,
				PRIMARY KEY (credential_ref_id, connection_id)
			)`,
			`CREATE INDEX idx_credential_bindings_connection ON credential_bindings (connection_id)`,
		},
	},
}

// SchemaVersion is the schema version this build expects after migration.
func SchemaVersion() int { return migrations[len(migrations)-1].version }

// Migrate applies every migration newer than the database's recorded version,
// in order, and returns the resulting schema version. It is safe to call on a
// fresh database (all migrations apply) and on an up-to-date one (no-op).
func Migrate(ctx context.Context, db *sql.DB) (int, error) {
	return applyMigrations(ctx, db, SchemaVersion())
}

// applyMigrations applies pending migrations up to and including target. It is
// used by tests to build an intermediate-version database and prove a later
// migration preserves existing rows.
func applyMigrations(ctx context.Context, db *sql.DB, target int) (int, error) {
	if err := ensureMigrationTable(ctx, db); err != nil {
		return 0, err
	}
	current, err := currentVersion(ctx, db)
	if err != nil {
		return 0, err
	}
	if current > target {
		return current, fmt.Errorf("database schema version %d is newer than requested target %d", current, target)
	}
	for _, m := range migrations {
		if m.version <= current || m.version > target {
			continue
		}
		if err := applyOne(ctx, db, m); err != nil {
			return current, err
		}
		current = m.version
	}
	return current, nil
}

func ensureMigrationTable(ctx context.Context, db *sql.DB) error {
	const stmt = `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		name       TEXT NOT NULL,
		applied_at TEXT NOT NULL
	)`
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	return nil
}

func currentVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version sql.NullInt64
	row := db.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`)
	if err := row.Scan(&version); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	if !version.Valid {
		return 0, nil
	}
	return int(version.Int64), nil
}

// applyOne runs a single migration and its version record in one transaction.
func applyOne(ctx context.Context, db *sql.DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", m.version, err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range m.stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migration %d (%s): %w", m.version, m.name, err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
		m.version, m.name, time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("record migration %d: %w", m.version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %d: %w", m.version, err)
	}
	return nil
}
