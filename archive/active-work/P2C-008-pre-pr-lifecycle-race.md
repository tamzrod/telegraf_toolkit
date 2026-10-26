# P2C-008 — Pre-PR replacement-versus-revocation race hardening

> **State: HUMAN-AUTHORIZED ACTIVE WORK — COMPLETE, VERIFIED 2026-10-26** (archived to `archive/active-work/`). The HIGH finding is remediated: replacement, enrollment and revocation commit through a SQLite lifecycle fence that re-checks the active reference, approved binding and profile endpoint/scope inside the same transaction as the envelope write, and no writer transaction is held across network validation.
**Designated branch:** `phase-2/registry-completion`. **Current and only task:** P2C-008 (complete).

## Independent audit finding (HIGH)
`registryservice.ReplaceCredential` reads profile/binding/reference and an existing envelope before running network verification. It then swaps the envelope using ciphertext CAS (`ReplaceIfCurrent`) without atomically checking that the credential reference remains active and approved. Concurrent `RevokeCredential` commits reference removal first, then deletes the envelope. The replacement can race into that window, possibly reporting success on a revoked reference, re-creating or leaving unreachable credential material, or interacting with revocation cleanup. The existing CAS only protects against other envelope replacements, not metadata revocation.

## Required fix
- Establish an atomic lifecycle fence/locking scheme for replacement, revocation, binding/rebinding, profile deletion and enrollment. The replacement commit must verify the same active reference, binding and endpoint/scope identity it validated against, under concurrency protection compatible with SQLite transaction behavior. Do not hold a long-lived SQLite writer transaction across network calls; use versioned state, lifecycle lock or equivalent verified design.
- Preserve `VerifyCredential` protected read-only authentication proof, pinned peer TLS/SSRF, administrator-only attachment and write privileges, and unique per-attempt stage material.
- Ensure post-revocation cleanup cannot destroy a successfully committed new credential or leave a credential active on a deleted reference. Explicitly document revocation retry semantics.
- Add deterministic race tests: pause candidate validation; revoke credential in parallel; finish validation; assert no successful replacement of a revoked reference, no reachable/orphan usable secret, and safe retry. Also test replacement vs profile deletion, binding revocation, two replacements and concurrent enrollment. Include failure injection before/after metadata commit and envelope cleanup.
- Run `make check`, `go test -race ./...` on supported Linux, policy/secret checks, isolated synthetic InfluxDB 2/3 lifecycle verification. Record actual outcomes and skipped tests honestly.

## Lifecycle
No production connection, historian writes, collector control or PR action. Stop TASK_BLOCKED for unsatisfied security requirements. On success archive via `git mv`, update handoff and Active Work README to empty queue, commit/push and independently verify HEAD; report WORK_PACKAGE_READY.

## Resolution

The confirmed defect chain was: `ReplaceCredential` observed profile, binding,
reference and ciphertext, ran network validation with no lock held, then swapped
the ciphertext with `ReplaceIfCurrent`. The CAS serialized envelope writers only.
`RevokeCredential` commits reference-row removal first and erases the envelope
afterwards, so a replacement could commit into that window: report success on a
reference revocation had already removed, recreate ciphertext for it, and then
have revocation cleanup delete the newly committed value (or leave an orphan).

The fix is an identity fence around the commit, not a lock around validation:

- **Observe → stage → validate → fenced commit.** The candidate is staged under
  its own per-attempt reference and validated against the profile's approved
  endpoint with no writer transaction open. The commit is one short immediate
  SQLite transaction (`Store.WithinRegistryTransaction`) that re-reads the
  profile pointer, the approved binding, the reference row and the observed
  ciphertext, and conditionally swaps the envelope only while all of them still
  match. Any of revocation, binding revocation, rebinding, endpoint/scope
  change, profile deletion or enrollment that commits during validation makes
  the swap fail closed as `credential-replace-conflict` (HTTP 409). Nothing is
  written on that path.
- **Enrollment is atomic.** `EnrollCredential` seals the value without writing,
  then commits ciphertext envelope + reference row + binding + profile pointer +
  audit event in one transaction. Revocation cleanup therefore cannot erase an
  in-flight enrollment's envelope, and a failed enrollment leaves no reference,
  pointer or ciphertext.
- **Revocation keeps P2C-007 ordering and gains a cleanup guard.** Dependency
  checks and reference-row removal commit together; envelope cleanup runs after
  that commit and refuses while the reference row exists. A replacement that
  committed while the credential was still active is therefore never destroyed,
  and a replacement that lost the race never wrote, so cleanup cannot leave an
  active credential on a deleted reference.
- **Retry semantics are explicit.** A revocation whose envelope erase fails has
  already committed the metadata transition; the credential is unreachable (no
  reference row, no binding, no profile pointer) and retrying the same
  revocation is an idempotent completion of cleanup. This is documented in
  `docs/SECRET_STORE.md`, `docs/METADATA_STORE.md`, `docs/DATABASE_REGISTRY.md`
  and the service godoc.

No schema migration and no `lifecycle_epoch` column were added: the identity
re-read inside the transaction covers the required invariants, and the envelope
compare-and-swap still serializes concurrent replacements.

Preserved unchanged: `VerifyCredential`'s protected read-only authorization
proof and pinned-peer TLS/SSRF controls (P2L-006), administrator-only credential
attachment/binding (P2L-007), write-only secret handling, and unique per-attempt
stage material. `ResolveCredential` behaviour is unchanged.

## Changed surfaces

- `backend/internal/secretstore/store.go` — `SealedCredential`,
  `PrepareReplacement`, `PrepareEnrollment`: seal and self-open-verify without
  writing.
- `backend/internal/metadatastore/store.go` — `WithinRegistryTransaction`: binds
  the registry repositories and the ciphertext-envelope repository to one
  immediate transaction, plus the secret-envelope conditional replace used
  inside it.
- `backend/internal/registryservice/lifecycle.go` (new) — the fence,
  `commitReplacement`, `enrollCredentialFenced`, `eraseRevokedEnvelope`, identity
  matching and the unexported nil-by-default test hook seam.
- `backend/internal/registryservice/service.go` — replacement now observes,
  stages, validates and commits through the fence; enrollment uses the fenced
  path when a metadata store is present; revocation fires the cleanup hook and
  erases only without a live reference.
- `backend/internal/registryservice/credential_lifecycle_test.go` — the test
  secret-store decorator implements the two new sealing methods.
- `backend/internal/registryservice/race_hardening_test.go` (new) — the
  deterministic race and failure-injection suite (replaces the obsolete deleted
  file).
- `docs/SECRET_STORE.md`, `docs/METADATA_STORE.md`,
  `docs/DATABASE_REGISTRY.md` — fence and retry semantics.
- `ICC/context/project.md` — directly affected project node refreshed.

## Verification (executed 2026-10-26)

Environment: Ubuntu 26.04 LTS, Linux 7.0.0-38-generic x86_64, Go 1.27.3
linux/amd64, Node v24.16.0, npm 11.13.0, Docker server 29.5.3. Branch
`phase-2/registry-completion`, based on `93062ab25e47d80d1b620479f04411683c853dae`.

| Check | Command | Result |
| --- | --- | --- |
| Repository gate | `make check` | **PASS** — backend gofmt/vet/build/test, UI lint/build/test, policy/secret heuristics |
| Go race suite | `go test -race -count=1 ./...` (in `backend/`) | **PASS** — all packages, including `registryservice` (4.3s), `secretstore`, `metadatastore`, `registryapi`, `liveprobe`, `httpapi` |
| Integration build | `go vet -tags integration ./...` and `go build -tags integration ./...` | **PASS** |
| Policy/secret | `scripts/policy-check.sh` (via `make check`) | **PASS** — no floating pins, unpinned actions, floating images or high-signal secret patterns |
| Isolated stack | `testing/influxdb/scripts/up.sh` | **PASS** — pinned images, both containers healthy on loopback |
| InfluxDB 2/3 checks | `testing/influxdb/scripts/run-tests.sh` | **PASS** — 16 passed, 0 failed (auth rejection, writes/queries, InfluxQL, restart persistence, field-type fidelity) |
| Live lifecycle | `testing/influxdb/scripts/credential-lifecycle.sh` | **PASS** — `TestIntegrationCredentialLifecycleAgainstInfluxDB2` and `...InfluxDB3`: enroll → refused invalid replacement (422) → authorized replacement (200) → refused revocation while referenced (409) → revocation after unreference (204) |
| Teardown | `testing/influxdb/scripts/teardown.sh` | **PASS** — only the test project's containers/network removed |

No required check was skipped. The full `go test -tags integration ./...` suite
was not run as a whole: the targeted lifecycle integration tests were executed
against both real containers through the harness above.

### Deterministic lifecycle tests

`backend/internal/registryservice/race_hardening_test.go` (all passing under
`-race`, and the pre-existing stage-isolation test is preserved):

1. `TestReplacementCannotCommitAfterRevocationDuringValidation` — validation
   paused; binding removed and revocation committed (cleanup also paused);
   validation resumes; replacement conflicts, no candidate becomes active, no
   orphan material remains, cleanup completes and retry is idempotent.
2. `TestReplacementFailsClosedWhenProfileDeletedDuringValidation`
3. `TestReplacementFailsClosedWhenBindingRevokedDuringValidation`
4. `TestReplacementFailsClosedWhenProfileIdentityChangesDuringValidation`
   (endpoint moved during validation)
5. `TestReplacementFailsClosedWhenEnrollmentRepointsProfile`
6. `TestConcurrentEnrollmentsCommitConsistentState` — two concurrent enrollments,
   both materialised, profile points at exactly one, no orphan envelope.
7. `TestTwoConcurrentReplacementsCommitExactlyOne` — aligned at the observed
   ciphertext; exactly one success and one conflict.
8. `TestReplacementFailureBeforeCommitPreservesTheOldCredential`
9. `TestReplacementFailureInCommitRollsBackTheSwap`
10. `TestReplacementFailureAfterCommitLeavesTheCommittedValue`
11. `TestRevocationCannotDestroyACommittedReplacement`
12. `TestRevocationFailureBeforeCommitPreservesTheCredential`
13. `TestRevocationFailureAfterCommitLeavesRetryableCleanup`
14. `TestRevocationCleanupFailureLeavesRetryableCleanup`
15. `TestEnrollmentFailureInCommitLeavesNoMaterial`

Failure injection uses an unexported, nil-by-default lifecycle hook at the
before-commit, in-commit (after the envelope write, rolling the whole
transaction back), after-commit and cleanup boundaries. No HTTP-reachable test
backdoor exists.

## Limitations and honest notes

- The in-memory adapter path (`Service.store == nil`, used only by tests without
  a real store) re-checks the same identity immediately before the ciphertext
  CAS but is not a single transaction. Production always constructs the service
  over the SQLite store, which is the fenced path.
- Revocation is two-phase by design (P2C-007): the metadata transition commits
  before the envelope is erased. If cleanup is interrupted, the envelope can
  still be decrypted if its reference ID is already known, but no reference row,
  binding or profile pointer can reach it, and retrying the revocation erases
  it. `ResolveCredential` was intentionally left unchanged so the existing
  revocation-recovery guarantees stay valid.
- Unverified: no live production historian or credential was contacted; all
  verification is synthetic and isolated.
