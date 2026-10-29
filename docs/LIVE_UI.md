# Live Registry UI Integration

Status: **Implemented on `phase-3/canonical-mapper` (live listing, read-only probing, authorized create/delete of unbound profiles, the administrator-only credential lifecycle, and the P3-003 read-only Canonical Mapper schema browser); mapping drafts and rotation remain gated.**
Tasks: P2L-001–P2L-005; P2C-004 (browser profile management); P2C-005/P2C-007 (credential lifecycle); P3-003 (read-only mapper schema browser).
The Phase 2 Registry UI was merged into `main` via PR #6 on 2026-10-21 (merge commit `0ba4827`) and the P2C workstream via PR #7; P3-003 is checkpointed on `phase-3/canonical-mapper` and is **not merged**.

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

## Canonical Mapper read-only schema browser (P3-003)

The Canonical Mapper consumes the same authorized Registry profiles but has an
independent, explicit source choice. It never reuses the Databases page state.

- **Mock adapter** (default) — the typed profile list is labeled `MOCK`; the
  schema groups come from a feature-local synthetic preview that is also labeled
  `MOCK` and `Mock preview`. No schema endpoint is called. An InfluxDB 1.x mock
  profile is shown as **Unknown**; a mock profile whose schema-discovery
  capability is not supported is shown as **Unsupported**. The preview is never
  used as a live fallback.
- **Live Registry API** — profiles come from the authorized backend after a
  session token is applied. Measurements, field keys and tag keys come only from
  the P3-002 `GET /api/v1/registry/schema/{measurements,fields,tags}` routes under
  `schema.read`. A live failure stays a failure; the mock preview is not
  rendered.

The operator must choose a **source and a distinct destination** explicitly.
The page sends only the stored `connection_id`, plus a measurement already
observed in the live measurement list for a field/tag read. It never sends an
endpoint, credential reference, credential value or query.

Measurements, fields and tags are separate typed groups. Field keys retain the
reported type (or say "not reported"); tag keys are a labelled list. A field or
tag read is issued only after a measurement is selected. Selecting a different
measurement changes the keys; it does not merge old data.

### States and cache safety

| State | What the page shows |
| --- | --- |
| Loading | An announced loading state for profiles, measurements, fields or tags. |
| Empty | "No measurements observed" (or equivalent for fields/tags) only for a positively **supported** empty answer. |
| Unknown | The observed outcome plus "Unknown is not a supported result", the edition and sanitized observations. No data. |
| Unsupported | "The deployment did not support this metadata read", the edition and observations. No data. |
| Permission / authentication | 401 or 403 wording; explicitly not a mock fallback. |
| Profile or measurement gone | 404 wording; no empty-schema claim. |
| Truncated | The measurement list plus an explicit truncation notice. |
| Stale | The answer is withheld and a "Stale schema answer" alert states which identity, scope, kind or measurement no longer matches. A reload control is offered. |

Every schema query key contains the selected `connection_id`, its canonical
scope and, for field/tag reads, the measurement. A selection, token or mode
change cancels and removes the previous schema queries; an in-flight answer
cannot render into the new selection. The two panels are keyed by connection ID
plus scope, so a scope change remounts the affected panel.

Safe identity only is rendered: display name, connection ID, family/major
version, canonical scope and observed edition. The page does not render an
endpoint or a credential reference. The session token remains memory-only. The
controls are native selects and buttons with visible focus styles, `aria-pressed`
on the mode and measurement controls, labelled lists, and `role="alert"`/
`role="status"` states; the keyboard and heading-order behaviour is covered by
the component tests.

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

**Phase 2 commands and actual outcomes:**
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

**Phase 3 P3-003 evidence (recorded 2026-10-26, branch
`phase-3/canonical-mapper`):**
- `cd ui && npx eslint .` → pass; `npx tsc -b` → pass;
  `npm run build` → pass.
- `cd ui && npx vitest run` → 23 files passed, 1 skipped; **223 tests passed**,
  1 skipped. The skipped test is the gated `schema.live.integration.test.ts`.
- `make check` → exit 0 (backend format/vet/build/tests, UI lint/build/test,
  policy check; 223/223 active UI tests passed, 1 gated test skipped).
- `bash scripts/policy-check.sh` → PASS (limited regex checks only); a scripted
  relative-link check of the touched documentation → 93 links, 0 broken.
- `./testing/influxdb/scripts/canonical-mapper-smoke.sh` → **6/6 PASS** against
  the isolated InfluxDB 2 OSS v2.9.1 container through the authenticated backend:
  anonymous profile request 401; admin profile create 201; fixed GET
  measurements returned `environment` + `sensor` with `outcome=supported` and
  `mock=false`; field read returned `value`; tag read returned `host` +
  `sensor_id`; the typed browser client read the same live profile and groups.
- The live client test asserts no credential/endpoint is rendered and every
  mapper request is a GET.
- **Manual real-browser/screen-reader accessibility evidence is an explicit
  gap.** The checks are jsdom component tests: keyboard activation of a
  measurement, native controls, one `h1`, no heading-level skips, unique IDs and
  labelled alert/status regions. No browser engine or screen reader was driven.

## Limitations

- The token is a static operator token; there is no login flow, expiry or
  rotation in the UI.
- Profile **update** is not implemented; create/delete are limited to unbound
  profiles, and credential attachment stays administrator-only.
- The Databases page and the Canonical Mapper schema browser are wired to live
  data; the other modules remain shells.
- The Canonical Mapper has no mapping drafts, canonical-schema versioning,
  import/export or sample-point preview.
- No real-browser visual/screen-reader validation was performed here (jsdom only);
  see the manual steps in [`DEVELOPER_SETUP.md`](DEVELOPER_SETUP.md). The P3-003
  manual accessibility evidence is the explicit gap recorded above.
- Only Linux amd64 is verified.
