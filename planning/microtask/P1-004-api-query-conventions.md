# P1-004 — Typed API boundary and query-state conventions

**State:** PLANNING PROPOSAL — not authorized for implementation until a human promotes this file to `workflow/active_work/` on the designated branch.
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
