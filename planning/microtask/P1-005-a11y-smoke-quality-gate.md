# P1-005 — Accessibility, navigation smoke tests and Phase 1 handoff

**State:** PLANNING PROPOSAL — not authorized for implementation until a human promotes this file to `workflow/active_work/` on the designated branch.
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
