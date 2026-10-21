# P2C-005 — Deliver secure credential enrollment, replacement and revocation workflow

> **State: HUMAN-AUTHORIZED ACTIVE WORK.** Approved six-task sequence on `phase-2/registry-completion`. Current task is selected by `handoff.md` and `workflow/active_work/README.md`; execute sequentially P2C-001 → P2C-006 and never skip dependencies.

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
