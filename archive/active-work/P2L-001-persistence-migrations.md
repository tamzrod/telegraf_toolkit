# P2L-001 — Persistent Registry storage and migrations

**State:** HUMAN-AUTHORIZED ACTIVE WORK — **COMPLETE, CHECKPOINTED 2026-10-21** (archived to `archive/active-work/`). ADR-0002 persistence implemented; ADR-0003 secret handling not implemented here.
**Branch:** `phase-2/live-registry-implementation`.
**Architecture gate:** ADR-0002 (single-host SQLite) and ADR-0003 (backend-only envelope encryption, separate key custody and rotation) were accepted by the human on 2026-10-21. Respect their exact scopes; ADR-0004/0005 remain Proposed.

## Objective
After human acceptance of ADR-0002, implement selected metadata store with versioned migrations for connection profiles, scopes, credential references and audit; tested restart/restore, concurrent access and no plaintext credentials.

## Acceptance and safety
- Implement actual working behavior, not in-memory or mock substitutes; clearly document coverage and unsupported editions.
- Never connect to production historians or manipulate real credentials without a separately authorized safe test environment and explicit permission. No historian writes, runtime deployments or Docker privilege expansion.
- Require server-side authorization, TLS and DNS/network SSRF protections before live probing. Verify via automated negative and isolated integration tests.
- Preserve exact dependency pins, `make check`, documented evidence and CWAL checkpoints. Human controls PR initiation, review and merge.

## CWAL execution instructions

- Execute strictly in queue order P2L-001 → P2L-005; complete and verify the current task, `git mv` it to `archive/active-work/`, update `handoff.md` and `workflow/active_work/README.md`, commit, push and confirm remote checkpoint before advancing without intermediate human approval.
- Do not weaken backend auth or SSRF protections to allow tests. **Only** isolated `testing/influxdb/` instances are authorized; local loopback/private-address exceptions must be confined to explicit test configuration, never enabled by default on production Registry endpoints.
- Preserve existing typed contracts and mock labelling until functionality is truly connected; no mock-only substitution for an acceptance claim.
- Run `make check`, relevant integration checks and policy checks for each task; capture tested features, blocked states, and limitations without committing tokens/database volumes/logs. Stop as `TASK_BLOCKED` for unavailable safety-critical dependencies, not counterfeit success.
- No OpenHands PR create/review/merge actions; final package `WORK_PACKAGE_READY` only when all five tasks have been archived with confirmed checkpoints.

**Specific acceptance:** SQLite schema migrations, working repository adapter wired to backend service (not only isolated interfaces), profile restart durability, transaction/concurrency tests, versioned migrations and backup/restore with only credential references in metadata; no secret values.

## Verification evidence (recorded 2026-10-21)

**Environment:** branch `phase-2/live-registry-implementation`; Go 1.27.3 from the
official release (SHA-256 verified against the published manifest); Node.js
24.21.0 / npm 11.21.0. New dependency pinned exactly:
`modernc.org/sqlite v1.60.1` (pure Go, no CGO) with `backend/go.sum` committed.

**Deliverables:**
- `backend/internal/metadatastore/` — versioned migration set (`migrations.go`),
  store open/backup/restore (`store.go`), and repositories implementing
  `registrycontract.ProfileStore` / `CredentialRefStore` / `AuditSink`
  (`profile_repository.go`, `credential_ref_repository.go`,
  `audit_repository.go`). No secret value is stored or returned.
- `backend/internal/registryservice/` — the Registry backend service composing
  the repositories, so persistence is wired to a service rather than left as
  isolated interfaces.
- `backend/cmd/api/main.go` — opens the store, applies migrations and builds the
  service at startup; fails closed if the store cannot be opened.
- `backend/internal/httpapi/handler.go` — adds read-only
  `GET /api/v1/registry/status` (store reachability + schema version).
- Tests: `migrations_test.go`, `repository_test.go`, `store_test.go`,
  `registryservice/service_test.go`, `httpapi/registry_status_test.go`.
- `docs/METADATA_STORE.md` — schema, migrations, concurrency limits,
  backup/restore, key separation, coverage and limitations.

**Commands and actual outcomes:**
- `gofmt -l .` → clean; `go vet ./...` → pass; `go build ./...` → pass.
- `go test ./... -count=1` → all packages `ok` (80 tests).
- `go test -race ./internal/metadatastore/ ./internal/registryservice/ ./internal/httpapi/ -count=1` → all `ok`.
- `make check` → exit 0 (backend gofmt/vet/build/test; frontend `npm ci` +
  eslint + `tsc -b && vite build` + `vitest run` → 13 files / 142 tests;
  `policy-check.sh` → PASS).
- `go mod tidy` → no diff (CI pin check satisfied).
- Process-level restart: built `api` binary served
  `{"status":"ok","store":"sqlite","connected":true,"schema_version":2}` and,
  after a restart against the same file, reopened at version 2 with no error.

**Acceptance mapping:**
- SQLite schema migrations → version reached, idempotent re-run, strictly
  ordered/unique versions, additive upgrade preserving a version-1 row,
  newer-than-target rejected.
- Repository adapter wired to the backend service → service tests write through
  the service and read back through the raw repository; `cmd/api` wires it.
- Profile restart durability → `TestProfilesSurviveRestart` and the binary
  restart smoke test.
- Transaction/concurrency tests → 24 distinct concurrent writes all persist, 16
  concurrent duplicates admit exactly one winner, mixed readers/writers complete;
  each ends with `PRAGMA integrity_check = ok`; all pass under `-race`.
- Versioned migrations → `schema_migrations` records name/timestamp; set is
  append-only and asserted ordered.
- Backup/restore with references only → `TestBackupRestoreRoundTrip` and
  `TestBackupDoesNotContainEncryptionKeyMaterial`; a column scan rejects any
  secret-named column.

**Limitations:** single-writer concurrency (documented); no secret value,
encryption or key custody (ADR-0003, P2L-002); `VACUUM INTO` backups are full,
not incremental; Linux amd64 verified only; no PostgreSQL adapter. No historian
read/write, Docker control or live probe was performed; no production
credentials were touched.
