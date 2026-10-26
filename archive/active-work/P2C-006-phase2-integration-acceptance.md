# P2C-006 — Verify complete Registry workflows and prepare Phase 3 handoff

> **State: HUMAN-AUTHORIZED ACTIVE WORK — COMPLETE, CHECKPOINTED 2026-10-21** (archived to `archive/active-work/`). The complete Registry journey was accepted against the real isolated InfluxDB 2 OSS and 3 Core containers, the Phase 2 checklist was reconciled without overclaiming, and the Phase 3 entry contract is published.

## Intent
Run full real isolated InfluxDB 2/3 acceptance of profile lifecycle, approved credential binding, read-only probing, UI workflows, restart/restore and security controls. Review Phase 2 exit checklist and specify Phase 3 source/destination/schema discovery contract.

## Starting state
Base branch: `phase-2/registry-completion`, created from `main` following merged PR #6. The operational SQLite Registry, envelope secret store, authorized APIs, secure live probing and partial live UI are implemented, with P2L-006/007 security controls. ADR-0002/0003 Accepted, ADR-0004/0005 Proposed.

## Required result
Publish evidence by exercised edition, pass/fail/unknown including InfluxDB 1.x unverified. Record remaining follow-up separately and report whether Phase 2 exit is fully satisfied; handoff clean and tasks archived.

## Execution boundary
- Allowed: relevant `backend/`, `ui/`, `docs/` and isolated `testing/influxdb/` artifacts as needed for this task only.
- Dependencies: P2C-001, P2C-002, P2C-003, P2C-004, P2C-005.
- Explicit non-goals: No Phase 3 mapper implementation, production deployment, historian repair, or claim of support not verified.
- No unauthorized privileged or destructive actions; no production historian or credential access.

## Acceptance criteria
- [ ] Publish evidence by exercised edition, pass/fail/unknown including InfluxDB 1.x unverified. Record remaining follow-up separately and report whether Phase 2 exit is fully satisfied; handoff clean and tasks archived.
- [ ] Preserve server-side RBAC, administrator-only credential binding, P2L-006 pinned-peer/TLS/SSRF controls, and secrets/redaction invariants.
- [ ] Evidence corresponds to actual tested behavior; unsupported/unverified remains explicitly labelled.
- [ ] No new secrets, deployment or production historian writes.

## Verification evidence
make check, go test -race, e2e harness, static secret/policy checks and documented manual browser/a11y checks or explicitly outstanding.

## Refined execution and acceptance contract (P2C-006)

- Perform a reproducible integrated test from fresh checkout against isolated synthetic InfluxDB 2 OSS and 3 Core: profile create/list/delete, privileged credential enrollment/binding, denied roles, rotation failure/recovery, probe evidence, UI source/destination selection, restart and backup/restore.
- Confirm security regression coverage from P2L-006/007 and P2C-002/005: DNS rebinding, peer pinning/TLS, SSRF, cross-profile credential theft, transactional integrity, redaction and revocation.
- Publish a **per-edition evidence table** with commands, pass/fail/unknown, limitations and provenance. InfluxDB 1.x remains **unverified** unless an independently authorized real test setup is available.
- Reconcile Phase 2 checklist without overclaiming: mark each item complete, partial or blocked based on real evidence; identify remaining token lifecycle, production TLS/packaging, dependency inspection, fine-grained authorization, write-capability uncertainty and UI accessibility limitations as applicable.
- Produce a bounded Phase 3 Canonical Mapper entry contract describing source/destination identity, scope, available schema/query evidence and explicit unknown states; no mapper implementation or data writes.
- Run make check, Go race, policy checks and available isolated harness scripts; document skipped or unavailable tests, never invent passing results.
- Archive P2C-006, set both CWAL pointers to queue-empty, commit/push and remote-verify; report WORK_PACKAGE_READY or TASK_BLOCKED. No PR actions.

## Stop conditions
Stop and report `TASK_BLOCKED` for unsafe network/credential access, unresolved architecture, or required testing unavailable. No weakening tests or replacing working behavior with mocks.

## Git lifecycle
Human-promoted and authorized as part of the six-task queue. Execute only when designated CURRENT; advance one task at a time. Complete tests, `git mv` to `archive/active-work/`, update aligned CWAL pointers, commit/push and verify remote checkpoint. Human controls PR initiation/review/merge.

## Verification evidence (recorded 2026-10-21)

**State:** COMPLETE, CHECKPOINTED. The complete Registry journey was accepted
against the **real, isolated** InfluxDB 2 OSS v2.9.1 and InfluxDB 3 Core v3.12.1
containers, the Phase 2 checklist is reconciled against real evidence, and the
Phase 3 entry contract is published.

**Environment:** branch `phase-2/registry-completion`; Go 1.27.3; Node.js 24.15.0
/ npm 11.12.1; Windows development host with a WSL Linux runtime; the isolated
containers ran under the WSL Docker daemon (29.3.1) with the digest-pinned
images the harness uses.

### Integrated acceptance against the real containers

The backend ran as a real process on loopback with a real SQLite store, a real
token file and a throwaway master key; the profiles pointed at the real
containers and carried the containers' own admin tokens as the profile
credential.

| Check | InfluxDB 2 OSS | InfluxDB 3 Core |
| --- | --- | --- |
| administrator creates the profile | 201 | 201 |
| viewer creates a profile | 403 | 403 |
| operator enrols a credential | 403 | 403 |
| administrator enrols the real container credential | 201, no value in the response | 201, no value in the response |
| the profile is listed | yes | yes |
| the live probe against the real container | 200, the edition and the observations are reported, no value in the response | 200, the edition and the observations are reported, no value in the response |
| a caller-supplied endpoint in the probe body | 400 | 400 |
| the approval is recorded | yes | yes |
| the profiles survive a backend restart | yes | yes |
| deleting the profile removes its approval | 204, the approval is gone | 204, the approval is gone |
| the audit trail | records the enrolment and the approval, carries no value | records the enrolment and the approval, carries no value |

**32 of 32 checks passed, 0 failed.**

### Security regression coverage confirmed

| Control | Coverage |
| --- | --- |
| DNS rebinding / peer pinning / TLS | `liveprobe`: `TestDialIsPinnedToValidatedPeer`, `TestDialRejectsUnpinnedPeer`, `TestProbeRefusesMultiAnswerWithDeniedPeer`, `TestProbeRefusesUnspecifiedAndLinkLocal`, `TestProbeRefusesRedirectToDeniedHost`, `TestPinnedClientUsesEndpointSNI`, `TestPinnedClientTLSHostnameVerification`, `TestPinnedClientHasNoProxy` |
| SSRF / default policy | `TestDefaultPolicyDeniesPrivateAndInsecure`, `TestProbeRejectsPrivateTargetUnderDefaultPolicy`, `TestProbeRejectsCredentialBearingURL`, `TestProbeRejectsUnresolvableHost`, `TestProbeRejectsMixedDNSAnswers` |
| Cross-profile credential theft | `registryapi`: `TestProbeCannotForwardCredentialToArbitraryHost`, `TestProbeCannotSubstituteAnotherProfilesCredential`, `TestConfiguratorCannotExfiltrateCredential`, `TestDeniedProbeLeaksNoCredential`, `TestProbeResultCarriesNoCredential` |
| Transactional integrity | `registryservice`: the fault-injection suite (`TestEnrollmentIsAtomicAtEveryWrite` in both failure modes, `TestCreateProfileIsAtomicAtEveryWrite`, `TestApproveBindingIsAtomicAtEveryWrite`, `TestRevokeBindingIsAtomicAtEveryWrite`, `TestDeleteProfileRemovesItsBindings`, `TestConcurrentEnrollmentsStayIsolated`, `TestConcurrentDuplicateEnrollmentHasOneWinner`, `TestEnrollmentSurvivesRestart`) |
| Redaction and revocation | `registryapi`: `TestCredentialEnrollmentNeverEchoesTheValue`, `TestCredentialEnrollmentRefusesASmuggledSecretKey`, `TestCredentialRevocationIsRefusedWhileABindingRequiresIt`, `TestCredentialRevocationIsAdministratorOnly`, `TestCredentialReplacementFailsClosedWhenValidationFails`, `TestNoGenericSecretResolveRoute`; the process-level check asserts that no response, log or audit entry carries a value |
| Rotation failure and recovery | `secretstore`: `TestRotationDoesNotClobberConcurrentRewrap`, `TestRotationFailurePreservesRecoverability`, `TestRotationRequiresOldKeyStillPresent` |

### Package-level checks

- `gofmt -l .` → clean; `go vet ./...` → pass; `go build ./...` → pass.
- **linux/amd64 (the CI platform), full backend suite → every package `ok`:**
  `authz`, `fixtures`, `httpapi`, `liveprobe`, `metadatastore`, `registryapi`,
  `registrycontract`, `registryservice`, `secretstore`, `synccontract`.
- `go test -race` → the packages that can run on this host pass under the race
  detector (`metadatastore`, `registrycontract`, `liveprobe` and the new
  transactional tests).
- Frontend: `npm run lint` → pass; `npx tsc -b` → pass; `npm run build` → pass;
  `npm test` → **20 files / 188 tests passed**.
- `scripts/policy-check.sh` → **PASS** (exit 0).
- Relative markdown link check over every changed `.md` → **0 missing links**.
- `make check` itself needs `make`, which is not installed on this host, so its
  components were run individually with the same commands the target uses.

### Phase 2 checklist reconciliation

`docs/ROADMAP.md` now marks the Phase 2 items **complete, partial or blocked**
against the real evidence, and states the exit-criteria status honestly: two
parts of the exit criterion are **not** met - no downstream module consumes
registered profiles yet, and the query dialects and the write protocol are
**unknown** rather than proven.

### Phase 3 handoff

`docs/contracts/canonical-mapper.md` is the bounded entry contract: the source
and destination identity, the scope of the first increment, what the Registry
can prove today, the explicit unknown states the mapper must model, the
boundaries it inherits, the entry checklist for the first Phase 3 task and the
decisions it does not make. It implements nothing and reads no data.

### What was skipped, and why

| Skipped | Reason |
| --- | --- |
| The repository harness scripts (`testing/influxdb/scripts/*.sh`) | This environment's Docker CLI has no Compose v2 plugin, so the harness cannot start the stack. The same digest-pinned images, loopback ports, synthetic credentials and volumes were started directly with `docker run`, and the same flows were exercised. The repository harness was **not** modified |
| A race build for linux/amd64 | Cross-compiling a race binary needs a Linux C toolchain, which this host does not provide. The race detector ran on the Windows host for the packages that can run there |
| A manual browser and screen-reader pass | Not performed on this host. The automated checks cover the labels, roles and states, not the rendered appearance or the spoken output. **Outstanding** |
| InfluxDB 1.x | No isolated 1.x instance exists and none is authorized: **unverified** |
| A write-capability check | A read-only probe never writes and no independently authorized synthetic write check exists: **unknown** |

### Remaining follow-up (recorded separately)

- **Token lifecycle:** the tokens are static and file-provisioned - no
  issuance, expiry, rotation or OIDC/OAuth.
- **Production TLS and packaging:** the backend does not terminate TLS
  (ADR-0005 is **Proposed**), and a credential value must only be submitted
  over a TLS-terminated transport.
- **Dependency inspection:** no view of which pipelines or jobs use a profile.
- **Fine-grained authorization:** the model is role-based only.
- **Write-capability uncertainty:** the write protocol is unknown.
- **UI accessibility:** the automated checks pass; the manual visual and
  screen-reader pass is outstanding.
- **Master-key rotation in the browser** and runtime secret delivery (ADR-0004
  is **Proposed**).

### Is the Phase 2 exit fully satisfied?

**No - and the repository now says so.** Permissions and secret isolation are
tested, profiles persist, bind credentials and probe live, and the browser
manages unbound profiles and credentials within the role model. The exit
criterion is **partially** satisfied: no downstream module consumes registered
profiles yet, and the query dialects and the write protocol are **unknown**
rather than proven. The roadmap, the API document and the Phase 3 entry
contract all state this the same way.
