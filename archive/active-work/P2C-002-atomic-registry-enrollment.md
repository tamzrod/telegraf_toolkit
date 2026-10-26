# P2C-002 — Make Registry enrollment and bindings transactional

> **State: HUMAN-AUTHORIZED ACTIVE WORK — COMPLETE, CHECKPOINTED 2026-10-21** (archived to `archive/active-work/`). Registry enrollment, bindings and audit events are applied atomically; no orphaned profile or binding can survive a failure.

## Intent
Inspect backend/internal/registryservice/service.go and SQLite repositories for multi-step create/bind/audit and delete/recreate failure risks. Implement an all-or-nothing transactional service/storage operation preserving administrator-only credential.attach permission and exact credential/endpoint/scope binding.

## Starting state
Base branch: `phase-2/registry-completion`, created from `main` following merged PR #6. The operational SQLite Registry, envelope secret store, authorized APIs, secure live probing and partial live UI are implemented, with P2L-006/007 security controls. ADR-0002/0003 Accepted, ADR-0004/0005 Proposed.

## Required result
Fault injection at every write; no orphaned profile/binding, no privilege bypass, verified restart and concurrent operations; add migration only if needed.

## Execution boundary
- Allowed: relevant `backend/`, `ui/`, `docs/` and isolated `testing/influxdb/` artifacts as needed for this task only.
- Dependencies: P2C-001.
- Explicit non-goals: No production DB, no new credential endpoint, no weakening P2L-007 protections.
- No unauthorized privileged or destructive actions; no production historian or credential access.

## Acceptance criteria
- [ ] Fault injection at every write; no orphaned profile/binding, no privilege bypass, verified restart and concurrent operations; add migration only if needed.
- [ ] Preserve server-side RBAC, administrator-only credential binding, P2L-006 pinned-peer/TLS/SSRF controls, and secrets/redaction invariants.
- [ ] Evidence corresponds to actual tested behavior; unsupported/unverified remains explicitly labelled.
- [ ] No new secrets, deployment or production historian writes.

## Verification evidence
Go unit/integration and race tests pass, make check passes; record isolation level and rollback evidence.

## Refined execution and acceptance contract (P2C-002)

- Inspect the existing multi-step profile/create/bind/audit and delete/recreate flows before editing. Implement a single transaction or equivalent atomic unit-of-work for dependent SQLite mutations; never delete or overwrite an existing profile as rollback for a failed enrollment.
- Enforce administrator-only credential attachment at the HTTP boundary and server-side exact reference + connection ID + endpoint + canonical scope binding. Reject stale, missing, or revoked bindings at probe time.
- Define deletion policy: remove/revoke dependent bindings atomically with a profile deletion, or explicitly preserve them only if their non-reusability is proven. Idempotency and uniqueness must be defined.
- Inject errors after each dependent write; verify **zero** partial profile/binding/audit commits, original data unchanged on failure, and reliable restart/retry and concurrent-write behavior.
- Use production SQLite adapter in tests, not only in-memory stubs. Document transaction boundaries and SQLite busy/constraint treatment.
- No new production access or credential ingestion API; preserve P2L-006/007 security tests. Evidence: focused transaction/fault tests, race tests, make check; do not claim Docker checks unless run.

## Stop conditions
Stop and report `TASK_BLOCKED` for unsafe network/credential access, unresolved architecture, or required testing unavailable. No weakening tests or replacing working behavior with mocks.

## Git lifecycle
Human-promoted and authorized as part of the six-task queue. Execute only when designated CURRENT; advance one task at a time. Complete tests, `git mv` to `archive/active-work/`, update aligned CWAL pointers, commit/push and verify remote checkpoint. Human controls PR initiation/review/merge.

## Verification evidence (recorded 2026-10-21)

**State:** COMPLETE, CHECKPOINTED. Every multi-write registry operation is
applied as one unit of work; a failure at any write leaves no orphaned profile,
no orphaned binding and no missing audit event. The P2L-006 and P2L-007
protections are unchanged.

**Environment:** branch `phase-2/registry-completion`; Go 1.27.3; Node.js 24.15.0
/ npm 11.12.1; Windows development host with a Linux (WSL) runtime for the
linux/amd64 test run.

### Findings that were fixed

| Risk found in `registryservice` / the SQLite repositories | Fix |
| --- | --- |
| `CreateProfileAs` wrote the profile, then the audit event: a failing audit write left a profile the caller was told does not exist | one transaction (profile + audit) |
| `CreateProfileWithBindingAs` wrote the profile, then the binding, then the audits, and compensated by deleting the profile — a compensation, not a guarantee, and the audit gap remained | one transaction (profile + binding + both audits) |
| `ApproveCredentialBinding` wrote the binding, then the audit: a failing audit write left an approval with no audit trail | one transaction (binding + audit) |
| `RevokeCredentialBinding` deleted the binding, then the audit: a failing audit write silently unbound a credential | one transaction (revocation + audit), failed revocation keeps the approval |
| `DeleteProfileAs` deleted the profile only: its approved bindings were left behind as **orphans**, and a binding could still be approved for a profile that no longer exists | bindings are deleted in the same transaction, and schema version 6 adds foreign keys with `ON DELETE CASCADE` |
| `StoreCredential` wrote the ciphertext envelope, then the reference: a failing reference write left unreferenced ciphertext | the envelope is deleted when the reference write fails |
| `credential_bindings` had no referential integrity at all | migration version 6 rebuilds the table with foreign keys to `connection_profiles` and `credential_refs` (`ON DELETE CASCADE`) and drops pre-existing orphan rows |

### Implementation

- `registrycontract/transaction.go` (new): `Repositories` (the repositories that
  take part in one operation) and `TransactionalStore`
  (`WithinTransaction(ctx, fn)`), the optional persistence capability.
- `metadatastore`: the repositories now run against a narrow executor interface
  that both the pool and a transaction satisfy; `Store.WithinTransaction` opens
  an immediate transaction and hands the caller transaction-bound repositories.
  Migration version 6 adds the binding foreign keys.
- `registryservice`: every write path runs through one `unitOfWork` helper.
  With the transactional store the writes commit or roll back together; the
  in-memory test adapters keep an explicit compensating behaviour (inverse
  actions applied in reverse order) because they have no transaction.
- **Authorization is untouched**: `credential.attach` is still administrator-only
  at the HTTP layer, and a binding still has to match reference, connection ID,
  endpoint and canonical scope exactly. Transactions change *how* writes are
  applied, never *what* is allowed.

### Tests added

- `registryservice/transaction_test.go`: a fault store injects a failure at a
  chosen write **inside** the transaction, in two modes: `faultBefore` (the
  write never happens) and `faultAfter` (**the write is applied and the error
  is raised afterwards** — only a real rollback can remove it).
  `TestEnrollmentIsAtomicAtEveryWrite` (writes 1–4, both modes) also asserts
  that the pre-existing profile and its approval are unchanged after the
  failure; `TestCreateProfileIsAtomicAtEveryWrite` (1–2),
  `TestApproveBindingIsAtomicAtEveryWrite` (1–2) and
  `TestRevokeBindingIsAtomicAtEveryWrite` (1–2) assert that no profile, no
  binding and no audit event survives a failure at any write;
  `TestDeleteProfileRemovesItsBindings` proves a delete leaves no orphan and
  that a failed delete restores the profile and its approval;
  `TestConcurrentEnrollmentsStayIsolated` and
  `TestConcurrentDuplicateEnrollmentHasOneWinner` prove concurrent privileged
  enrollments do not interleave and that a race for one identity has exactly one
  winner; `TestEnrollmentSurvivesRestart` proves durability across a close and
  reopen.
- `registryapi/transaction_test.go`: `TestDeniedAttachLeavesNoResidue` proves a
  refused attachment writes nothing at all (no profile, no binding, no audit
  event) — no privilege bypass and no residue; `TestAdminAttachIsAtomicAndBound`
  proves the privileged path still records the exact binding;
  `TestDeleteProfileRemovesItsBindingOverHTTP` proves the HTTP delete also
  removes the approvals.
- `metadatastore/migrations_test.go`:
  `TestCredentialBindingReferentialIntegrity` proves a binding for a missing
  profile is refused and that deleting a profile removes its binding;
  `TestBindingMigrationPreservesValidRowsAndDropsOrphans` proves the version-6
  upgrade keeps valid bindings and removes the orphaned ones.
  `TestCredentialBindingSurvivesMigration` was updated: a binding now requires
  its profile and reference to exist.

### Refined contract compliance (P2C-002)

| Refined requirement | How it is met | Evidence |
| --- | --- | --- |
| Inspect the multi-step flows before editing; one transaction per dependent mutation; never delete or overwrite an existing profile as rollback | The write paths were mapped first (see the findings table) and each now runs in one transaction. A compensation is registered only **after** the insert succeeded, so it can only undo a row this operation created; the store rejects a duplicate identity before that | `registryservice/transaction_test.go` (duplicate race → exactly one winner), `service.go` comments |
| Administrator-only attachment at the HTTP boundary; exact reference + connection ID + endpoint + canonical scope binding; stale/missing/revoked bindings rejected at probe time | Unchanged from P2L-007: `credential.attach` is administrator-only, `BindingMatches` compares all four fields, and the probe re-verifies the binding | `registryapi/transaction_test.go` (`TestDeniedAttachLeavesNoResidue`, `TestAdminAttachIsAtomicAndBound`), P2L-007 tests unchanged and green |
| Define the deletion policy; define idempotency and uniqueness | Deletion **removes** the dependent bindings atomically (never preserved); uniqueness is the primary keys, and the idempotency rules are documented per operation | `docs/METADATA_STORE.md` "Deletion policy" and "Idempotency and uniqueness" |
| Inject errors after each dependent write; zero partial commits; original data unchanged; restart/retry and concurrent behaviour | Faults are injected after the write is applied as well as before it; the pre-existing data is asserted unchanged; restart and concurrent tests are included | `registryservice/transaction_test.go` |
| Use the production SQLite adapter in tests; document transaction boundaries and SQLite busy/constraint treatment | Every new test runs against the real SQLite store; the boundaries are the table in `docs/METADATA_STORE.md`, and the busy/constraint treatment is documented there | `docs/METADATA_STORE.md` "Transactional registry writes", "SQLite busy and constraint treatment" |
| No new production access or credential ingestion API; preserve P2L-006/007 tests; do not claim Docker checks | No route was added; the P2L-006/007 tests are unchanged and green; no Docker check is claimed (the harness could not run — see Limitations) | Linux suite result below; `git diff --stat` shows no route change |

### Commands and actual outcomes

- `gofmt -l .` → clean; `go vet ./...` → pass; `go build ./...` → pass.
- **linux/amd64 (the CI platform), full backend suite → every package `ok`:**
  `authz`, `fixtures`, `httpapi`, `liveprobe`, `metadatastore`, `registryapi`,
  `registrycontract`, `registryservice`, `secretstore`, `synccontract`. The
  binaries were cross-compiled for linux/amd64 and executed in the WSL Linux
  runtime, because this Windows host cannot satisfy the POSIX 0600 checks.
- `go test -race ./internal/registryservice/... ./internal/metadatastore/...
  ./internal/registrycontract/...` → the new transactional, concurrency and
  migration tests pass under the race detector; `metadatastore` and
  `registrycontract` pass in full.
- Windows host, full backend suite: every failure is the pre-existing platform
  limitation — 35 of them carry only `master key file permissions 0666 are too
  open; require 0600` or `authentication token file permissions 0666 are too
  open; require 0600`. No functional failure. The same code passes completely
  on linux/amd64 (above), and the CI runner (ubuntu-latest) reported the
  pre-change commits green.
- Frontend: `npm run lint` → pass; `npm run build` → pass; `npm test` → 15 files
  / 152 tests passed (unchanged by this task).
- `scripts/policy-check.sh` → PASS (exit 0).
- `make check` itself needs `make`, which is not installed on this Windows
  host; its components were run individually with the same commands the target
  uses (`gofmt -l`, `go vet`, `go build`, `go test`, `npm ci`, `npm run lint`,
  `npm run build`, `npm test`, `scripts/policy-check.sh`).

### Isolation level and rollback evidence

- **Isolation:** the change touches the local control-plane metadata store only.
  No production historian, credential, Docker control or Telegraf runtime was
  touched; no network call was made; the tests use synthetic data and a
  temporary SQLite file.
- **Rollback:** the code change is a normal branch commit (no migration is
  irreversible at the schema level — version 6 is forward-only and additive in
  effect: it keeps every valid binding and drops only the orphaned rows). The
  transaction rollback evidence is the fault-injection suite above: a failure
  at any write leaves the store exactly as it was before the operation.

### Limitations

- The ciphertext envelope of a credential value is written by the ADR-0003
  secret store, which owns that write; it is not part of the registry
  transaction. The service deletes the envelope when the reference write that
  follows fails, so no unreferenced ciphertext is left behind. Moving the
  envelope write into the same transaction belongs to the credential lifecycle
  work.
- The in-memory test adapters have no transaction capability; the service
  compensates explicitly for them. The SQLite store is the production path and
  uses a real rollback.
- The isolated InfluxDB 2/3 harness could not be run on this host (the Windows
  Docker Desktop daemon is not running and the bash environment's Docker CLI
  has no Compose v2 plugin). **No containers were started.** P2C-002 changes no
  probe or wire behavior, so no live-database evidence is required for it; the
  P2L-006/P2L-007 live evidence remains recorded in those archived tasks.
- `-race` could not be executed on linux/amd64 from this host (cross-compiling
  a race binary needs a Linux C toolchain); the race run is Windows-only and
  covers the new transactional, concurrency and migration tests.
- InfluxDB 1.x remains unverified; ADR-0004 and ADR-0005 remain **Proposed**.
