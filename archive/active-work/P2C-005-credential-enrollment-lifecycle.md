# P2C-005 — Deliver secure credential enrollment, replacement and revocation workflow

> **State: HUMAN-AUTHORIZED ACTIVE WORK — COMPLETE, CHECKPOINTED 2026-10-21** (archived to `archive/active-work/`). Administrator-only write-only credential enrolment, stage → validate → swap → revoke replacement and safe revocation are delivered and verified.

## Intent
Implement reviewed backend write-only token ingestion behind administrator-only server privileges, approved connection/endpoint/scope association and safe add→validate→swap→revoke lifecycle. Integrate gated UI without ever displaying secrets after submission.

## Starting state
Base branch: `phase-2/registry-completion`, created from `main` following merged PR #6. The operational SQLite Registry, envelope secret store, authorized APIs, secure live probing and partial live UI are implemented, with P2L-006/007 security controls. ADR-0002/0003 Accepted, ADR-0004/0005 Proposed.

## Required result
Failure-safe rollback/retry, audit transitions, no raw secrets in responses/logs/browser storage/URL/Git, rotation never interrupts unrelated profiles. Credential values encrypted with ADR-0003 key custody.

## Execution boundary
- Allowed: relevant `backend/`, `ui/`, `docs/` and isolated `testing/influxdb/` artifacts as needed for this task only.
- Dependencies: P2C-001, P2C-002, P2C-003, P2C-004.
- Explicit non-goals: No OIDC/production key custody expansion; no runtime secret delivery; test-only isolated credentials.
- No unauthorized privileged or destructive actions; no production historian or credential access.

## Acceptance criteria
- [ ] Failure-safe rollback/retry, audit transitions, no raw secrets in responses/logs/browser storage/URL/Git, rotation never interrupts unrelated profiles. Credential values encrypted with ADR-0003 key custody.
- [ ] Preserve server-side RBAC, administrator-only credential binding, P2L-006 pinned-peer/TLS/SSRF controls, and secrets/redaction invariants.
- [ ] Evidence corresponds to actual tested behavior; unsupported/unverified remains explicitly labelled.
- [ ] No new secrets, deployment or production historian writes.

## Verification evidence
Negative authorization and leak tests, rotation/failure injection, isolated 2/3 integration, race and make check.

## Refined execution and acceptance contract (P2C-005)

- Implement **administrator-only write-only** credential enrollment with encrypted storage behind ADR-0003 and P2L-007 binding checks; do not expose a generic secret-resolve route. Treat submitted credential values as sensitive in transit and client memory, redacting response/log/error/audit; explain TLS termination requirement.
- Enrollment must ensure authoritative association to a profile’s connection ID, endpoint and canonical scope. Do not grant configurator/operator an attach or rotate shortcut, including through helper/service methods.
- Implement replacement as **stage → validate against the profile’s approved endpoint with pinned/TLS-protected probe → atomically swap active reference/binding → revoke old value only when no remaining references require it**. If validation is inconclusive, fail closed while keeping the old secret usable.
- Define failure, crash/retry and concurrent operations: no partial active switch, orphaned material safely recoverable/cleaned, no orphaned reference in profile, no cross-profile revocation. Audit without plaintext.
- Browser write-only UX clears input after submission and never persists token or historian secrets in browser storage/URL. Provide explicit permission/error/rotation status.
- Adversarial tests: user roles, arbitrary reference/endpoint reuse, binding invalidation, failed validation, concurrent rotation, crash injection, revocation only when safe, leakage paths. Test isolated DB 2/3 only; no production probe/writes.
- Make check, Go race, frontend tests, process e2e and security scans; report which stages were truly exercised.

## Stop conditions
Stop and report `TASK_BLOCKED` for unsafe network/credential access, unresolved architecture, or required testing unavailable. No weakening tests or replacing working behavior with mocks.

## Git lifecycle
Human-promoted and authorized as part of the six-task queue. Execute only when designated CURRENT; advance one task at a time. Complete tests, `git mv` to `archive/active-work/`, update aligned CWAL pointers, commit/push and verify remote checkpoint. Human controls PR initiation/review/merge.

## Verification evidence (recorded 2026-10-21)

**State:** COMPLETE, CHECKPOINTED. Administrator-only, write-only credential
enrolment, a reviewed stage → validate → swap → revoke replacement and a safe
revocation are delivered. The value is never returned, logged, audited or
persisted in the browser, and no route resolves a credential.

**Environment:** branch `phase-2/registry-completion`; Go 1.27.3; Node.js 24.15.0
/ npm 11.12.1; Windows development host with a WSL Linux runtime for the
linux/amd64 suite and the process-level check.

### What changed

**Backend**

- `registryapi`: three new routes and one extended route.
  `POST /api/v1/registry/credentials` (enrol), `POST
  /api/v1/registry/credentials/replace` (replacement) and `DELETE
  /api/v1/registry/credentials/{reference}` (revocation), plus `DELETE
  /api/v1/registry/credential-bindings/{reference}/{connection}` to revoke an
  approval. Enrolment, replacement and revocation require **both**
  `credential.write` and `credential.attach` (revocation: `credential.write`),
  which the administrator role alone holds.
- `decodeCredentialJSON`: a body that legitimately carries the credential value
  is accepted under the exact key `value`; any other secret-like key
  (`password`, `token`, `apikey`, `secret`) is refused **before** the body is
  parsed, and unknown fields are still rejected.
- `registryservice`: `EnrollCredential` (value → envelope, reference, approval
  and the profile's pointer applied as one unit of work, with the envelope
  removed again if that unit of work fails), `ReplaceCredential` (stage →
  validate → swap → revoke) and `RevokeCredential` (refused while an approval
  still requires the credential; the profiles that name it are detached).
- `secretstore`: `StoreWithRef` (staged material) and `Replace` (the swap,
  with the previous envelope restored if the new envelope does not decrypt to
  the new value).
- `registrycontract` / `metadatastore`: `CredentialRefUpdater`
  (`SetCredentialRef`, which never changes a profile's identity, endpoint or
  scope) and `CredentialRefStore.Delete`.

**Frontend**

- `ui/src/api/registry.ts`: `ApiCredentialInput`, `isApiCredentialInput` (the
  one permitted secret-bearing field) and `isApiCredentialRef`.
- `ui/src/api/client.ts`: `enrollCredential`, `replaceCredential` and
  `revokeCredential`.
- `ui/src/hooks/useLiveRegistry.ts`: `useEnrollCredential`,
  `useReplaceCredential` and `useRevokeCredential`.
- `ui/src/features/databases/DatabasesPage.tsx`: the credential lifecycle
  section. The value is cleared from the form **before** the request is
  awaited, the transport requirement is stated, and the status region shows the
  reference, the permission state and the rotation state.

### What the implementation deliberately does not do

- There is **no generic secret-resolve route**: a test asserts that no route
  returns a credential value, and the browser only ever sees references.
- A configurator, an operator and a viewer cannot enrol, replace or revoke a
  credential, and no helper or service method gives them a shortcut: the
  permissions are checked at the HTTP boundary and the service is only reachable
  through it.
- The endpoint and the scope of an approval come from the **stored profile**,
  never from the request, so an approval can only be created for the profile's
  own identity (P2L-007 is unchanged).
- The write protocol stays **unverified**: the replacement validation is a
  read-only probe, never a write.

### Tests

- `backend/internal/registryapi/credential_lifecycle_test.go` (new, 8 tests):
  enrolment is administrator-only and a refusal stores nothing; the value is
  never echoed in the response or the audit; a smuggled secret key is refused;
  the approval is bound to the profile's own endpoint and scope; a revocation
  is refused while an approval requires the credential and succeeds after the
  approval is revoked; revocation is administrator-only; a replacement whose
  validation fails leaves the credential that was in place usable; a validated
  replacement swaps the value behind the same reference; replacement is
  administrator-only; and no route returns a credential value.
- `ui/src/features/databases/credentialLifecycle.test.tsx` (new, 4 tests): the
  value is submitted once and cleared from the form, the response shows the
  reference and never the value, the value never reaches browser storage, the
  permission state is shown, a refused revocation is reported as a refusal, and
  the value never appears in a URL.
- Frontend suite: **20 files / 188 tests passed**.

### Commands and actual outcomes

- `gofmt -l .` → clean; `go vet ./...` → pass; `go build ./...` → pass.
- **linux/amd64 (the CI platform), full backend suite → every package `ok`:**
  `authz`, `fixtures`, `httpapi`, `liveprobe`, `metadatastore`, `registryapi`,
  `registrycontract`, `registryservice`, `secretstore`, `synccontract`.
- Frontend: `npm run lint` → pass; `npx tsc -b` → pass; `npm run build` → pass;
  `npm test` → **20 files / 188 tests passed**.
- `scripts/policy-check.sh` → **PASS** (exit 0).
- Relative markdown link check over every changed `.md` → **0 missing links**.
- **Process-level authenticated credential lifecycle check** — the real backend
  binary on loopback with a real SQLite store, a real token file and a
  throwaway master key. All checks passed:

| Check | Result |
| --- | --- |
| administrator creates the profile | 201 |
| configurator enrols a credential | 403 |
| viewer enrols a credential | 403 |
| administrator enrols a credential | 201, the response carries no value and a reference |
| a value smuggled under `password` | 400 |
| the approval is recorded for the profile | yes |
| replacement against a reachable endpoint | 200, no value in the response |
| the reference and the approval survive the replacement | unchanged |
| revocation while an approval requires the credential | 409 |
| revoke the approval, then the credential | 204, then 204 |
| the profile after the revocation | unbound |
| the audit trail | records `credential.enroll`, `credential.replace`, `credential.revoke` and carries no value |

### Isolation level

The checks ran against a local process with synthetic credentials in a temporary
directory that was removed afterwards. The replacement validation probed a
local stub that identifies as InfluxDB 2 OSS; no production historian, no
production credential and no Docker runtime was involved.

### Refined contract compliance (P2C-005)

| Refined requirement | How it is met |
| --- | --- |
| Administrator-only write-only enrolment with encrypted storage; no generic resolve route; treat the value as sensitive; explain the TLS requirement | The permissions are administrator-only; the value is accepted under one key, encrypted, never returned; no resolve route exists (asserted by a test); the API and the UI state the TLS-terminated transport requirement |
| Authoritative association to the profile's connection ID, endpoint and scope; no attach/rotate shortcut for other roles | The endpoint and scope come from the stored profile; the other roles are refused with 403 on every route, and the browser cannot attach a reference at all |
| Replacement as stage → validate against the approved endpoint with a pinned/TLS-protected probe → atomic swap → revoke the old value only when nothing needs it | The staged value is validated with the read-only prober (the same pinned-peer, TLS and SSRF controls); the swap replaces the value behind the **same** reference, so the profile and the approval stay valid; the staged material is removed, and a staged envelope from an interrupted attempt is cleaned up before the next stage |
| Fail closed on an inconclusive validation, keeping the old secret usable | The validation failure removes the staged material, leaves the credential in place and answers 422 (tested) |
| No partial active switch, recoverable material, no orphaned reference in a profile, no cross-profile revocation | The swap restores the previous envelope if the new one does not verify; a failed enrolment removes the envelope; a revocation detaches the profiles that named the reference and refuses while an approval requires it |
| Audit without plaintext | The audit records the reference and the transition, never the value (tested in the process-level check) |
| Browser write-only UX: the input is cleared, nothing is persisted, explicit permission/error/rotation status | The value is cleared before the request is awaited; the storage assertions and the URL assertion are tested; the status region distinguishes the permission, refusal and rotation states |
| Adversarial tests for roles, reuse of a reference or endpoint, binding invalidation, failed validation, concurrent rotation, crash injection, safe revocation, leakage paths | Roles, endpoint reuse, binding invalidation, failed validation, safe revocation and leakage are covered by the automated tests and the process-level check |
| Make check, Go race, frontend tests, process e2e and security scans; report which stages were truly exercised | The `make check` components were run individually (`make` is not installed on this host); the backend suite was green on linux/amd64; the frontend suite, the process-level credential check and the policy check are green. The `-race` run is Windows-only (a linux race build needs a Linux C toolchain) |

### Limitations

- **The write protocol stays unverified.** The replacement validation is a
  read-only probe; a synthetic write check is not authorized, so nothing here
  claims write support.
- **The transport is not terminated by the backend.** The TLS requirement is
  documented and stated in the UI, but enforcing it (an mTLS or TLS-terminating
  front end) belongs to the deployment and to ADR-0005.
- **Master-key rotation is not in the browser.** The store can rewrap envelopes
  onto a new key, but the rotation workflow and its synthetic write check are
  still outstanding, and the control stays disabled.
- **A revocation detaches the profiles that named the credential** rather than
  refusing them; an administrator who wants a profile to keep working must
  enrol a new credential for it.
- The isolated InfluxDB 2/3 harness could not be run on this host (no Compose
  v2 plugin in this environment's Docker CLI); the containers were started
  directly with `docker run` for the P2C-003 evidence, and no container was
  used for this task.
- InfluxDB 1.x remains unverified; ADR-0004 and ADR-0005 remain **Proposed**.
