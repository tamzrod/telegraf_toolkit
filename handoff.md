# Telegraf Toolkit — CWAL handoff

## CURRENT OPERATION
**Workflow phase:** `initialization` (human-owned), with application work governed by branch/PR workflow.
**Human-authorized Phase 0 work branch:** `phase-0/foundation`.
**Promotion:** P0-001 through P0-004 explicitly promoted by human request to this work branch. They are not authorized on `main` until the promotion is merged there.
**Execution policy:** one current task at a time; Phase 0 work remains on the dedicated branch. Human-reviewed PR and merge are required for application code. Do not push application implementation directly to `main`.

## Current Active Task
None pending execution. P0-001 implemented and verified on `phase-0/foundation`; awaiting human-reviewed PR/merge. Provisional archive move staged in the open PR; not durably complete until confirmed on `main`.

## Authorized queued task IDs
P0-002, P0-003, P0-004. Queued files must not be opened before the preceding task reaches its durable checkpoint. Do not begin P0-002 while P0-001 awaits review/merge.

## Checkpoints
- Human explicitly authorized all four Phase 0 microtasks for promotion to this dedicated branch.
- P0-001 (contract inventory) implemented at `docs/contracts/README.md`; task file provisionally archived; pending human review/merge.
- This handoff names the work branch and current task; task-file presence governs authorization, not the handoff alone.
- Existing `main` Active Work remains empty until a reviewed promotion/implementation merge.
- No application work executed as part of the promotion.

## Recovery
Inspect branch/ref, task file presence, pointers, verification evidence, local commits, open PR and `origin/main` before resuming. Never infer completion from an archive move on an unmerged branch.
