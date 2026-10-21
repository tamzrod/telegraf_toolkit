# P2C-004 — Enable authorized live Registry profile management in browser

> **State: PLANNING ONLY — not authorized.** Promotion to `workflow/active_work/` requires explicit human authorization and updated CWAL handoff.

## Intent
Extend Databases live UI with validated profile creation, inspection and deletion using existing server APIs and explicit source/destination scope. Preserve Mock/Live isolation, session token in-memory-only and role based 401/403.

## Starting state
Base branch: `phase-2/registry-completion`, created from `main` following merged PR #6. The operational SQLite Registry, envelope secret store, authorized APIs, secure live probing and partial live UI are implemented, with P2L-006/007 security controls. ADR-0002/0003 Accepted, ADR-0004/0005 Proposed.

## Required result
Functional authenticated user journeys, visible validation/error/loading/permission states; no browser secret values, no implicit mock fallback; keyboard/accessibility checks.

## Execution boundary
- Allowed: relevant `backend/`, `ui/`, `docs/` and isolated `testing/influxdb/` artifacts as needed for this task only.
- Dependencies: P2C-001, P2C-002, P2C-003.
- Explicit non-goals: Credential enrollment and attachment UI remains gated until P2C-005; no new authentication provider.
- No unauthorized privileged or destructive actions; no production historian or credential access.

## Acceptance criteria
- [ ] Functional authenticated user journeys, visible validation/error/loading/permission states; no browser secret values, no implicit mock fallback; keyboard/accessibility checks.
- [ ] Preserve server-side RBAC, administrator-only credential binding, P2L-006 pinned-peer/TLS/SSRF controls, and secrets/redaction invariants.
- [ ] Evidence corresponds to actual tested behavior; unsupported/unverified remains explicitly labelled.
- [ ] No new secrets, deployment or production historian writes.

## Verification evidence
Vitest, typed API tests, real API e2e smoke and make check; no production access.

## Stop conditions
Stop and report `TASK_BLOCKED` for unsafe network/credential access, unresolved architecture, or required testing unavailable. No weakening tests or replacing working behavior with mocks.

## Git lifecycle
Execute only when human-promoted. Complete tests, `git mv` to `archive/active-work/`, update aligned CWAL pointers, commit/push and verify remote checkpoint. Human controls PR initiation/review/merge.
