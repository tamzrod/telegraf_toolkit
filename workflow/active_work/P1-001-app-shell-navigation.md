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
