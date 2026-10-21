# P2L-005 — Live Registry UI, integration testing and security gate

**State:** HUMAN-AUTHORIZED ACTIVE WORK — **COMPLETE, CHECKPOINTED 2026-10-21** (archived to `archive/active-work/`). Live UI integration and end-to-end verification complete; package ready for human PR audit.
**Branch:** `phase-2/live-registry-implementation`.
**Architecture gate:** ADR-0002 (single-host SQLite) and ADR-0003 (backend-only envelope encryption, separate key custody and rotation) were accepted by the human on 2026-10-21. Respect their exact scopes; ADR-0004/0005 remain Proposed.

## Objective
Wire typed client to authenticated endpoints, distinguish actual health from unknown/unsupported, write-only secrets and rotation UI; verify real persistence, restart, logs/exports redaction, negative auth tests, integration tests, and make check. End with CWAL WORK_PACKAGE_READY for human PR audit, no agent PR.

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

**Specific acceptance:** working UI against real authorized APIs, scoped profile selection, write-only credential UX, health and probes, sanitized errors, state/restart/negative-auth checks, integration documentation and reproducible test output against the isolated stack. Finish final package audit.

## Verification evidence (recorded 2026-10-21)

**Environment:** branch `phase-2/live-registry-implementation`; Node.js 24.21.0 /
npm 11.21.0; Go 1.27.3; Docker Engine 29.9.1 / Compose v5.6.0 on the isolated
test host.

**Deliverables:**
- `ui/src/api/session.ts` — in-memory-only session token holder (never written to
  browser storage).
- `ui/src/api/registry.ts` — live API wire contract (`ApiProfile`,
  `ApiProbeResult`) with secret-key-rejecting guards and `toRegistryProfile`.
- `ui/src/api/client.ts` — bearer-token attachment, POST support, 204 handling,
  live profile and probe calls.
- `ui/src/hooks/useLiveRegistry.ts` — live query (gated on selection) and probe
  mutation.
- `ui/src/features/databases/DatabasesPage.tsx` — explicit mock/live source
  switch, session-token entry, LIVE labeling, live probe UX.
- `ui/src/app/modules.ts`, `ui/src/components/common/ModulePageParts.tsx` —
  "partially implemented" status for Databases.
- Tests: `DatabasesPage.live.test.tsx`, `api/registry.live.test.ts`; updated
  `moduleEntryPoints.test.tsx`.
- `testing/influxdb/scripts/toolkit-live-registry.sh` — end-to-end harness.
- `docs/LIVE_UI.md` — integration, token handling, coverage, limits.

**Commands and actual outcomes:**
- `npm run lint` → pass; `tsc -b && vite build` → pass; `vitest run` → 15 files,
  152 tests, all passing.
- `make check` → exit 0 (backend + frontend + policy).
- `./scripts/toolkit-live-registry.sh` → exit 0 (7 checks): anonymous GET → 401;
  admin POST → 201; admin GET lists profile; viewer POST → 403; admin probe →
  `edition=influxdb-2.x-oss`, `mock=false`; profile survives backend restart.

**Acceptance mapping:**
- Working UI against real authorized APIs → `DatabasesPage.live.test.tsx` and the
  e2e harness.
- Write-only credential UX / no browser secrets → token is in-memory only; a test
  asserts empty `localStorage`/`sessionStorage` and no echoed value.
- Health and probes → live listing plus the read-only probe route.
- Sanitized errors → the client renders only a status code and short message.
- State/restart/negative-auth checks → the e2e harness (401/403/restart) and unit
  tests.
- Reproducible output against the isolated stack → `toolkit-live-registry.sh`.

**Limitations:** static operator token with no login/expiry/rotation in the UI;
browser write flows intentionally disabled; only the Databases page is live; no
real-browser visual/screen-reader validation (jsdom only); Linux amd64 verified
only. No historian write, production access or Docker privilege expansion
occurred.
