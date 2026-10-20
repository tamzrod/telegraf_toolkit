# Telegraf Toolkit — CWAL handoff

## CURRENT OPERATION
**Workflow phase:** `initialization` (human-owned). Direct-`main` is permitted only for explicitly approved repository/governance setup, not application code. Normal implementation uses designated work branches.

**Human-designated Phase 0 branch:** `phase-0/foundation`. P0-001 through P0-004 were promoted there by explicit human instruction. This root handoff on `main` records the designated branch but does not replace its branch-local task pointers.

## Current Active Task on main
None. The main-branch Active Work folder is empty. Phase 0 task progression is tracked on its designated work branch.

## Checkpoints and PR authority
- Human-authorized microtasks execute sequentially with tested, committed, pushed and confirmed branch checkpoints; no human review gate between tasks.
- OpenHands advances after each `TASK_CHECKPOINTED`. After final checks it reports `WORK_PACKAGE_READY` and stops.
- **Only humans initiate/open, review/approve and merge pull requests.** PRs are strict integration gates, not intermediate microtask gates.
- PR #2 was already opened before the human-only PR rule; it is an existing integration artifact, not authorization to create another PR.
- Human controls workflow phase, architecture acceptance and any privileged actions.

## Recovery
Inspect authorized ref, branch-local queue, archive/evidence, local/remote Git checkpoint and any open PR. Never force-push or repeat work blindly. Changes archived only on a working branch are checkpointed, not integrated into `main`.
