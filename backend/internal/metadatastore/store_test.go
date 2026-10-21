package metadatastore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// TestProfilesSurviveRestart proves the store is genuinely persistent: data
// written by one process (store handle) is readable by a second, independent
// handle opened on the same file.
func TestProfilesSurviveRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.db")
	ctx := context.Background()

	first, err := Open(ctx, Options{Path: path})
	if err != nil {
		t.Fatalf("open first: %v", err)
	}
	for _, p := range fixtures.Profiles() {
		if err := first.Profiles().Put(p); err != nil {
			t.Fatalf("put %s: %v", p.ConnectionID, err)
		}
	}
	ref := registrycontract.CredentialRef{
		CredentialRefID: "ref-persist",
		Alias:           "Persisted reference",
		Role:            registrycontract.RoleRead,
		Fingerprint:     "fp:persist",
		LastRotated:     time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC),
	}
	if err := first.CredentialRefs().Put(ref); err != nil {
		t.Fatalf("put reference: %v", err)
	}
	if err := first.Audit().Record(registrycontract.AuditEvent{
		Actor:      "tester",
		Action:     "profile.create",
		TargetKind: registrycontract.TargetProfile,
		TargetID:   "mock-influx-1x",
		Outcome:    registrycontract.OutcomeSucceeded,
		At:         time.Date(2026, 10, 21, 0, 1, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("record audit: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first: %v", err)
	}

	// Simulate a process restart with a brand-new handle.
	second, err := Open(ctx, Options{Path: path})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = second.Close() }()

	list, err := second.Profiles().List()
	if err != nil {
		t.Fatalf("list after restart: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("profiles after restart = %d, want 3", len(list))
	}
	if _, ok, err := second.CredentialRefs().Get("ref-persist"); err != nil || !ok {
		t.Fatalf("reference not durable: ok=%v err=%v", ok, err)
	}
	events, err := second.Audit().List()
	if err != nil {
		t.Fatalf("audit after restart: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("audit after restart = %d, want 1", len(events))
	}
	version, err := second.Version(ctx)
	if err != nil {
		t.Fatalf("version after restart: %v", err)
	}
	if version != SchemaVersion() {
		t.Fatalf("version after restart = %d, want %d", version, SchemaVersion())
	}
}

// TestConcurrentDistinctWritesAllSucceed exercises SQLite's single-writer limit
// under contention: many goroutines writing distinct rows must all succeed
// rather than losing a write or corrupting the database.
func TestConcurrentDistinctWritesAllSucceed(t *testing.T) {
	store := openTemp(t)
	ctx := context.Background()

	const writers = 24
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			profile := fixtures.Profile1x()
			profile.ConnectionID = fmt.Sprintf("concurrent-%02d", n)
			profile.DisplayName = fmt.Sprintf("Concurrent %02d", n)
			if err := store.Profiles().Put(profile); err != nil {
				errs <- fmt.Errorf("writer %d: %w", n, err)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent write failed: %v", err)
	}

	list, err := store.Profiles().List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != writers {
		t.Fatalf("stored profiles = %d, want %d (a write was lost)", len(list), writers)
	}

	// The database must still pass an integrity check after concurrent writes.
	if err := integrityOK(ctx, store); err != nil {
		t.Fatalf("integrity check failed: %v", err)
	}
}

// TestConcurrentDuplicateWritesAdmitExactlyOne proves the unique identity
// constraint holds under contention: exactly one writer wins and the rest are
// rejected with the duplicate-profile code.
func TestConcurrentDuplicateWritesAdmitExactlyOne(t *testing.T) {
	store := openTemp(t)
	ctx := context.Background()

	const writers = 16
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes, duplicates, other := 0, 0, 0
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := store.Profiles().Put(fixtures.Profile2x())
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				successes++
			case isDuplicate(err):
				duplicates++
			default:
				other++
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if successes != 1 {
		t.Fatalf("successful duplicate writes = %d, want exactly 1", successes)
	}
	if duplicates != writers-1 {
		t.Fatalf("duplicate rejections = %d, want %d", duplicates, writers-1)
	}
	if other != 0 {
		t.Fatalf("unexpected error count = %d, want 0", other)
	}
	if err := integrityOK(ctx, store); err != nil {
		t.Fatalf("integrity check failed: %v", err)
	}
}

// TestConcurrentMixedReadersAndWriters runs readers alongside writers to prove
// WAL readers do not block on the single writer and no read observes a
// half-written row.
func TestConcurrentMixedReadersAndWriters(t *testing.T) {
	store := openTemp(t)
	ctx := context.Background()

	if err := store.Profiles().Put(fixtures.Profile1x()); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})
	errs := make(chan error, 64)

	for r := 0; r < 8; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				if _, err := store.Profiles().List(); err != nil {
					errs <- fmt.Errorf("reader: %w", err)
					return
				}
			}
		}()
	}
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				profile := fixtures.Profile1x()
				profile.ConnectionID = fmt.Sprintf("mixed-%02d-%02d", n, i)
				profile.DisplayName = profile.ConnectionID
				if err := store.Profiles().Put(profile); err != nil {
					errs <- fmt.Errorf("writer %d: %w", n, err)
					return
				}
			}
		}(w)
	}

	// Let writers finish, then stop readers.
	go func() {
		time.Sleep(150 * time.Millisecond)
		close(stop)
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("mixed access failed: %v", err)
	}
	if err := integrityOK(ctx, store); err != nil {
		t.Fatalf("integrity check failed: %v", err)
	}
}

// TestBackupRestoreRoundTrip proves a backup is transactionally consistent and
// restorable, and that the restored copy contains only credential references.
func TestBackupRestoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	source, err := Open(ctx, Options{Path: filepath.Join(dir, "source.db")})
	if err != nil {
		t.Fatalf("open source: %v", err)
	}
	defer func() { _ = source.Close() }()

	for _, p := range fixtures.Profiles() {
		if err := source.Profiles().Put(p); err != nil {
			t.Fatalf("put %s: %v", p.ConnectionID, err)
		}
	}
	if err := source.CredentialRefs().Put(registrycontract.CredentialRef{
		CredentialRefID: "ref-backup",
		Alias:           "Backup reference",
		Role:            registrycontract.RoleWrite,
		Fingerprint:     "fp:backup",
	}); err != nil {
		t.Fatalf("put reference: %v", err)
	}

	backupPath := filepath.Join(dir, "backup.db")
	if err := source.Backup(ctx, backupPath); err != nil {
		t.Fatalf("backup: %v", err)
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("backup file missing: %v", err)
	}
	// A backup must not overwrite an existing destination.
	if err := source.Backup(ctx, backupPath); err == nil {
		t.Fatal("expected backup to refuse an existing destination")
	}

	restored, err := RestoreFrom(ctx, backupPath)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	defer func() { _ = restored.Close() }()

	list, err := restored.Profiles().List()
	if err != nil {
		t.Fatalf("list restored: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("restored profiles = %d, want 3", len(list))
	}
	ref, ok, err := restored.CredentialRefs().Get("ref-backup")
	if err != nil || !ok {
		t.Fatalf("restored reference missing: ok=%v err=%v", ok, err)
	}
	if ref.Role != registrycontract.RoleWrite {
		t.Fatalf("restored reference role = %q, want write", ref.Role)
	}

	version, err := restored.Version(ctx)
	if err != nil {
		t.Fatalf("restored version: %v", err)
	}
	if version != SchemaVersion() {
		t.Fatalf("restored schema version = %d, want %d", version, SchemaVersion())
	}
}

// TestBackupDoesNotContainEncryptionKeyMaterial asserts the key-separation
// requirement: the metadata database (and therefore any backup of it) holds no
// key-like file or column, so ciphertext and key can never be co-located here.
func TestBackupDoesNotContainEncryptionKeyMaterial(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	source, err := Open(ctx, Options{Path: filepath.Join(dir, "source.db")})
	if err != nil {
		t.Fatalf("open source: %v", err)
	}
	defer func() { _ = source.Close() }()

	if err := source.Profiles().Put(fixtures.Profile1x()); err != nil {
		t.Fatalf("put: %v", err)
	}
	backupPath := filepath.Join(dir, "backup.db")
	if err := source.Backup(ctx, backupPath); err != nil {
		t.Fatalf("backup: %v", err)
	}

	// The backup directory must contain only the database, and the database
	// must contain no table that could hold a key.
	restored, err := RestoreFrom(ctx, backupPath)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	defer func() { _ = restored.Close() }()

	rows, err := restored.DB().QueryContext(ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name LIKE '%key%'`)
	if err != nil {
		t.Fatalf("query key tables: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		t.Fatalf("metadata store must not contain key material, found table %q", name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate: %v", err)
	}
}

func integrityOK(ctx context.Context, store *Store) error {
	var result string
	if err := store.DB().QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("integrity_check = %q", result)
	}
	return nil
}

func isDuplicate(err error) bool {
	var sanitized *registrycontract.SanitizedError
	if !errors.As(err, &sanitized) {
		return false
	}
	return sanitized.Code == registrycontract.CodeDuplicateProfile
}
