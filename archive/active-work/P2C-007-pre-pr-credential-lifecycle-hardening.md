# P2C-007 — Pre-PR credential lifecycle security hardening

> **State: HUMAN-AUTHORIZED ACTIVE WORK — COMPLETE, CHECKPOINTED 2026-10-21** (archived to `archive/active-work/`). The three pre-PR audit findings are remediated and verified: replacement validation now proves protected authorization, staging is per-attempt and the swap is a compare-and-swap, and revocation refuses while anything references the credential and erases the secret only after the metadata transition commits.

## Findings to resolve
1. **HIGH — Replacement validation does not establish credential authorization.** In `backend/internal/registryapi/registryapi.go`, the validation callback accepts an edition-identifying health probe, which may succeed without authenticating the submitted token. Require a read-only protected endpoint/operation proving that the candidate credential authenticates *and has the needed access for the bound profile's role/scope*. An inconclusive, anonymous or permission-denied response must fail closed, preserving the old secret. Never mutate an actual historian. If safe privilege validation cannot be proven with read-only calls, report the limitation and stop rather than claim success.
2. **HIGH — Nonunique replacement staging reference.** `backend/internal/registryservice/service.go` uses `staged-` + the existing reference ID and deletes it unconditionally before each attempt. Concurrent replacements can delete or interfere with another attempt's candidate, or allow racing swaps of different approved candidates. Use collision-resistant per-attempt staging references and serialized/CAS-protected replacement of an unchanged current credential; isolate cleanup and verify failure/retry/restart behavior.
3. **HIGH — Revocation check/use race and destructive detachment.** `RevokeCredential` checks approved bindings outside the subsequent metadata unit of work, then deletes the secret before transactional cleanup and silently detaches profiles still naming the reference. Enforce referential safety and profile dependency policy transactionally. Prefer refuse-and-preserve while *any* profile references or binding uses a credential, not merely while approved bindings exist. Never erase a working secret before safe, durable authorization/metadata transition; handle failures and simultaneous bind/revoke operations. Align docs/UI with actual revoke behavior.

## Acceptance criteria
- [x] Negative test: a fake/wrong token against a public `/health` endpoint must **never** replace an existing usable credential. Valid candidate must demonstrate actual protected authorization, or replacement fails closed.
- [x] Concurrent replacements/revoke/enrollment use unique stage IDs and safe atomic transition; tests induce timing races, failures, restart and retries without losing valid material.
- [x] Revocation of credential referenced by any profile/binding is refused without detaching that profile; concurrent attach/revoke cannot bypass this; standalone unreferenced revocation is idempotent or precisely documented with failure recovery.
- [x] Preserve all P2L-006/P2L-007 SSRF/DNS/TLS, auth and binding rules. Explicitly test role denials, cross-profile credential handling, secret leakage and audit behavior.
- [x] Re-run `make check`, `go test -race ./...` in a supported Linux environment, isolated InfluxDB 2 OSS/3 Core authorization and lifecycle tests, and policy/secret checks. Report actual executed versus skipped results, with reasons; do not claim Linux race pass if not run.
- [x] Update API/Secret Store docs and acceptance evidence; after success archive via `git mv`, clear CWAL queue pointers, commit/push and remote-verify. Report `WORK_PACKAGE_READY`; otherwise report `TASK_BLOCKED`.

## Boundaries
Synthetic isolated credentials and InfluxDB containers only; no production access, destructive historian writes, collector/runtime privileges, new ADR approvals or PR actions. Do not weaken guards to satisfy tests. Execute only this one task.

## What was changed

### Finding 1 — replacement validation proves protected authorization
- **New** `backend/internal/liveprobe/verify.go`: `Prober.VerifyCredential` proves a candidate is **authorized** by a protected, read-only listing. The deployment is identified first, then the protected check for that edition is used (`/api/v2/buckets` for InfluxDB 2, `/api/v3/configure/database?format=json` for InfluxDB 3 — the same catalogue entries the capability probe uses). The check requires the protected listing to **refuse an anonymous request** (401/403) and **accept the candidate** (200 with the expected body). An anonymous success, a denial or an inconclusive answer is a fail-closed verdict; the same endpoint policy, DNS resolution and pinned-peer controls as a probe apply, and only GET requests are made.
- `backend/internal/registryapi/registryapi.go`: the replacement validation now calls the verification and requires `Authorized`. The old check (a non-empty edition label, which a public `/health` satisfies) is gone.
- **Limitation recorded, not papered over:** a read-only check can prove *authenticated read access* only. Whether a candidate also holds write or admin privileges cannot be proven without mutating the historian, which the verification never does; the documentation and the API contract now say so explicitly, and the browser's rotation notice already states that a rotation needs an independently authorized synthetic write check.

### Finding 2 — per-attempt staging and a compare-and-swap swap
- `backend/internal/metadatastore/secret_envelope_repository.go`: new `ReplaceIf` — a conditional update keyed on the **full stored ciphertext** (`WHERE credential_ref_id = ? AND envelope = ?`), so any intervening change of the value is detected.
- `backend/internal/secretstore/store.go`: new `CurrentEnvelope` (an opaque handle on the stored ciphertext) and `ReplaceIfCurrent` (the swap, applied only while the stored ciphertext is still the observed handle; the failure-restore is itself conditional, so a restore cannot clobber a concurrent writer). The unconditional `Replace` was removed so no caller can bypass the compare-and-swap.
- `backend/internal/registryservice/service.go`: `ReplaceCredential` now **observes** the current ciphertext as the swap precondition, **stages** under a collision-resistant random reference (`Store` mints one; the old deterministic `staged-<reference>` id and its unconditional pre-delete are gone), and **swaps** with `ReplaceIfCurrent`. A lost race applies nothing and reports `credential-replace-conflict` (409). Cleanup removes only this attempt's staged material, on every exit path.

### Finding 3 — refuse-and-preserve revocation, secret erased last
- `backend/internal/registryservice/service.go`: `RevokeCredential` now performs the binding check **and** a new profile-reference check **inside** the same unit of work as the reference removal, so a concurrent approval cannot interleave (SQLite immediate transactions serialize writers; an approval that runs after the removal fails with `credential-reference-not-found`). It **refuses** with 409 while any binding or profile still references the credential and never detaches a dependent profile. The metadata transition commits **before** the secret material is erased, so an interruption leaves an unreferenced, unreachable envelope — never a profile pointing at a deleted secret. Revoking an already-revoked reference is an idempotent completion that still removes any material a prior interrupted attempt left behind.
- Contract: new `credential-replace-conflict` code (409) in `backend/internal/registrycontract/contract.go`; the `credential-in-use` message now names both kinds of dependency.
- UI and docs aligned: the browser's 409 wording covers "already exists **or is still in use**", the replacement notice states that the previous value is no longer stored, and `docs/REGISTRY_API.md`, `docs/SECRET_STORE.md`, `docs/METADATA_STORE.md`, `docs/LIVE_PROBING.md`, `docs/LIVE_UI.md` and `docs/ROADMAP.md` describe the actual behaviour.

## Verification evidence (recorded 2026-10-21)

**State:** COMPLETE, CHECKPOINTED. All three findings are remediated and verified; the replacement lifecycle was exercised against the **real, isolated** InfluxDB 2 OSS v2.9.1 and InfluxDB 3 Core v3.12.1 containers.

**Environment:** branch `phase-2/registry-completion`; Go 1.27.3 (linux/amd64 for the race suite, via the WSL runtime); Node.js 24.15.0 / npm 11.12.1; Windows development host with a WSL Ubuntu 22.04 runtime; the isolated containers ran under the WSL Docker daemon with the digest-pinned images the harness uses (`influxdb:2.9.1@sha256:cdd3dbd…`, `influxdb:3.12.1-core@sha256:1c8200f…`).

### Commands and actual outcomes

| Command | Outcome |
| --- | --- |
| `gofmt -l .` (backend) | clean |
| `go vet ./...`, `go vet -tags integration ./...` | pass |
| `go build ./...`, `go build -tags integration ./...` | pass |
| `go test ./... -count=1` (linux/amd64) | every package `ok` |
| `go test -race ./... -count=1` (linux/amd64) | every package `ok` |
| `npm ci && npm run lint && npm run build && npm test` (ui) | 0 vulnerabilities; lint clean; build clean; **20 files / 188 tests passed** |
| `bash scripts/policy-check.sh` | **PASS** (floating versions, unpinned actions, floating image tags, committed secret material) |

### Isolated InfluxDB 2 OSS / 3 Core authorization and lifecycle

The gated integration tests were run against the real containers started by the
unchanged harness (`testing/influxdb/scripts/up.sh`, digest-pinned images,
synthetic throwaway credentials, loopback-only test policy). Results:

| Check | InfluxDB 2 OSS v2.9.1 | InfluxDB 3 Core v3.12.1 |
| --- | --- | --- |
| capability probe (regression) | edition `influxdb-2.x-oss`, schema discovery supported, dialects unknown, write protocol unknown | edition `influxdb-3.x-core`, schema discovery supported, dialects unknown, write protocol unknown |
| protected listing, **anonymous** request | **401** | **401** |
| protected listing, **valid candidate** | **200 → authorized** (`schema-v2-buckets`) | **200 → authorized** (`schema-v3-databases`) |
| protected listing, **wrong candidate** | **401 → not authorized** ("the protected endpoint denied the candidate credential") | **fails closed** at edition identification (the deployment authenticates every request) |
| production default policy against the harness endpoint | refused (`tls-validation-failed`) | refused (`tls-validation-failed`) |
| lifecycle: enrol the real credential | 201 | 201 |
| lifecycle: replacement with an invalid candidate | **422**, the credential in place still resolves to the old value | **422**, the credential in place still resolves to the old value |
| lifecycle: replacement with the authorized candidate | **200**, the same reference resolves to the new value | **200**, the same reference resolves to the new value |
| lifecycle: revocation while a profile and a binding still reference it | **409**, nothing detached, the working credential still resolves | **409**, nothing detached, the working credential still resolves |
| lifecycle: revocation once nothing references it | **204**, the credential no longer resolves | **204**, the credential no longer resolves |

**14 of 14 checks passed, 0 failed** (7 per edition; the remaining gated tests
of the other edition were skipped explicitly in each per-edition run).

### Adversarial unit coverage (all green)

- `liveprobe`: `TestVerifyAcceptsAnAuthorizedCandidate`,
  `TestVerifyRefusesAWrongCandidate`,
  `TestVerifyFailsClosedWhenTheProtectedListingServesAnonymous`,
  `TestVerifyFailsClosedWhenTheProtectedListingIsInconclusive`,
  `TestVerifyAgainstAuthenticatedInfluxDB3`,
  `TestVerifyRejectsUnsupportedAndInvalidEndpoints`,
  `TestVerifyAppliesTheProductionEndpointPolicy`,
  `TestVerifyFailsClosedWhenTheBudgetIsExhausted`,
  `TestVerifyCarriesNoSecretValue`.
- `secretstore`: `TestCurrentEnvelopeReturnsTheStoredCiphertext`,
  `TestReplaceIfCurrentAppliesWhileUnchanged`,
  `TestReplaceIfCurrentConflictsOnAStaleObservation`,
  `TestReplaceIfCurrentConcurrentAttemptsHaveOneWinner` (8 racing attempts, 1 winner),
  `TestReplaceIfCurrentFailureAppliesNothing`,
  `TestReplaceIfCurrentRecordsNoValueInAudit`.
- `metadatastore`: `TestEnvelopeReplaceIfAppliesOnlyOnTheExactCiphertext`.
- `registryservice`: `TestReplacementFailsClosedAndPreservesTheOldCredential`,
  `TestReplacementRetrySucceedsAfterAFailure`,
  `TestReplacementAfterAnInterruptedAttemptSucceeds` (crash leftover is inert),
  `TestReplacementSurvivesRestart`,
  `TestReplacementStageReferencesAreUniqueAndCleanupIsIsolated` (6 concurrent
  attempts: 6 distinct staged references, 1 winner, 5 conflicts, each attempt
  removes only its own material),
  `TestRevocationRefusesAndPreservesWhileAnythingReferences`,
  `TestRevocationOfAnUnreferencedCredentialRemovesEverything`,
  `TestRevocationIsIdempotent`,
  `TestRevocationRecoveryAfterAnEnvelopeDeletionFailure`,
  `TestRevocationFailureInsideTheUnitOfWorkPreservesEverything` (a failure
  after the reference row was deleted rolls back and never touches the secret),
  `TestConcurrentBindAndRevokeCannotBypass`.
- `registryapi`: `TestCredentialReplacementFailsClosedForAnUnauthorizedCandidate`
  (the audit's exact negative: a wrong token against a public `/health` never
  replaces the usable credential), `TestCredentialReplacementSucceedsForAnAuthorizedCandidate`,
  `TestConcurrentReplacementsHaveOneWinner` (1 × 200, 1 × 409),
  `TestCredentialRevocationIsRefusedWhileAnythingReferencesIt`.

### Security invariants preserved (P2L-006 / P2L-007)

- The verification reuses the probe's endpoint policy, DNS resolution and
  pinned-peer controls; the production default policy still refuses the harness
  endpoint, and the peer-pinning and SSRF tests are unchanged and green.
- Role denials are unchanged: enrol, replace and revoke remain administrator
  only (`TestCredentialEnrollmentIsAdministratorOnly`,
  `TestCredentialReplacementIsAdministratorOnly`,
  `TestCredentialRevocationIsAdministratorOnly`).
- Binding rules are unchanged: a credential is still resolved only for a
  profile whose identity is approved, and the cross-profile attachment path is
  untouched.
- No secret reaches a response, an error, the audit trail or a log: the
  verification result, the swap, the references and the audit events were all
  asserted to carry references only.
- The policy check found no committed secret material.

### Environment limitations and skips (honest record)

1. **Host-side harness checks could not run in this environment.** The WSL
   Docker daemon in this host is the distribution's `docker.io` build
   (29.1.3); Docker 29's loopback port publishing is broken in this WSL
   runtime — host-originated traffic to `127.0.0.1:<published-port>` is routed
   into the special `loopback0` interface and dropped by the `FORWARD` policy
   (measured: a 6 s connect timeout with zero packets in the `FORWARD`
   counters, while the same port answered from inside the container's own
   network namespace). The gated integration tests were therefore executed from
   **inside each container's network namespace** (`nsenter -t <pid> -n`), where
   `127.0.0.1:<service-port>` is the service itself. The harness scripts and
   compose file were **not modified** for this; the test-only loopback policy
   and the read-only request shape are unchanged, and every request is still a
   plain loopback GET.
2. **The container baseline matrix (`scripts/run-tests.sh`) was not executed
   here** for the same reason. Its recorded evidence stands from P2C-006, which
   ran it against these same digest-pinned images on a host where the harness's
   published ports were reachable. This task's acceptance criteria cover the
   authorization and lifecycle checks, and those were executed and passed.
3. The `docker compose` v2 plugin (v5.6.1, SHA256-verified against the
   published checksum) was installed into the WSL user's `~/.docker/cli-plugins`
   because the distribution had none; the compose file itself is unchanged.
4. A failing `snap.docker.dockerd` unit on this host leaves orphan
   `docker-proxy` processes that squat the published ports; they were cleared
   before the run. No service configuration was changed.
5. **InfluxDB 1.x remains unverified** — no isolated 1.x instance exists. The
   query dialects and the write protocol remain **unknown** by design: a
   read-only GET cannot prove them.
6. The verification proves authenticated **read** access only; write-scoped
   privileges are not proven read-only and are not claimed (see Finding 1).

