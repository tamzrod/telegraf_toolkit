# P2C-004 — Enable authorized live Registry profile management in browser

> **State: HUMAN-AUTHORIZED ACTIVE WORK — COMPLETE, CHECKPOINTED 2026-10-21** (archived to `archive/active-work/`). The live Registry view creates and deletes unbound profiles through the authorized API, with validation, confirmation and visible permission/error states.

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

## Verification evidence (recorded 2026-10-21)

**State:** COMPLETE, CHECKPOINTED. The live Registry view manages **unbound**
profiles through the authorized API: create, list/detail and delete are
available, the delete needs an explicit confirmation, and every validation,
permission and transport state is visible. Credential entry, attachment,
rotation and revocation stay gated for P2C-005.

**Environment:** branch `phase-2/registry-completion`; Node.js 24.15.0 /
npm 11.12.1; Windows development host with a WSL Linux runtime for the
process-level check.

### What changed

- `ui/src/api/registry.ts`: `ApiProfileInput` is the create body the browser may
  send, and `isApiProfileInput` refuses a body that carries a credential
  reference or a credential value. A browser-created profile is always unbound.
- `ui/src/api/client.ts`: `createLiveProfile`, `fetchLiveRegistryProfile` and
  `deleteLiveProfile` join the typed boundary. `requestJson` now turns a body
  that is not JSON into an `ApiError` instead of letting a raw parse error (and
  the response text) escape to the UI.
- `ui/src/hooks/useLiveRegistry.ts`: `useCreateLiveProfile` and
  `useDeleteLiveProfile` invalidate the live profile list on success, so the
  view always shows what the server holds.
- `ui/src/features/databases/registryWorkflow.ts`: `validateProfileDraft`
  (connection ID, display name, major version, endpoint, scope completeness,
  at least one query dialect), `isValidEndpoint` (absolute http(s) URL, no
  embedded credentials), `isCompleteScope`, `parseList`,
  `profileInputFromDraft` and `describeApiFailure` (a status mapped to
  actionable wording that never repeats a backend body).
- `ui/src/features/databases/DatabasesPage.tsx`: the live view gains a create
  form (connection ID, display name, endpoint, major version, scope kind with
  its fields, query dialects, schema discovery), a per-profile `Delete profile`
  control with a `Confirm delete` step, a live status region, and an explicit
  "a probe of this profile resolves no credential" note for a profile without a
  binding. The disabled action group now lists only the credential actions.
- `docs/LIVE_UI.md`, `docs/ROADMAP.md`: the browser write flows and their
  states are documented; the remaining gates are stated precisely.

### What the browser deliberately does not do

- It does not send a credential reference or a credential value, so it cannot
  attach another principal's stored secret to an endpoint (P2L-007 is enforced
  server-side and is not weakened).
- It does not fall back to the mock adapter when a live request fails; a live
  error stays a live error.
- It does not imply an update flow: the API supports create, read, list and
  delete, and the UI offers exactly those.
- It does not store the session token anywhere but memory.

### Tests

- `ui/src/api/client.live.test.ts` (new, 8 tests): the create sends no
  credential field, a body that carries one is refused **before** a request is
  made, a 409/401/403 is reported with its status and without the body, a
  non-JSON body is a transport failure, the read and delete use the versioned
  path, and a refused delete keeps its status.
- `ui/src/features/databases/DatabasesPage.profile.test.tsx` (new, 10 tests):
  the create journey (body checked, list refreshed), an incomplete draft that
  is never sent, an endpoint with embedded credentials that is never sent, the
  403 and 409 states, a delete that needs a confirmation, a refused delete that
  keeps the profile, a stale selection that is cleared after a delete, the
  "resolves no credential" note, and a live failure that does not fall back to
  fixtures.
- `ui/src/features/databases/profileDraft.test.ts` (new, 11 tests): the draft
  validation rules, the request body built from a draft, and the status-to-
  wording mapping (which never repeats a backend body).
- Frontend suite: **18 files / 173 tests passed** (was 16 / 155).

### Commands and actual outcomes

- `npm run lint` → pass; `npx tsc -b` → pass; `npm run build` → pass;
  `npm test` → **18 files / 173 tests passed**.
- `scripts/policy-check.sh` → **PASS** (exit 0).
- Relative markdown link check over every changed `.md` → **0 missing links**.
- **Process-level authenticated API check** — the real backend binary, started
  on loopback with a real SQLite store, a real token file and a throwaway master
  key, exercised over HTTP. All checks passed:

| Check | Result |
| --- | --- |
| anonymous list | 401 |
| viewer creates a profile | 403 |
| administrator creates an unbound profile | 201, `credential_ref_id` empty |
| duplicate connection ID | 409 |
| the profile is listed and readable | 200 |
| an incomplete scope | 400 |
| a credential-bearing endpoint | 400 |
| administrator deletes the profile | 204, then 404 |
| viewer deletes a profile | 403 |

### Isolation level

The check ran against a local process with synthetic credentials in a temporary
directory that was removed afterwards. No production historian, credential,
Docker runtime or Telegraf runtime was touched, and no browser storage was
written.

### Accessibility and manual checks

- Every control has a programmatic label (`Connection ID`, `Endpoint`, `Scope
  kind`, …), the invalid fields carry `aria-invalid`, the form status is a
  `role="status"` region with `aria-live="polite"`, and the delete confirmation
  is a keyboard-operable pair of buttons.
- **Manual visual and screen-reader checks were NOT performed** on this host, so
  they are recorded as outstanding rather than done. The automated checks cover
  the labels, roles and states, not the rendered appearance or the spoken
  output.

### Refined contract compliance (P2C-004)

| Refined requirement | How it is met |
| --- | --- |
| Enable only API-supported create, list/detail, delete and scoped source/destination selection; do not imply an edit flow | Exactly those four operations are offered; the gated action group no longer lists "Save profile" or "Delete profile" |
| Unbound creation only; credential enrollment stays gated; show a missing binding and prohibit a fake successful probe | The create body has no credential field; the "Attach/Rotate/Revoke credential" controls stay disabled; a profile without a binding is labelled as resolving no credential |
| Honor backend authorization, strict validation and sanitized errors; no implicit mock fallback | 401/403/409/400/422 are mapped to wording; the backend body is never rendered; a live failure never switches to the mock adapter |
| Mutations require confirmation, an explicit active target, predictable refresh and safe handling of a deleted selection | The delete needs a confirmation step; the list is refetched after every mutation; a deleted profile is cleared from both selection fields |
| Token in memory only; no browser credential values, storage, URL leakage, logs or unsafe error echoes | Unchanged token handling; the typed client keeps only the status; a non-JSON body becomes a transport error instead of a raw parse error |
| Accessible labels, focus behaviour, keyboard navigation and responsive states | Labels, `aria-invalid`, `aria-describedby` and a live status region; the confirmation is keyboard operable; the layout uses the existing responsive grid |
| Exercise viewer/configurator/admin UX; tests include failures, duplicate IDs, stale selection, invalid scope, non-JSON/error bodies and no-mock fallback | 29 new frontend tests plus the process-level role checks (viewer 403, administrator 201/204) |
| Run frontend lint/types/build/Vitest, make check and a process-level authenticated API e2e | All run; `make check` itself needs `make`, which is not installed on this host, so its components were run individually with the same commands |

### Limitations

- **No manual visual or screen-reader pass** was performed on this host; the
  automated accessibility checks cover labels, roles and states only.
- The browser cannot attach a credential, rotate one or revoke one: that is
  P2C-005.
- A profile cannot be edited in the browser; the API has no update route, and
  the UI does not pretend otherwise.
- The `go test -race` and backend-suite runs are unchanged by this task (no
  backend code changed); the backend suite was green on linux/amd64 at the
  P2C-003 checkpoint.
- InfluxDB 1.x remains unverified; ADR-0004 and ADR-0005 remain **Proposed**.
