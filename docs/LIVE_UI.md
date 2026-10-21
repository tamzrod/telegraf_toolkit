# Live Registry UI Integration

Status: **Implemented (live listing and read-only probing; writes remain gated).**
Tasks: P2L-001–P2L-005.

This document describes how the browser SPA connects to the authorized Registry
API, what is real versus mock, the credential-safety rules, and the end-to-end
verification evidence.

## Two explicit data sources

The Databases page offers a deliberate choice, never an implicit fallback:

- **Mock adapter** (default) — profiles from the typed test-only adapter, every
  entry labeled `MOCK`. No request reaches a live endpoint beyond the typed
  client's own mock path.
- **Live Registry API** — profiles from the authorized backend. Every entry is
  labeled `LIVE` unless the server itself marks it mock. If the request is
  rejected the page shows an explicit error and states it is **not** a mock
  fallback; mock data never leaks into the live view.

The live query is gated (`enabled`) on the live source being selected, so mock
mode does not issue an unauthorized request.

## Session token handling

- The bearer token is held **in memory only** (`src/api/session.ts`). It is never
  written to `localStorage`, `sessionStorage`, cookies, the URL or IndexedDB, so
  a refresh drops it. A test asserts both storages remain empty after a token is
  applied.
- The token input is `type="password"` with `autocomplete="off"`, and the value
  is cleared from component state once applied; it is never re-rendered.
- The typed client attaches `Authorization: Bearer …` only when a token is held.

## Live capability results

- A probe result is rendered from the live three-valued vocabulary. A capability
  is shown as **Supported** only when the probe confirmed it; otherwise it is
  **Unsupported**. `Unknown` is reserved for a probe that could not complete and
  is never rendered as support.
- The live API reports positively-supported capabilities as lists; a capability
  absent from the list is rendered `unsupported`, which is what the backend
  observed. The conversion lives in `toRegistryProfile`.

## What remains gated

- **Saving a profile from the UI, credential entry and deletion** stay disabled.
  The backend supports them (P2L-003), but enabling browser writes is a separate,
  reviewable step; the disabled controls state this honestly.
- **Credential values** are never entered, displayed, stored or resolved in the
  browser. The page shows credential **references** only.

## Verification evidence (recorded 2026-10-21)

**Environment:** branch `phase-2/live-registry-implementation`; Node.js 24.21.0 /
npm 11.21.0; Docker Engine 29.9.1 with Compose v5.6.0 on the isolated test host.

**Commands and actual outcomes:**
- `npm run lint` → pass; `tsc -b && vite build` → pass; `vitest run` → 15 files,
  148 tests, all passing.
- `make check` → exit 0 (backend + frontend + policy).
- `./scripts/toolkit-live-registry.sh` (isolated stack) → exit 0:
  - anonymous `GET /profiles` → **401**;
  - admin `POST /profiles` → **201** (persisted);
  - admin `GET /profiles` → profile present;
  - viewer `POST /profiles` → **403**;
  - admin `POST /probe` → `edition=influxdb-2.x-oss`, `mock=false`;
  - profile survives a backend restart → present after restart.

**Acceptance mapping (P2L-005):**
- Working UI against real authorized APIs → `DatabasesPage.live.test.tsx` plus the
  e2e harness script.
- Scoped profile selection → the existing source/destination selectors operate on
  whichever profile list is active.
- Write-only credential UX / no browser secrets → session token in memory only;
  a test asserts empty browser storage and no echoed value.
- Health and probes → live listing and read-only probe route.
- Sanitized errors → the client surfaces only a status code and a short message;
  no response body is rendered.
- State/restart/negative-auth checks → e2e harness (401/403/restart) and unit
  tests.
- Reproducible output against the isolated stack → `toolkit-live-registry.sh`.

## Limitations

- The token is a static operator token; there is no login flow, expiry or
  rotation in the UI.
- Browser writes (create/delete) are intentionally disabled.
- Only the Databases page is wired to live data; other modules remain shells.
- No real-browser visual/screen-reader validation was performed here (jsdom only);
  see the manual steps in [`DEVELOPER_SETUP.md`](DEVELOPER_SETUP.md).
- Only Linux amd64 is verified.
