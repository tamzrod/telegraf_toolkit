# P2C-005 — Deliver secure credential enrollment, replacement and revocation workflow

> **State: PLANNING ONLY — not authorized.** Promotion to `workflow/active_work/` requires explicit human authorization and updated CWAL handoff.

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

## Stop conditions
Stop and report `TASK_BLOCKED` for unsafe network/credential access, unresolved architecture, or required testing unavailable. No weakening tests or replacing working behavior with mocks.

## Git lifecycle
Execute only when human-promoted. Complete tests, `git mv` to `archive/active-work/`, update aligned CWAL pointers, commit/push and verify remote checkpoint. Human controls PR initiation/review/merge.
