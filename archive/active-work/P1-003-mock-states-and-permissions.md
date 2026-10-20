# P1-003 — Honest mock, loading, error, empty and permission states

**State:** HUMAN-AUTHORIZED ACTIVE WORK — approved for the Phase 1 batch.
**Work branch:** `phase-1/application-shell`.
**Safety:** Mock-only UI; no live InfluxDB access, Docker control, credential mutation, Telegraf operations or historian writes. Never accept Proposed ADRs implicitly. No agent-created, approved or merged PRs.
**Checkpoint:** On human authorization, verify acceptance, archive via `git mv`, update queue/handoff, commit/push and confirm remote checkpoint before autonomously advancing.

## Objective and acceptance
**Goal:** Establish consistent demonstrative UX states without pretending there is backend authentication/authorization.
**Work:** Reusable labeled mock/empty/loading/error/permission-denied presentations; selectable local-only demos where useful; ensure any permission simulation is explicitly labeled illustrative, not enforced access control. Preserve no-secret/no-live-side-effect safeguards.
**Acceptance:** Module representative states are testable and visibly labeled; no unauthorized side effects; UI never reports mock data as live or simulated permission as server-enforced; loading/retry behavior for health endpoint is tested; `make check` passes.

## Dependencies and stop conditions
- Follow earlier authorized P1 checkpoints in sequence if promoted as a batch. Confirm branch, existing code and task-specific acceptance before implementation.
- Stop for unresolved architecture dependencies, destructive/privileged requests, unverified tests, conflicting work or material changes outside this proposal.

## Verification evidence (recorded 2026-10-20)

**Environment:** branch `phase-1/application-shell`; Node.js 24.21.0 / npm 11.19.1; Go 1.27.3 from `$HOME/go-sdk`. No package version changes.

**Deliverable paths:**
- `ui/src/components/common/StatePanels.tsx` — `MockDataNotice`, `LoadingState` (`role="status"`, `aria-live="polite"`), `EmptyState`, `ErrorState` (`role="alert"`, optional retry), `PermissionDeniedState`.
- `ui/src/components/common/StateDemo.tsx` — local-only state selector (`aria-pressed`) demonstrating each state; performs no request.
- All seven non-Overview feature pages render `StateDemo`.
- `ui/src/features/overview/OverviewPage.tsx` — health query now uses `LoadingState`/`ErrorState` with a working `refetch` retry.
- Tests: `ui/src/components/common/StatePanels.test.tsx`; extended `ui/src/features/overview/OverviewPage.test.tsx`.

**Commands and actual outcomes:**
- `make check` → exit 0. Backend gofmt/vet/build/test clean; frontend `npm ci` + `eslint` + `tsc -b && vite build` + `vitest run`; `scripts/policy-check.sh` → `PASS`.
- `npm test` → `Test Files 7 passed (7)`, `Tests 54 passed (54)`.

**Acceptance mapping:**
- Representative states testable and visibly labeled: each state has a dedicated test; `MockDataNotice` asserts "Sample data only" and that data is "not read from any database" and "does not represent a live connection".
- No unauthorized side effects: `StateDemo` tests assert `fetch` is never called while switching states or retrying; a shell test asserts every `fetch` URL across all module routes is `/api/v1/health`.
- Mock data never reported as live; permission simulation explicitly illustrative: `PermissionDeniedState` asserts the "Illustrative permission state — not enforced" label and the explicit "no authentication or server-side authorization" statement.
- Loading/retry behaviour for the health endpoint tested: a deferred-promise test asserts the loading state before resolution and its removal after; a retry test asserts the error state, a successful second `/api/v1/health` call and exactly two total calls.

**Limitations:** These are UI-only demonstrations; no authentication, authorization or backend state exists, and none is claimed. The permission preview is presentation only and must not be treated as access control. No live backend, database, Docker or historian contact was made or claimed.
