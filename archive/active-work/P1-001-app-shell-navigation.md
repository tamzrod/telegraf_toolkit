# P1-001 — SPA shell, routes and accessible navigation

**State:** HUMAN-AUTHORIZED ACTIVE WORK — promoted as part of the approved P1-001–P1-005 Phase 1 batch on `phase-1/application-shell`.
**Work branch:** `phase-1/application-shell`.
**Safety:** Mock-only UI; no live InfluxDB access, Docker control, credential mutation, Telegraf operations or historian writes. Never accept Proposed ADRs implicitly. No agent-created, approved or merged PRs.
**Checkpoint:** On human authorization, verify acceptance, archive via `git mv`, update queue/handoff, commit/push and confirm remote checkpoint before autonomously advancing.

## Objective and acceptance
**Goal:** Evolve existing Phase 0 React/TS/Vite scaffold into a responsive application shell with reusable layout, theme and routing. Preserve pinned toolchain and existing health status semantics.
**Work:** Implement shared header/sidebar/navigation, primary routes for Overview, Runtimes, Pipelines, Databases, Credentials, Canonical Mapper, Synchronizer, Audit/Settings, and not-found fallback; use existing React Router dependency. Keep modules explicitly non-operational.
**Acceptance:** Direct route navigation and back/forward work; mobile and keyboard navigation function; no new unpinned dependency; existing overview health/error display remains honest; route and navigation tests pass; `make check` passes.

## Dependencies and stop conditions
- Follow earlier authorized P1 checkpoints in sequence if promoted as a batch. Confirm branch, existing code and task-specific acceptance before implementation.
- Stop for unresolved architecture dependencies, destructive/privileged requests, unverified tests, conflicting work or material changes outside this proposal.

## Verification evidence (recorded 2026-10-20)

**Environment:** branch `phase-1/application-shell`; Node.js 24.21.0 / npm 11.19.1 (pinned, matched CI); Go 1.27.3 installed from the official `go.dev` tarball into `$HOME/go-sdk` because `/usr/local` is not writable for the non-root user and Go was absent from the image. No package versions changed.

**Deliverable paths:**
- `ui/src/app/modules.ts` — single module registry (id, path, label, description, honest status) consumed by navigation, routing and the Overview module list.
- `ui/src/app/AppLayout.tsx` — shared header, primary `<nav aria-label="Primary">`, skip link, labelled `<main id="main-content">`, responsive menu toggle.
- `ui/src/app/ModulePlaceholderPage.tsx` — explicit "Not implemented" module entry point.
- `ui/src/app/NotFoundPage.tsx` — not-found fallback.
- `ui/src/app/App.tsx` — nested shell routes for every registered module plus `*` fallback.
- `ui/src/app/App.test.tsx` — shell/navigation tests.
- `ui/src/features/overview/OverviewPage.tsx` — reads the shared registry; still reports `connected`/`scaffold` honestly.

**Commands and actual outcomes:**
- `make check` → exit 0. Backend gofmt/vet/build/test clean; frontend `npm ci` + `eslint` + `tsc -b && vite build` + `vitest run`; `scripts/policy-check.sh` → `PASS`.
- `npm test` → `Test Files 4 passed (4)`, `Tests 20 passed (20)`.

**Acceptance mapping:**
- Direct route navigation: tested (`renderApp("/synchronizer")` renders the Synchronizer placeholder without a click).
- Back/forward: tested with `BrowserRouter` + `window.history.back()/forward()`.
- Mobile + keyboard navigation: responsive menu toggle asserted via `aria-expanded`; skip link and labelled `main` asserted.
- No new unpinned dependency: `git diff --exit-code ui/package.json` reports unchanged; `policy-check` floating-version scan passes.
- Overview health/error honesty: existing assertions (`connected: false`, `scaffold: true`, backend-unreachable message) still pass.

**Limitations:** jsdom does not evaluate Tailwind CSS media queries, so responsive behaviour is verified at the ARIA/state level, not by visual layout. No live backend, database, Docker or historian contact was made or claimed.
