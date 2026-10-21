# Persistent Control-Plane Metadata Store

Status: **Implemented for profile, credential-reference and audit metadata, with transactional registry writes.**
Decision: [ADR-0002](adr/0002-metadata-store-selection.md) (Accepted — Option A, SQLite, single host).
Task: P2L-001; transactional writes and binding referential integrity added by P2C-002.

This document describes the persistent metadata store that ADR-0002 selects. It
covers the schema and versioned migrations, the repository adapter wired into
the backend service, restart durability, transaction/concurrency behaviour,
backup/restore, and the key-separation boundary. It also states coverage and
unsupported editions honestly.

## Scope

Implemented and verified:

- SQLite schema with a committed, versioned migration set.
- Persistent repositories for connection profiles, credential **references** and
  audit events, wired into the backend Registry service (not only isolated
  interfaces).
- Profile and credential-reference durability across process restart.
- Transactional, single-writer concurrency with a bounded busy timeout.
- **All-or-nothing registry writes**: a multi-write operation (profile, approved
  binding, audit events) commits together or rolls back together, so a failure
  at any write leaves no orphaned profile, no orphaned binding and no missing
  audit event (P2C-002).
- **Referential integrity for bindings**: a binding may only name an existing
  profile and an existing credential reference, and deleting a profile removes
  the bindings that approved it (P2C-002).
- Backup (`VACUUM INTO`) and restore, with a schema-version check.
- A read-only `GET /api/v1/registry/status` endpoint reporting store reachability
  and schema version.

Not in scope for this task (deliberately deferred):

- Secret values are stored by the ADR-0003 secret store (P2L-002) as ciphertext
  envelopes in `secret_envelopes`; the keys live in a separate file. See
  [`SECRET_STORE.md`](SECRET_STORE.md). This store still holds credential
  **references** as metadata; it never holds a plaintext value or a key.
- Server-authorized profile API routes (P2L-003), live capability probing
  (P2L-004) and UI integration (P2L-005).
- Multi-host control-plane management (out of scope per ADR-0002).
- A PostgreSQL adapter. The repository boundary is interface-based so it can be
  added later, but PostgreSQL is not implemented and not selected.

## Dependency pin

The store uses `modernc.org/sqlite` **v1.60.1** — a pure-Go SQLite
implementation, so no CGO toolchain is required. It is pinned exactly in
`backend/go.mod` with `backend/go.sum` committed. CI verifies `go mod tidy`
produces no diff, so the pin cannot drift silently.

## Schema and migrations

Migrations live in `backend/internal/metadatastore/migrations.go`. They are:

- **Versioned and append-only.** `schema_migrations` records each applied
  version with its name and timestamp. An existing migration must never be
  edited after release; a new version is added instead.
- **Ordered.** Versions must strictly increase; a test asserts this.
- **Transactional.** Each migration and its version record commit together, so a
  failure cannot leave the schema half-changed.
- **Idempotent.** Re-running `Migrate` on an up-to-date database is a no-op.
- **Forward-only.** Migrating a database whose recorded version is newer than
  the target fails rather than guessing.

Current set:

| Version | Name | Change |
| --- | --- | --- |
| 1 | `initial-control-plane-metadata` | `connection_profiles`, `credential_refs`, `audit_events` |
| 2 | `audit-and-profile-lookup-indexes` | lookup indexes on audit target, audit time, profile family |
| 3 | `encrypted-secret-envelopes` | `secret_envelopes` (ciphertext envelopes only) |
| 4 | `profile-endpoint` | `connection_profiles.endpoint` (server-resolved probe target) |
| 5 | `credential-bindings` | `credential_bindings` (approved credential↔profile bindings) |
| 6 | `credential-binding-referential-integrity` | rebuilds `credential_bindings` with foreign keys to `connection_profiles` and `credential_refs` (`ON DELETE CASCADE`) and drops pre-existing orphan rows (P2C-002) |

A test builds a database at version 1, writes a row, upgrades to the current
version, and asserts the row survives — proving later migrations are additive.

### Tables

- `connection_profiles` — profile metadata, a `credential_ref_id`
  **reference**, and the profile `endpoint`. There is deliberately no column for
  a credential value. The endpoint is part of the profile identity, so a probe
  resolves its target server-side rather than from a caller-supplied host.
- `credential_refs` — alias, role, fingerprint, last-rotated, expiry, mock
  discriminator. No value column.
- `audit_events` — append-oriented actor/action/target/outcome/timestamp. No
  update or delete method; each event is validated to reject secret-like
  material before it is written.
- `secret_envelopes` — authenticated ciphertext envelopes plus the key ID that
  wrapped each one (ADR-0003). The envelope contains no plaintext and no key; the
  key lives in a separate file. See [`SECRET_STORE.md`](SECRET_STORE.md).
- `credential_bindings` — approved credential-to-profile bindings: the credential
  reference, connection ID, endpoint and canonical scope. A probe resolves a
  credential only when a matching binding exists (P2L-007). No secret value.
  Since version 6 the table carries foreign keys to `connection_profiles` and
  `credential_refs` with `ON DELETE CASCADE`, so a binding cannot outlive the
  profile it approves and a deleted profile cannot leave an orphaned binding
  behind (P2C-002).

A test enumerates every column of every table and fails if any is named like a
secret value (`value`, `secret`, `password`, `token`, …).

## Repository adapter and service wiring

`backend/internal/metadatastore` implements the contract interfaces:

- `ProfileRepository` implements `registrycontract.ProfileStore`.
- `CredentialRefRepository` implements `registrycontract.CredentialRefStore`.
- `AuditRepository` implements `registrycontract.AuditSink`.
- `CredentialBindingRepository` implements `registrycontract.CredentialBindingStore`.

Every repository runs against a narrow executor interface that both the
connection pool and a transaction satisfy, so the same implementation serves
both paths.

`backend/internal/registryservice` composes these into the Registry backend
service. It depends only on the interfaces, so it runs against the SQLite
repositories in production and the in-memory adapters in tests. A test exercises
the service over both, demonstrating the ADR-0002 migration path.

`backend/cmd/api` opens the store at startup, applies migrations, and builds the
service. If the store cannot be opened, the process logs the failure and serves
without a registry rather than serving against an unverified schema (fail
closed).

## Transactional registry writes (P2C-002)

`Store.WithinTransaction` runs a unit of work inside one SQLite transaction and
hands the caller a repository bundle bound to that transaction
(`registrycontract.TransactionalStore`). The Registry service uses it for every
multi-write operation:

| Operation | Writes in one unit of work |
| --- | --- |
| Create a profile | profile row + `profile.create` audit event |
| Create a profile with a credential (privileged enrollment) | profile row + binding row + `credential.bind` + `profile.create` audit events |
| Approve a binding | binding row + `credential.bind` audit event |
| Revoke a binding | binding removal + `credential.unbind` audit event |
| Delete a profile | binding removals + profile row + `profile.delete` audit event |
| Store a credential reference | reference row + `credential.reference.attach` audit event |
| Revoke a credential (P2C-007) | dependency checks (bindings, profiles) + reference-row removal + `credential.revoke` audit event |

The credential revocation deserves its note: the dependency checks - is any
binding still approved for the credential, does any profile still name it? -
run **inside** the same transaction as the reference-row removal, so a
concurrent approval cannot interleave with a revocation. The ciphertext
envelope itself is erased by the secret store only **after** the transaction
commits (see [`SECRET_STORE.md`](SECRET_STORE.md)): an interruption between
the two leaves an unreferenced, unreachable envelope, never a profile pointing
at a deleted secret. Revoking a reference whose row is already gone is an
idempotent completion that still removes any envelope a prior interrupted
attempt left behind.

Consequences, each covered by a test:

- A failure at **any** write rolls the whole operation back. No profile, no
  binding and no audit event of a failed operation survives, so a caller that
  receives an error is never left with a half-applied enrollment.
- A failed approval leaves the previous approval in place; a failed revocation
  keeps the approval rather than silently unbinding a credential.
- A deleted profile never leaves an orphaned binding: the service removes the
  bindings in the same transaction, and the schema enforces it with
  `ON DELETE CASCADE`.
- The exact binding rule is unchanged: a binding still matches on reference,
  connection ID, endpoint and canonical scope, and attaching a credential
  reference still requires the administrator-only `credential.attach` permission
  (P2L-007). Transactions change *how* the writes are applied, never *what* is
  allowed.

The in-memory test adapters have no transaction capability. The service keeps an
explicit compensating behaviour for them: a failed write is undone by a
registered inverse action, applied in reverse order. The SQLite path is the
production path and relies on the real rollback.

**Boundary:** the ciphertext envelope of a credential value is written by the
ADR-0003 secret store, which owns that write. If the reference write that
follows fails, the service deletes the envelope it just wrote, so no
unreferenced ciphertext is left behind. Moving the envelope write itself into
the same transaction belongs to the credential lifecycle work.

### Deletion policy

Deleting a profile **removes** the approvals that belong to it, atomically with
the profile removal and the `profile.delete` audit event. A binding is never
preserved for a profile that no longer exists, so no approval can be reused
against a later profile that reuses the same connection ID. The rule is
enforced twice: the service removes the dependent bindings inside the
transaction, and schema version 6 cascades the removal at the database level.

### Idempotency and uniqueness

| Operation | Uniqueness | Idempotency |
| --- | --- | --- |
| Create a profile | `connection_profiles.connection_id` is the primary key: one profile per identity | **Not** idempotent by design. A repeated create of an existing identity is refused with `duplicate-profile` (HTTP 409); it never overwrites the stored profile |
| Approve a binding | `credential_bindings` is keyed by (credential reference, connection ID): at most one approval per pair | Idempotent: re-approving keeps one row and re-records the approval. The endpoint and scope are taken from the stored profile, so an approval cannot drift |
| Revoke a binding | — | Idempotent: revoking an approval that is not present removes nothing and still records the `credential.unbind` audit event |
| Revoke a credential (P2C-007) | — | Idempotent: revoking a reference whose row is already gone completes without an error and still removes any envelope a prior interrupted attempt left behind; each completion records the store-level `credential.revoke` event |
| Delete a profile | — | Not idempotent-success: deleting an unknown identity is refused with `profile-not-found` (HTTP 404). The removal of the profile, of its bindings and of the audit event is atomic |
| Retry after a failure | — | Safe: a failed operation leaves the store exactly as it was, so the same request can be retried without cleaning up first |

### SQLite busy and constraint treatment

- `busy_timeout` (default 5s) turns a competing writer into a wait instead of
  an immediate `SQLITE_BUSY` failure, and `_txlock=immediate` takes the write
  lock when the transaction starts, so two writers cannot deadlock by upgrading
  a read lock.
- A writer that cannot take the lock within the timeout fails cleanly: the
  transaction is rolled back and no partial write survives.
- `PRAGMA foreign_keys(1)` is enabled on every connection, so the binding
  constraints of schema version 6 are enforced by the database, not only by the
  service.
- A uniqueness violation is translated into the contract's sanitized
  `duplicate-profile` error (HTTP 409); a foreign-key violation is reported as a
  sanitized error and rolls the transaction back. Neither leaks a SQL
  statement or a database path to the caller.

## Restart durability

Durability is verified two ways:

- In-process: a test writes profiles, a reference and an audit event, closes the
  handle, reopens the same file with a new handle, and reads everything back.
- Process-level: the built `api` binary was started, its status endpoint
  confirmed `schema_version: 2`, then stopped and restarted against the same
  file; the store reopened at version 2 with no re-migration error.

## Concurrency and transaction behaviour

SQLite allows many concurrent readers with WAL journaling but only **one writer
at a time**. The store is configured accordingly:

- `journal_mode=WAL` so readers do not block on the writer.
- `busy_timeout` (default 5s) so a competing writer waits and retries instead of
  failing immediately with `SQLITE_BUSY`.
- `_txlock=immediate` so a write transaction takes its lock up front and cannot
  deadlock by upgrading a read lock.
- A bounded connection pool (max 8 open connections).

Tests assert that 24 concurrent distinct writes all succeed with no lost write,
that 16 concurrent duplicate writes admit exactly one winner (the rest rejected
with `duplicate-profile`), and that mixed readers and writers complete without
error. Each concurrency test ends with `PRAGMA integrity_check` returning `ok`.
All of these also pass under `go test -race`.

**Documented limit:** throughput is bounded by the single-writer model. This is
acceptable for the single-host control plane ADR-0002 selects. Heavy concurrent
job/block churn is the trigger to revisit the store (ADR-0002 Option B,
PostgreSQL) under a new explicit decision.

## Backup and restore

- **Backup:** `Store.Backup` uses `VACUUM INTO`, which reads a transactionally
  consistent snapshot rather than copying files under a live writer. It refuses
  to overwrite an existing destination.
- **Restore:** `metadatastore.RestoreFrom` opens the backup and verifies it is a
  readable store at the expected schema version.
- Tests round-trip profiles and a credential reference through backup and
  restore and confirm the restored copy is complete and at the current version.

### Key separation

ADR-0002 and ADR-0003 require that encryption keys never live in the same backup
as encrypted values. This store holds no encryption keys at all: it has no
key-like table or column, and no key file is written beside the database. A test
asserts the backup contains no `%key%` table. Because keys are custodied
separately (ADR-0003, P2L-002), a metadata backup alone can never decrypt
anything — there is nothing encrypted in it.

## Coverage and unsupported editions

- **Coverage:** profile metadata, logical scopes, credential references and
  audit events. These are the entities the Registry needs to persist first.
  Canonical schemas, mapping profiles, runtime revisions and sync job/block
  state are catalogued in [`docs/contracts/README.md`](contracts/README.md) but
  are **not** yet persisted by this store; they are later work.
- **Unsupported editions:** the store is a control-plane database and is
  edition-independent. It has no bearing on InfluxDB 1.x/2.x/3.x capability
  support, which is a separate concern. InfluxDB 1.x remains unverified as a
  live target (see the integration handoff). No historian read or write is
  performed by anything in this task.

## Verification evidence (recorded 2026-10-21)

**Environment:** branch `phase-2/live-registry-implementation`; Go 1.27.3 from
the official release (verified against the published SHA-256); Node.js 24.21.0 /
npm 11.21.0. New dependency pinned: `modernc.org/sqlite v1.60.1`.

**Commands and actual outcomes:**

- `gofmt -l .` → clean; `go vet ./...` → pass; `go build ./...` → pass.
- `go test ./... -count=1` → all packages `ok` (80 tests including subtests).
- `go test -race ./internal/metadatastore/ ./internal/registryservice/ ./internal/httpapi/ -count=1` → all `ok`.
- `make check` → exit 0 (backend gofmt/vet/build/test; frontend `npm ci` +
  eslint + `tsc -b && vite build` + `vitest run` → 13 files / 142 tests;
  `policy-check.sh` → PASS).
- Process-level restart: built `api` binary served
  `{"status":"ok","store":"sqlite","connected":true,"schema_version":2}` and
  reopened the same database at version 2 after a restart.

**Acceptance mapping (P2L-001):**

- SQLite schema migrations → `migrations_test.go` (version reached, idempotent,
  ordered/unique, additive upgrade, newer-than-target rejected).
- Working repository adapter wired to the backend service → `registryservice`
  package plus `cmd/api` wiring; tests write through the service and read back
  through the raw repository.
- Profile restart durability → `TestProfilesSurviveRestart` plus the binary
  restart smoke test.
- Transaction/concurrency tests → `store_test.go` (distinct writes, duplicate
  contention, mixed readers/writers, integrity check).
- Versioned migrations → `migrations_test.go`.
- Backup/restore with only credential references in metadata → `store_test.go`
  (`TestBackupRestoreRoundTrip`, `TestBackupDoesNotContainEncryptionKeyMaterial`).

## Limitations

- Single-writer concurrency only; documented above.
- The store holds no secret value and performs no encryption; that is ADR-0003
  and P2L-002.
- `VACUUM INTO` backups are consistent but not incremental; large databases copy
  in full.
- Only Linux amd64 is verified.
- The `:memory:` path does not survive restart and is test-only.
- No PostgreSQL adapter exists; the interface boundary only makes one possible.
- A credential value's ciphertext envelope is written by the secret store
  outside the registry transaction; the service removes it if the reference
  write that follows fails.
