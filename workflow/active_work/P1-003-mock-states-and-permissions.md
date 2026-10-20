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
