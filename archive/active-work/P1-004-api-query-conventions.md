# P1-004 — Typed API boundary and query-state conventions

**State:** HUMAN-AUTHORIZED ACTIVE WORK — approved for Phase 1 implementation on the designated branch.
**Work branch:** `phase-1/application-shell`.
**Safety:** Mock-only UI; no live InfluxDB access, Docker control, credential mutation, Telegraf operations or historian writes. Never accept Proposed ADRs implicitly. No agent-created, approved or merged PRs.
**Checkpoint:** On human authorization, verify acceptance, archive via `git mv`, update queue/handoff, commit/push and confirm remote checkpoint before autonomously advancing.

## Objective and acceptance
**Goal:** Stabilize frontend integration seams for subsequent modules while keeping Phase 1 isolated.
**Work:** Consolidate typed health API client and TanStack Query patterns, cancellation, errors, mock adapters/fixtures clearly separated from real API; document feature folder/testing conventions. Do not introduce endpoints that claim real DB/runtime capabilities or make irreversible architectural persistence decisions.
**Acceptance:** Typed health request and response tested with mocked fetch; query loading/error/success and cancellation behavior covered; no credentials exposed in UI; API paths consistent with `docs/contracts/README.md`; `make check` passes.

## Dependencies and stop conditions
- Follow earlier authorized P1 checkpoints in sequence if promoted as a batch. Confirm branch, existing code and task-specific acceptance before implementation.
- Stop for unresolved architecture dependencies, destructive/privileged requests, unverified tests, conflicting work or material changes outside this proposal.

## Verification evidence (recorded 2026-10-20)

**Environment:** branch `phase-1/application-shell`; Node.js 24.21.0 / npm 11.19.1; Go 1.27.3 from `$HOME/go-sdk`. No package version changes.

**Deliverable paths:**
- `ui/src/api/client.ts` — single typed boundary: `API_BASE` (`/api/v1`), `ApiError` (status + URL, never a response body), `requestJson` (GET, `Accept: application/json`, `AbortSignal`, runtime guard), `fetchHealth`.
- `ui/src/types/health.ts` — added `isHealth` runtime guard.
- `ui/src/api/queryKeys.ts` — central query-key factory (`health`).
- `ui/src/hooks/useHealth.ts` — shared health query (`retry: false`) forwarding the query `AbortSignal`.
- `ui/src/api/mock/health.ts` — test-only mock adapter, isolated under `api/mock/`.
- `ui/src/features/overview/OverviewPage.tsx` — now consumes `useHealth` instead of an inline query.
- `docs/DEVELOPMENT.md` — "Frontend integration conventions (Phase 1)" section.
- Tests: `ui/src/api/client.test.ts`, `ui/src/hooks/useHealth.test.tsx`, `ui/src/api/boundary.test.ts`.

**Commands and actual outcomes:**
- `make check` → exit 0. Backend gofmt/vet/build/test clean; frontend `npm ci` + `eslint` + `tsc -b && vite build` + `vitest run`; `scripts/policy-check.sh` → `PASS`.
- `npm test` → `Test Files 10 passed (10)`, `Tests 76 passed (76)`.

**Acceptance mapping:**
- Typed health request/response with mocked fetch: `client.test.ts` asserts URL `${API_BASE}/health`, `GET`, signal forwarding, and an exact typed payload.
- Query loading/error/success and cancellation covered: `useHealth.test.tsx` asserts loading → success, error state, and that unmounting aborts the captured `AbortSignal`.
- No credentials exposed in UI: `boundary.test.ts` scans all sources for hardcoded credential-like assignments (none), and the client never sends or stores credentials.
- API paths consistent with `docs/contracts/README.md`: only the versioned `/api/v1/health` path is used; no endpoint claims DB/runtime capability.
- Mock separation: `boundary.test.ts` proves no application module imports `api/mock/` or `fixtures/`, and that every `fetch` call lives in `api/client.ts`.
- Negative controls (run, then reverted): appending a direct `fetch(...)` to a feature page failed the boundary test; appending a mock import failed the mock-separation test. Both files were restored with a clean `git diff`.

**Limitations:** Cancellation is verified by asserting the abort signal TanStack Query supplies, not by observing a real network teardown. `requestJson` is GET-only by design for Phase 1; write methods are intentionally not introduced. No live backend, database, Docker or historian contact was made or claimed.
