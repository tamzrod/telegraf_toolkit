# Telegraf Toolkit — CWAL handoff

## CURRENT OPERATION
**Workflow phase:** `initialization` (human-owned). Direct-`main` commits are permitted only for explicitly approved repository/governance setup, never application code. Normal implementation uses a designated work branch.
**Human-designated Phase 0 work branch:** `phase-0/foundation`. P0-001 through P0-004 were promoted there by explicit human instruction. This branch-local handoff records the designated ref and the package's task progression; the root `main` handoff records the designated branch only and does not replace these branch-local task pointers.

**Execution policy:** agents execute the already-authorized microtasks sequentially on this branch with real verification, archive, commit, push and a confirmed remote checkpoint after each task. After a confirmed `TASK_CHECKPOINTED`, the agent automatically advances to the next authorized microtask; no intermediate human review or merge gate. After final package checks the agent reports `WORK_PACKAGE_READY` and stops.

## Current Active Task
P0-005 — `workflow/active_work/P0-005-pre-pr-integration-corrections.md`, explicitly human-authorized for the current Phase 0 branch. P0-001–P0-004 remain checkpointed. Previous `WORK_PACKAGE_READY` is superseded pending correction verification and a new package checkpoint.

## Checkpointed tasks (this branch)
- P0-001 (domain/API contract inventory) — `TASK_CHECKPOINTED`. Deliverable `docs/contracts/README.md`; task file archived at `archive/active-work/P0-001-domain-contract-inventory.md`; implementation committed and pushed on `phase-0/foundation`.
- P0-002 (enabling architecture decisions) — `TASK_CHECKPOINTED`. Deliverables `docs/adr/0002`–`0005` (all **Proposed**, not Accepted); task file archived at `archive/active-work/P0-002-architecture-decision-records.md`; committed and pushed on `phase-0/foundation`.
- P0-003 (reproducible scaffold and CI) — `TASK_CHECKPOINTED`. Deliverables `backend/` Go API skeleton, `ui/` React+TS+Vite SPA skeleton, `.github/workflows/ci.yml`, `Makefile`, `docs/DEVELOPMENT.md` with pinned versions; task file archived at `archive/active-work/P0-003-scaffold-toolchain-and-ci.md`; committed and pushed on `phase-0/foundation`.
- P0-004 (isolated fixtures and developer guide) — `TASK_CHECKPOINTED`. Deliverables `backend/internal/fixtures`, `backend/internal/synccontract`, `ui/src/fixtures`, `docs/DEVELOPER_SETUP.md`; task file archived at `archive/active-work/P0-004-local-fixtures-and-developer-guide.md`.

## Authorized queued task IDs
P0-005 is current; no additional queued tasks. Do not self-promote further work.

## PR authority
- **PR initiation, review, approval and merge are human-only.** Agents never create, open, submit, approve or merge a pull request.
- Existing draft PR #2 (human-opened) is a human-controlled integration artifact, not a per-task pause condition. Agents must not create a duplicate or change PR state.

## Checkpoints
- Human explicitly authorized P0-005 as the pre-PR correction microtask on this branch; no product feature scope expansion is authorized.
- Human explicitly authorized all four original Phase 0 microtasks for promotion to this dedicated branch.
- Branch checkpoints are verified work, not integration into `main`. Integration occurs only after a human-initiated PR is merged.
- This handoff names the work branch, current task and queue; task-file presence governs authorization, not the handoff alone.
- Root `main` Active Work remains empty until a human-initiated integration merge; the empty `main` ledger neither cancels nor expands this branch's human authorization.

## Recovery
Inspect authorized ref, branch-local queue, archive/evidence, working tree, local/remote Git checkpoint and any human-initiated PR before resuming. Never force-push or repeat a confirmed checkpoint blindly. An archived task on an unmerged branch is `TASK_CHECKPOINTED`, not `INTEGRATED`.
