package metadatastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, pinned in go.mod

	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// driverName is the database/sql driver registered by modernc.org/sqlite.
const driverName = "sqlite"

// Options configures a metadata store.
type Options struct {
	// Path is the SQLite database file. It must be an absolute or relative
	// filesystem path; ":memory:" selects a private in-memory database, which is
	// only appropriate for tests because it does not survive process restart.
	Path string
	// BusyTimeout bounds how long a writer waits for a competing writer. SQLite
	// allows only one writer at a time; a bounded wait turns contention into a
	// retry instead of an immediate SQLITE_BUSY failure.
	BusyTimeout time.Duration
}

// Store is the persistent control-plane metadata store. It owns the database
// handle and exposes narrow repositories over it.
type Store struct {
	db   *sql.DB
	path string
}

// Open opens (creating if needed) the metadata database and applies every
// pending migration. It returns an error rather than a usable store when the
// schema cannot be brought to the expected version, so a caller never serves
// requests against a partially migrated database.
func Open(ctx context.Context, opts Options) (*Store, error) {
	if strings.TrimSpace(opts.Path) == "" {
		return nil, errors.New("metadata store path is required")
	}
	if opts.BusyTimeout <= 0 {
		opts.BusyTimeout = 5 * time.Second
	}
	if err := ensureParentDir(opts.Path); err != nil {
		return nil, err
	}

	db, err := sql.Open(driverName, dsn(opts))
	if err != nil {
		return nil, fmt.Errorf("open metadata store: %w", err)
	}
	// SQLite permits many concurrent readers with WAL but only one writer.
	// A modest pool plus a busy timeout keeps concurrent writers correct rather
	// than fast.
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(0)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping metadata store: %w", err)
	}

	store := &Store{db: db, path: opts.Path}
	if _, err := Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

// dsn builds a SQLite DSN that enables WAL journaling, foreign keys and an
// immediate write lock. `_txlock=immediate` avoids the read-then-upgrade
// deadlock two concurrent writers can otherwise hit.
func dsn(opts Options) string {
	if opts.Path == ":memory:" {
		return "file::memory:?cache=shared&_pragma=foreign_keys(1)&_txlock=immediate"
	}
	params := url.Values{}
	params.Add("_pragma", "journal_mode(WAL)")
	params.Add("_pragma", "busy_timeout("+fmt.Sprintf("%d", opts.BusyTimeout.Milliseconds())+")")
	params.Add("_pragma", "foreign_keys(1)")
	params.Add("_pragma", "synchronous(NORMAL)")
	params.Add("_txlock", "immediate")
	return "file:" + opts.Path + "?" + params.Encode()
}

func ensureParentDir(path string) error {
	if path == ":memory:" {
		return nil
	}
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	if _, err := os.Stat(dir); err == nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create metadata store directory: %w", err)
	}
	return nil
}

// DB exposes the underlying handle for the repository implementations and for
// administrative checks. Callers outside this package should prefer the typed
// repositories.
func (s *Store) DB() *sql.DB { return s.db }

// Path returns the configured database path.
func (s *Store) Path() string { return s.path }

// Version reports the applied schema version.
func (s *Store) Version(ctx context.Context) (int, error) {
	return currentVersion(ctx, s.db)
}

// Ping verifies the database is reachable.
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// Profiles returns the profile repository.
func (s *Store) Profiles() *ProfileRepository { return &ProfileRepository{db: s.db} }

// CredentialRefs returns the credential-reference repository.
func (s *Store) CredentialRefs() *CredentialRefRepository {
	return &CredentialRefRepository{db: s.db}
}

// Audit returns the audit repository.
func (s *Store) Audit() *AuditRepository { return &AuditRepository{db: s.db} }

// SecretEnvelopes returns the ciphertext-envelope repository. It stores and
// returns opaque ciphertext only; the keys live in a separate key file.
func (s *Store) SecretEnvelopes() *SecretEnvelopeRepository {
	return &SecretEnvelopeRepository{db: s.db}
}

// CredentialBindings returns the approved credential-binding repository.
func (s *Store) CredentialBindings() *CredentialBindingRepository {
	return &CredentialBindingRepository{db: s.db}
}

func (s *Store) CanonicalSchemas() *CanonicalSchemaRepository {
	return &CanonicalSchemaRepository{db: s.db}
}

func (s *Store) MappingProfiles() *MappingProfileRepository {
	return &MappingProfileRepository{db: s.db}
}

func (s *Store) RevisionHistory() *RevisionHistoryRepository {
	return &RevisionHistoryRepository{db: s.db}
}

// WithinTransaction runs fn inside a single SQLite transaction.
//
// The repositories handed to fn are bound to that transaction, so every write
// fn performs through them is committed together or rolled back together. A
// multi-write registry operation - a profile, its approved binding and the
// audit events - therefore leaves no partial state behind when any write
// fails (P2C-002).
//
// The transaction is immediate: the DSN sets _txlock=immediate, so the write
// lock is taken when the transaction starts rather than upgraded mid-flight,
// which is what keeps two concurrent writers from deadlocking.
func (s *Store) WithinTransaction(ctx context.Context, fn func(registrycontract.Repositories) error) error {
	if fn == nil {
		return errors.New("transaction body is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin registry transaction: %w", err)
	}
	// Rollback is a no-op once the transaction has been committed.
	defer func() { _ = tx.Rollback() }()

	repositories := registrycontract.Repositories{
		Profiles:         &ProfileRepository{db: tx},
		References:       &CredentialRefRepository{db: tx},
		Bindings:         &CredentialBindingRepository{db: tx},
		Audit:            &AuditRepository{db: tx},
		CanonicalSchemas: &CanonicalSchemaRepository{db: tx},
		MappingProfiles:  &MappingProfileRepository{db: tx},
		RevisionHistory:  &RevisionHistoryRepository{db: tx},
	}
	if err := fn(repositories); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit registry transaction: %w", err)
	}
	return nil
}

// WithinRegistryTransaction runs fn inside one immediate SQLite transaction and
// binds both the registry repositories and the ciphertext-envelope repository
// to that transaction.
//
// Credential lifecycle commits use it so a replacement swap, an enrollment, or
// an envelope cleanup cannot interleave with a metadata revocation: the
// identity re-check and the envelope write commit together or roll back
// together (P2C-008). The caller must not perform network I/O inside fn. The
// DSN sets _txlock=immediate, so the write lock is taken when the transaction
// starts.
func (s *Store) WithinRegistryTransaction(ctx context.Context, fn func(repos registrycontract.Repositories, envelopes *SecretEnvelopeRepository) error) error {
	if fn == nil {
		return errors.New("transaction body is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin registry transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	repositories := registrycontract.Repositories{
		Profiles:         &ProfileRepository{db: tx},
		References:       &CredentialRefRepository{db: tx},
		Bindings:         &CredentialBindingRepository{db: tx},
		Audit:            &AuditRepository{db: tx},
		CanonicalSchemas: &CanonicalSchemaRepository{db: tx},
		MappingProfiles:  &MappingProfileRepository{db: tx},
		RevisionHistory:  &RevisionHistoryRepository{db: tx},
	}
	if err := fn(repositories, &SecretEnvelopeRepository{db: tx}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit registry transaction: %w", err)
	}
	return nil
}

// Backup writes a transactionally consistent copy of the database to dest using
// SQLite's VACUUM INTO, which reads a stable snapshot rather than copying files
// under a live writer. dest must not already exist.
//
// The copy contains metadata and credential *references* only. Encryption keys
// are never stored in this database, so a backup cannot contain a key
// (ADR-0002/ADR-0003 key-separation requirement).
func (s *Store) Backup(ctx context.Context, dest string) error {
	if strings.TrimSpace(dest) == "" {
		return errors.New("backup destination is required")
	}
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("backup destination %q already exists", dest)
	}
	if err := ensureParentDir(dest); err != nil {
		return err
	}
	// VACUUM INTO cannot be parameterised, so the destination is quoted.
	quoted := "'" + strings.ReplaceAll(dest, "'", "''") + "'"
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO "+quoted); err != nil {
		return fmt.Errorf("backup metadata store: %w", err)
	}
	return nil
}

// RestoreFrom opens a database at src and verifies it is a readable metadata
// store at the expected schema version. It is used by restore tests and by an
// operator restoring a backup into place; it does not overwrite a live file.
func RestoreFrom(ctx context.Context, src string) (*Store, error) {
	return Open(ctx, Options{Path: src})
}
