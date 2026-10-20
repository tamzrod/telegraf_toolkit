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
