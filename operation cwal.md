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
4. Confirm checkout is on `main`, fetch/check remote state, and identify pre-existing changes before editing. Never discard unrelated edits or overwrite remote changes.

## Execution
- Implement only the current task's explicit objective, work surface and acceptance criteria. Required directly related test fixes are allowed; materially new work or ambiguous scope is blocked.
- No unapproved deployments, live syncs, credential operations, historian writes/deletes, or destructive Docker actions.
- Enforce fixed dependency pins, secret protection and isolated runtime/worker behavior.
- Run required checks and capture commands, outcomes and limitations. No unsupported PASS claims.

## Completion/checkpoint
1. Verify every acceptance criterion; record test commands/results and relevant evidence.
2. After verification, move the task file with `git mv` to `archive/active-work/` and update the current task mirror and `handoff.md` together.
3. Commit implementation, evidence, archive move and handoff changes together on `main`. Push to `origin/main` after each verified task; never force-push. Confirm the remote commit matches the expected checkpoint before advancing.
4. A task is durably complete only when the push to `origin/main` succeeds and the remote tree contains the verified implementation and archived task. Advance only after confirming that checkpoint.
5. If commit/push fails or the remote branch has advanced, stop; retain local evidence and inspect Git status/history, current task location and remote HEAD before resuming. Do not overwrite concurrent changes or blindly retry.
6. When no authorized task exists, stop. On blocker before verification, keep the task in Active Work, document the blocker and do not archive it.

## Resource controls
- Bounded discovery: stop searching when the named implementation seam is found. Do not preload other tasks.
- On repeated identical failures, stop with a diagnostic summary rather than retry indefinitely.
- No automatic permission elevation, tool installation from unverified sources, or broad refactors.

## Human ownership
Humans create/approve plans, promote tasks, decide architecture and approve exceptional privileged actions. During project initialization, agents may commit and push verified Active Work directly to `main`; authority never extends beyond the current task.
