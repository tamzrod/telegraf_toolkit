# Telegraf Toolkit — CWAL handoff

## CURRENT OPERATION
**Workflow phase:** `development`, human-designated branch `phase-1/application-shell`.
**Authorized package:** Phase 1 Application Shell — P1-001 through P1-005. Human explicitly approved promotion of all five proposed microtasks into Active Work as one sequential batch.

## Current Active Task
**P1-002:** `workflow/active_work/P1-002-feature-module-entrypoints.md`.

## Authorized queue (strict sequence)
1. P1-001 — SPA shell, routes, accessible navigation — **checkpointed** (`archive/active-work/P1-001-app-shell-navigation.md`)
2. P1-002 — feature module entry points (**current**)
3. P1-003 — mock/empty/loading/error/permission states
4. P1-004 — typed API and query-state conventions
5. P1-005 — accessibility, smoke tests, final quality gate

The queue is authoritative only through the human-promoted task files under `workflow/active_work/`; this handoff mirrors its README. OpenHands may automatically execute subsequent tasks **only after** verifying, archiving, committing, pushing and confirming each earlier checkpoint. If blocked, stop with actual evidence. Do not read future task bodies before they are current.

## Previous milestone
Phase 0 PR #2 was merged to `main`; P0-001–P0-005 are integrated. Phase 1 must retain the explicitly mock/scaffold-only boundaries. ADR-0002–0005 remain Proposed; do not silently accept or implement them.

## Integration and safety
PR initiation, review, approval and merge belong solely to humans. No agent PR creation or changes of PR state. No live database operations, historian writes, Docker control, credential operations or scope expansion without separate authorization. After completing the package, perform full checks and report `WORK_PACKAGE_READY` for human pre-PR audit.

## Recovery
Inspect the current branch, Git status, the Active Work task file and pointers, archived tasks, verification evidence and remote commit before resuming. Never force-push, overwrite other writers or redo confirmed tasks blindly.
