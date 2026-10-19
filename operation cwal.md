# Operation CWAL — Telegraf Toolkit

Autonomous, bounded OpenHands execution. This command is a workflow, not an installation package.

## Authorization
- The **only** implementation authorization ledger is `workflow/active_work/`.
- Each direct child `*.md` other than `README.md` is a human-promoted, incomplete task. No agent may promote tasks from `planning/` or choose work outside Active Work.
- Exactly one canonical `workflow/active_work/` directory must exist and persist. Never rename/delete it.
- `handoff.md` and the Active Work README are informational pointers, not additional authorization sources. If their current-task pointers disagree or the named file is absent, stop for human reconciliation.
- Inspect current-task filenames to locate the pointer, but do not read future task bodies.

## Task entry (read narrowly)
1. Read `AGENTS.md`, this command, the current sections of `handoff.md` and `workflow/active_work/README.md`, and the selected current task file.
2. Run BLACK SHEEP WALL for only the task's declared work seam; load the smallest valid ICC nodes and verify necessary claims against repository source.
3. Check `IDENTITY.md`, relevant accepted ADRs and domain contracts. Conflicting or missing architecture requires a human decision.
4. Confirm clean task branch state and identify pre-existing changes. Never discard unrelated edits.

## Execution
- Implement only the current task's explicit objective, work surface and acceptance criteria. Required directly related test fixes are allowed; materially new work or ambiguous scope is blocked.
- No unapproved deployments, live syncs, credential operations, historian writes/deletes, or destructive Docker actions.
- Enforce fixed dependency pins, secret protection and isolated runtime/worker behavior.
- Run required checks and capture commands, outcomes and limitations. No unsupported PASS claims.

## Completion/checkpoint
1. Verify every acceptance criterion; record test commands/results and relevant evidence.
2. Move the verified task file with `git mv` to `archive/active-work/`, and update the current task mirror and `handoff.md` in the same change.
3. Commit code, evidence and archive/handoff bookkeeping atomically on a dedicated task branch; publish a pull request targeting `main`. Never push directly to `main`.
4. A task counts as durably complete only after the configured human review/merge gate is satisfied. Do not start the next task while this gate is pending.
5. If publishing, checks or merge fails, do not silently retry or advance: preserve the branch, task evidence and exact blocker. On resume inspect branch, PR, merge and local status before rerunning any step.
6. When no authorized task exists, stop. On blocker, leave the active task present and document what is missing; do not archive a failed task.

## Resource controls
- Bounded discovery: stop searching when the named implementation seam is found. Do not preload other tasks.
- On repeated identical failures, stop with a diagnostic summary rather than retry indefinitely.
- No automatic permission elevation, tool installation from unverified sources, or broad refactors.

## Human ownership
Humans create/approve plans, promote tasks, decide architecture, approve exceptional privileged actions, and merge reviewed work. Agent autonomy exists only inside the current authorized task.
