# P2C-004 — Enable authorized live Registry profile management in browser

> **State: HUMAN-AUTHORIZED ACTIVE WORK.** Approved six-task sequence on `phase-2/registry-completion`. Current task is selected by `handoff.md` and `workflow/active_work/README.md`; execute sequentially P2C-001 → P2C-006 and never skip dependencies.

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

## Refined execution and acceptance contract (P2C-004)

- Enable only API-supported profile **create, list/detail, delete, and scoped source/destination selection** in Live mode. Do not imply a UI can edit/update when only create/delete exist.
- This task handles **unbound** profile creation; credential enrollment/attachment remains gated until P2C-005. Explicitly show a missing credential/binding and prohibit a fake successful probe.
- Honor backend authorization (401/403), strict validation and sanitized errors. Mock/Live sources stay separate and Live errors never silently fall back to fixtures.
- Mutations require confirmation for deletion, an explicit active connection target, predictable refresh/invalidation and safe handling of deletion of a selected profile.
- Token stays in memory only; no browser credential values, local/sessionStorage, URL leakage, logs or unsafe error echoes. Use accessible labels, focus behavior, keyboard navigation and responsive states.
- Exercise viewer/configurator/admin UX as applicable; tests include failures, duplicate IDs, stale selection, invalid scope, non-JSON/error bodies and no-mock fallback.
- Run frontend lint/types/build/Vitest, make check, and process-level authenticated API e2e where available. Manual visual/screen-reader checks must be marked done only if actually performed.

## Stop conditions
Stop and report `TASK_BLOCKED` for unsafe network/credential access, unresolved architecture, or required testing unavailable. No weakening tests or replacing working behavior with mocks.

## Git lifecycle
Human-promoted and authorized as part of the six-task queue. Execute only when designated CURRENT; advance one task at a time. Complete tests, `git mv` to `archive/active-work/`, update aligned CWAL pointers, commit/push and verify remote checkpoint. Human controls PR initiation/review/merge.
