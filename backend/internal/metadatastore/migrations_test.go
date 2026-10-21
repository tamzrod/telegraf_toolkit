package metadatastore

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// openTemp opens a store on a fresh file under t.TempDir() so each test is
// isolated and the file genuinely persists across a close/reopen.
func openTemp(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "metadata.db")
	store, err := Open(context.Background(), Options{Path: path})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestMigrateFreshDatabaseReachesCurrentVersion(t *testing.T) {
	store := openTemp(t)

	version, err := store.Version(context.Background())
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if version != SchemaVersion() {
		t.Fatalf("schema version = %d, want %d", version, SchemaVersion())
	}
	if SchemaVersion() < 2 {
		t.Fatalf("expected at least two migrations, got %d", SchemaVersion())
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	store := openTemp(t)
	ctx := context.Background()

	first, err := Migrate(ctx, store.DB())
	if err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	second, err := Migrate(ctx, store.DB())
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if first != second || second != SchemaVersion() {
		t.Fatalf("re-migrate changed version: %d then %d, want %d", first, second, SchemaVersion())
	}
}

func TestMigrationsAreOrderedAndUnique(t *testing.T) {
	seen := map[int]bool{}
	previous := 0
	for _, m := range migrations {
		if m.version <= previous {
			t.Fatalf("migration versions must strictly increase: %d after %d", m.version, previous)
		}
		if seen[m.version] {
			t.Fatalf("duplicate migration version %d", m.version)
		}
		if m.name == "" || len(m.stmts) == 0 {
			t.Fatalf("migration %d must have a name and statements", m.version)
		}
		seen[m.version] = true
		previous = m.version
	}
	if previous != SchemaVersion() {
		t.Fatalf("SchemaVersion() = %d, want last migration %d", SchemaVersion(), previous)
	}
}

// TestUpgradePreservesExistingRows proves a later migration is additive: rows
// written under version 1 survive the upgrade to the current version.
func TestUpgradePreservesExistingRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.db")
	ctx := context.Background()

	db, err := sql.Open(driverName, dsn(Options{Path: path}))
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	if _, err := applyMigrations(ctx, db, 1); err != nil {
		t.Fatalf("migrate to v1: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO connection_profiles (
			connection_id, display_name, family, major_version,
			scope_kind, query_dialects, write_protocols, schema_discovery,
			credential_ref_id, mock
		) VALUES ('pre-upgrade', 'Pre upgrade', 'influxdb', 2,
			'organization-bucket', '["flux"]', '["line-protocol"]', 1,
			'ref-1', 1)`); err != nil {
		t.Fatalf("insert pre-upgrade row: %v", err)
	}

	version, err := applyMigrations(ctx, db, SchemaVersion())
	if err != nil {
		t.Fatalf("upgrade to current: %v", err)
	}
	if version != SchemaVersion() {
		t.Fatalf("upgrade version = %d, want %d", version, SchemaVersion())
	}

	var displayName string
	if err := db.QueryRowContext(ctx,
		`SELECT display_name FROM connection_profiles WHERE connection_id = 'pre-upgrade'`).
		Scan(&displayName); err != nil {
		t.Fatalf("pre-upgrade row missing after upgrade: %v", err)
	}
	if displayName != "Pre upgrade" {
		t.Fatalf("display_name = %q, want %q", displayName, "Pre upgrade")
	}

	// The version-2 indexes must exist after the upgrade.
	var indexCount int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_audit_events_target'`).
		Scan(&indexCount); err != nil {
		t.Fatalf("query index: %v", err)
	}
	if indexCount != 1 {
		t.Fatalf("expected idx_audit_events_target after upgrade, found %d", indexCount)
	}
	_ = db.Close()
}

func TestMigrationRejectsDatabaseNewerThanTarget(t *testing.T) {
	store := openTemp(t)
	ctx := context.Background()

	if _, err := applyMigrations(ctx, store.DB(), 1); err == nil {
		t.Fatal("migrating a newer database to an older target should fail")
	}
}

func TestSchemaHasNoSecretValueColumn(t *testing.T) {
	store := openTemp(t)
	ctx := context.Background()

	rows, err := store.DB().QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table'`)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table: %v", err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate tables: %v", err)
	}

	want := map[string]bool{
		"connection_profiles": false,
		"credential_refs":     false,
		"audit_events":        false,
		"schema_migrations":   false,
	}
	for _, table := range tables {
		if _, ok := want[table]; ok {
			want[table] = true
		}
	}
	for table, found := range want {
		if !found {
			t.Errorf("expected table %q to exist", table)
		}
	}

	// No column anywhere may be named like a secret value.
	colRows, err := store.DB().QueryContext(ctx,
		`SELECT m.name, p.name FROM sqlite_master m
		 JOIN pragma_table_info(m.name) p
		 WHERE m.type = 'table'`)
	if err != nil {
		t.Fatalf("list columns: %v", err)
	}
	defer colRows.Close()
	for colRows.Next() {
		var table, column string
		if err := colRows.Scan(&table, &column); err != nil {
			t.Fatalf("scan column: %v", err)
		}
		switch column {
		case "value", "secret", "password", "passwd", "token", "apikey", "api_key", "secret_value":
			t.Errorf("table %q has secret-bearing column %q", table, column)
		}
	}
	if err := colRows.Err(); err != nil {
		t.Fatalf("iterate columns: %v", err)
	}
}
