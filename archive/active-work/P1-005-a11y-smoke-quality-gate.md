# P1-005 — Accessibility, navigation smoke tests and Phase 1 handoff

**State:** HUMAN-AUTHORIZED ACTIVE WORK — approved for Phase 1 implementation on the designated branch.
**Work branch:** `phase-1/application-shell`.
**Safety:** Mock-only UI; no live InfluxDB access, Docker control, credential mutation, Telegraf operations or historian writes. Never accept Proposed ADRs implicitly. No agent-created, approved or merged PRs.
**Checkpoint:** On human authorization, verify acceptance, archive via `git mv`, update queue/handoff, commit/push and confirm remote checkpoint before autonomously advancing.

## Objective and acceptance
**Goal:** Prove Phase 1 shell is navigable and accessible enough for subsequent real feature development.
**Work:** Add keyboard/focus/semantic landmark checks, route smoke tests, responsive verification instructions, developer guide and appropriate CI tests. Update roadmap/handoff based on actual evidence only. Validate scope and runtime mock-only limitations.
**Acceptance:** Route/feature smoke suite passes; no critical keyboard navigation regression in tested flows; build/lint/unit tests pass via `make check`; report accessibility coverage and remaining limitations honestly; final package diff reviewed, verified task archive and empty queue; report `WORK_PACKAGE_READY` without opening PR.

## Dependencies and stop conditions
- Follow earlier authorized P1 checkpoints in sequence if promoted as a batch. Confirm branch, existing code and task-specific acceptance before implementation.
- Stop for unresolved architecture dependencies, destructive/privileged requests, unverified tests, conflicting work or material changes outside this proposal.

## Verification evidence (recorded 2026-10-20)

**Environment:** branch `phase-1/application-shell`; Node.js 24.21.0 / npm 11.19.1; Go 1.27.3 from `$HOME/go-sdk`. No package version changes.

**Deliverable paths:**
- `ui/src/app/accessibility.test.tsx` — accessibility and route smoke suite.
- `ui/src/components/common/ModulePageParts.tsx` — `PageHeader` changed from `<header>` to a plain container to remove a duplicate `banner` landmark (a real defect this suite caught).
- `docs/DEVELOPER_SETUP.md` — "Responsive and accessibility verification" section with manual steps and stated limits.
- `docs/ROADMAP.md` — Phase 1 items marked with a status note scoping them to verified branch work.

**Commands and actual outcomes:**
- `make check` → exit 0. Backend gofmt/vet/build/test clean (`ok` for fixtures, httpapi, synccontract); frontend `npm ci` + `eslint` + `tsc -b && vite build` + `vitest run`; `scripts/policy-check.sh` → `PASS`.
- `npm test` → `Test Files 11 passed (11)`, `Tests 123 passed (123)`.

**Acceptance mapping:**
- Route/feature smoke suite passes: `it.each` over all eight registered paths plus an unknown route asserts each renders its title without error.
- No critical keyboard navigation regression: tests assert the skip link is the first tab stop, every primary navigation link is reachable by keyboard and activates with Enter, and the responsive toggle is keyboard-operable with correct ARIA state.
- Semantic checks: per route, exactly one `main`, one `banner` and one labelled `navigation`; exactly one level-1 heading; no skipped heading levels; no duplicate element ids.
- `make check` passes (build/lint/unit tests, backend tests, policy checks).
- Honest accessibility reporting: coverage and jsdom limitations are documented in `docs/DEVELOPER_SETUP.md`; the roadmap note states responsive layout is verified semantically, permission states are illustrative, and Phase 1 is branch-verified rather than merged.
- Final package: verified archive for P1-001–P1-005, empty Active Work queue, `WORK_PACKAGE_READY` reported without opening a PR.

**Limitations:** Automated accessibility checks are semantic/state assertions in jsdom, not a full WCAG audit; no axe-style rule engine, real browser, screen reader or visual regression tooling is used. Manual responsive and screen-reader verification is documented but not executed here. No live backend, database, Docker or historian contact was made or claimed.
