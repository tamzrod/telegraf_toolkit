# P1-002 — Feature entry points and shared UI conventions

**State:** HUMAN-AUTHORIZED ACTIVE WORK — promoted as part of the approved P1-001–P1-005 Phase 1 batch on `phase-1/application-shell`.
**Work branch:** `phase-1/application-shell`.
**Safety:** Mock-only UI; no live InfluxDB access, Docker control, credential mutation, Telegraf operations or historian writes. Never accept Proposed ADRs implicitly. No agent-created, approved or merged PRs.
**Checkpoint:** On human authorization, verify acceptance, archive via `git mv`, update queue/handoff, commit/push and confirm remote checkpoint before autonomously advancing.

## Objective and acceptance
**Goal:** Build feature-local pages for each Phase 1 module without implementing Phase 2+ behavior.
**Work:** Consistent page title, scope and status text; shared presentation components; clear disabled actions; per-feature ownership and simple typed interfaces. Do not imply connections, deployments, runtime management or user secrets work.
**Acceptance:** All eight main areas have reachable distinct pages; placeholders are visibly non-operational; no network call except approved scaffold health endpoint; maintainable feature-local tests; `make check` passes.

## Dependencies and stop conditions
- Follow earlier authorized P1 checkpoints in sequence if promoted as a batch. Confirm branch, existing code and task-specific acceptance before implementation.
- Stop for unresolved architecture dependencies, destructive/privileged requests, unverified tests, conflicting work or material changes outside this proposal.

## Verification evidence (recorded 2026-10-20)

**Environment:** branch `phase-1/application-shell`; Node.js 24.21.0 / npm 11.19.1; Go 1.27.3 from `$HOME/go-sdk` (image lacks Go and `/usr/local` is not writable for the non-root user). No package version changes.

**Deliverable paths:**
- `ui/src/components/common/ModulePageParts.tsx` — shared `PageHeader`, `StatusBadge`, `NotImplementedNotice`, `DisabledAction`, `DisabledActionGroup`.
- `ui/src/features/runtimes/RuntimesPage.tsx`, `pipelines/PipelinesPage.tsx`, `databases/DatabasesPage.tsx`, `credentials/CredentialsPage.tsx`, `canonical/CanonicalMapperPage.tsx`, `synchronizer/SynchronizerPage.tsx`, `settings/SettingsPage.tsx` — feature-local pages with consistent title, scope, status and disabled actions.
- `ui/src/app/App.tsx` — `MODULE_PAGES` map resolves each module id to its feature page; a missing page throws rather than rendering blank.
- Removed `ui/src/app/ModulePlaceholderPage.tsx` (superseded; no remaining references).
- Tests: `ui/src/features/moduleEntryPoints.test.tsx`, `ui/src/components/common/ModulePageParts.test.tsx`.

**Commands and actual outcomes:**
- `make check` → exit 0. Backend gofmt/vet/build/test clean; frontend `npm ci` + `eslint` + `tsc -b && vite build` + `vitest run`; `scripts/policy-check.sh` → `PASS`.
- `npm test` → `Test Files 6 passed (6)`, `Tests 43 passed (43)`.

**Acceptance mapping:**
- All eight main areas have reachable distinct pages: `it.each` renders every registered module path and asserts exactly one level-1 heading matching the module label.
- Placeholders visibly non-operational: each non-Overview module asserts a `role="status"` notice and a labelled "Planned actions" group whose buttons are all `disabled` + `aria-disabled="true"`.
- No network call except the approved health endpoint: shell test records every `fetch` call across all module routes and asserts each URL is `/api/v1/health`.
- Maintainable feature-local tests: page-level tests live under `features/` and `components/`; shared primitives tested independently.

**Limitations:** Only Overview issues a network request at all; the other modules are static and were confirmed to issue none. Responsive CSS is verified at ARIA/state level only (jsdom does not evaluate media queries). No live backend, database, Docker or historian contact was made or claimed.
