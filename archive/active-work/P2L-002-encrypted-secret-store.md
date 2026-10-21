# P2L-002 — Operational secret store, key custody and lifecycle

**State:** HUMAN-AUTHORIZED ACTIVE WORK — **COMPLETE, CHECKPOINTED 2026-10-21** (archived to `archive/active-work/`). ADR-0003 secret store implemented.
**Branch:** `phase-2/live-registry-implementation`.
**Architecture gate:** ADR-0002 (single-host SQLite) and ADR-0003 (backend-only envelope encryption, separate key custody and rotation) were accepted by the human on 2026-10-21. Respect their exact scopes; ADR-0004/0005 remain Proposed.

## Objective
After human acceptance of ADR-0003 and its key-custody/rotation plan, implement a backend-only pluggable secret store, authenticated write-only credential entry, encryption/key separation, sanitized audit, rotation and tests proving no secret leaks.

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

**Specific acceptance:** secure externally provisioned master key file, authenticated envelope encryption via established cryptographic library, versioned key IDs, server-only resolver, rotation/rewrap rollback and recovery tests, inaccessible browser/token logs, keys kept outside database and git. Do not use generated development secrets in production by default.

## Verification evidence (recorded 2026-10-21)

**Environment:** branch `phase-2/live-registry-implementation`; Go 1.27.3. No new
dependency: envelope encryption uses only the Go standard library
(`crypto/aes`, `crypto/cipher`, `crypto/rand`, `crypto/sha256`).

**Deliverables:**
- `backend/internal/secretstore/keyring.go` — key file loading with fail-closed
  validation (existence, owner-only mode, 32-byte keys, present active key) and
  `WriteKeyFile` for out-of-band provisioning.
- `backend/internal/secretstore/envelope.go` — versioned envelope format.
- `backend/internal/secretstore/store.go` — AES-256-GCM envelope store
  satisfying `SecretResolver`/`SecretWriter`, plus `Rotate` and `Delete`.
- `backend/internal/metadatastore/secret_envelope_repository.go` + migration v3
  (`secret_envelopes` table, ciphertext + key ID only).
- `backend/internal/registryservice` — `StoreCredential`, `ResolveCredential`,
  `RotateSecrets`, and status reporting of secret-store availability + active key
  ID.
- `backend/cmd/api/main.go` — `-key-file` flag; attaches the secret store only
  when a valid key loads, otherwise runs without credential storage.
- `docs/SECRET_STORE.md` — key custody, envelope format, rotation, key
  separation, coverage and limitations.

**Commands and actual outcomes:**
- `gofmt -l .` → clean; `go vet ./...` → pass; `go build ./...` → pass.
- `go test ./... -count=1` → all packages `ok`.
- `go test -race ./internal/secretstore/ ./internal/registryservice/ -count=1`
  → `ok`.
- `make check` → exit 0 (backend + frontend + policy).
- Process-level: `api -key-file <path>` served
  `{"schema_version":3,"secret_store":true,"active_key_id":"k1"}`; with no key
  file it reported `secret_store:false` and logged credential storage
  unavailable.

**Acceptance mapping:**
- Externally provisioned master key file → `TestLoadKeyringRejectsMissingFile`,
  `TestLoadKeyringRejectsWorldReadableFile`,
  `TestLoadKeyringRejectsWrongKeySize`, `TestLoadKeyringRejectsMissingActiveKey`,
  `TestWriteKeyFileRefusesToOverwrite`.
- Authenticated envelope encryption via an established library → AES-256-GCM;
  `TestTamperedEnvelopeFailsAuthentication`,
  `TestResolveWithWrongKeyFailsClosed`.
- Versioned key IDs → envelope carries `key_id`; key file has `active_key_id`;
  `TestLoadKeyringHoldsMultipleKeys`.
- Server-only resolver → `Resolve` is internal; the API exposes only a status
  flag; `TestEnvelopeContainsNoPlaintext` asserts the DB file holds no plaintext.
- Rotation/rewrap rollback and recovery → `TestRotationRewrapsAndVerifies`,
  `TestRotationRequiresOldKeyStillPresent`,
  `TestRotationFailurePreservesRecoverability`,
  `TestRotationDoesNotClobberConcurrentRewrap`,
  `TestServiceRotationEndToEnd`.
- Inaccessible browser/token logs → `TestAuditRecordsReferenceNotValue`; no
  value appears in any returned or stored reference field.
- Keys outside database and git → key file is separate; `.gitignore` excludes
  `*.key` and `backend/keys/`; the DB-plaintext test covers the store file.

**Limitations:** key files are protected by filesystem permissions only (no
OS keyring/HSM); no external secret-manager/KMS integration; no runtime
secret-delivery mechanism; Linux amd64 verified only. No historian read/write,
Docker control or live probe occurred; no production credential was used.
