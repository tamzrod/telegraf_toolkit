# Live Registry UI Integration

Status: **Implemented (live listing, read-only probing, and authorized create/delete of unbound profiles; credential writes remain gated).**
Tasks: P2L-001–P2L-005; P2C-004 (browser profile management).
Merged into `main` via PR #6 on 2026-10-21 (merge commit `0ba4827`); the browser write flows are added by P2C-004.

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
- A probe is requested by **connection profile ID only**: the browser never
  supplies an endpoint or a credential reference, so it cannot direct a stored
  secret to an origin of its choosing. The server resolves both from the stored
  profile.
- The live API reports positively-supported capabilities as lists; a capability
  absent from the list is rendered `unsupported`, which is what the backend
  observed. The conversion lives in `toRegistryProfile`.

## Browser write flows (P2C-004)

The live Registry view now manages **unbound** connection profiles through the
authorized API:

- **Create** — a validated form creates a profile through
  `POST /api/v1/registry/profiles`. The request body carries **no credential
  reference and no credential value**: a browser-created profile is unbound, and
  the server keeps enforcing the administrator-only attachment rule (P2L-007).
- **Delete** — each profile has a `Delete profile` control that requires an
  explicit `Confirm delete` step. The server removes the profile and its
  approved credential bindings in one transaction (P2C-002).
- **List and detail** — the list is refetched from the server after every
  mutation; the browser never assumes a write succeeded locally.

States the browser shows, and what they mean:

| State | Shown as |
| --- | --- |
| A missing token | "The live API rejected the session token." (401) |
| A role without the permission | "Your role is not permitted to perform this action in the Registry." (403) |
| A conflict — a duplicate identity, or a credential still in use, or a replacement that lost a race | "The Registry reported a conflict: that entry already exists or is still in use." (409) |
| An invalid body | "The Registry rejected the profile as invalid." (400) or the field-level reason from the local draft validation |
| A transport failure | "The live Registry API could not be reached." |

The backend's error body is never rendered: the typed client keeps only the
status, and the status is mapped to the wording above. A failure in live mode
never falls back to the mock adapter.

A profile without a credential binding is shown as such
("No credential reference attached — a probe of this profile resolves no
credential"), so a missing binding can never look like a successful probe.

## Credential lifecycle in the browser (P2C-005)

The live view manages credentials for a profile. The value is **write-only**:

- The value is entered in a `type="password"` field with `autocomplete="off"`,
  submitted once, and **cleared from the form before the request is awaited**,
  so the browser holds it only until it is sent. It is never written to
  `localStorage`, `sessionStorage`, a cookie, the URL or a log, and it is never
  echoed back into the page.
- The response shows the **reference** and the fingerprint, never the value.
- The section states the transport requirement: a value must only be submitted
  over a TLS-terminated transport.
- Enrolling, replacing and revoking require the administrator role; a refusal
  is shown as a refusal ("Your role is not permitted to perform this action in
  the Registry."), a credential that is still approved for or referenced by a
  profile is reported as such rather than as a silent success, and a
  replacement whose authorization could not be proven is refused with the
  credential in place unchanged (422, P2C-007).

## What remains gated

- **Rotation** (rewrapping envelopes onto a new master key) and **runtime
  secret delivery** stay disabled. A rotation needs an independently
  authorized synthetic write check, and runtime secret injection depends on the
  host-agent boundary (ADR-0004, still **Proposed**).
- **Updating** a profile is not implemented: the API supports create, read,
  list and delete, and the UI does not imply an edit flow.
- **Credential values** are never entered into any other field, displayed,
  stored or resolved in the browser. The page shows credential **references**
  only, and the session token is held in memory only.

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
