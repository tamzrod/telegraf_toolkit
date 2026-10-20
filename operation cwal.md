# Operation CWAL — Telegraf Toolkit

Autonomous, bounded OpenHands task execution. This is a repository workflow, not an installable package.

## Authorization and phase
- The only implementation authorization ledger is `workflow/active_work/`. Human promotion places an incomplete task `*.md` directly there; `README.md` is not a task. No agent promotion or self-selected tasks from Planning.
- There must be exactly one persistent `workflow/active_work/` directory. Never move, rename or remove the directory itself.
- `handoff.md` declares human-owned phase: `initialization` or `development`. Phase does not grant task authorization.
- Direct `main` applies only to human-approved repository initialization/infrastructure instructions, not ordinary application code. Any ambiguous task or phase takes the development branch/PR path.
- `handoff.md` and Active Work README must agree on the current task and the task file must exist on the authorized work ref. Normally promotion is recorded on `main`. For the explicitly human-approved Phase 0 batch on `phase-0/foundation`, the work-branch promotion is authorized for preparation/execution on that branch only; it does not authorize direct application-code pushes to `main`. If authorization/ref/pointers conflict, stop. Read future task filenames only, never their bodies.
- Single writer per task/work surface; only one direct-`main` writer. If exclusivity cannot be established or conflicting activity is detected, stop.

## Entry and bounded context
1. Read `AGENTS.md`, this operation, phase/current-task sections of `handoff.md`, `workflow/active_work/README.md`, and the one current task file.
2. Use BLACK SHEEP WALL with `ICC/INDEX.md` only for relevant context. Verify needed facts against `IDENTITY.md`, accepted ADRs and relevant source.
3. Check task whitelist, acceptance, stop conditions, and Git state. Fetch `origin/main` and validate that the task exists on the authorized ref (`main` by default; explicitly human-promoted Phase 0 batch on `phase-0/foundation` is the sole current exception). Never discard unrelated local changes.
4. Select Git mode: narrowly permitted setup documentation uses `main`; all application work uses a dedicated reviewed branch/PR. The human-designated Phase 0 branch is the working branch for the four promoted tasks.

## Execute and verify
- Work within the current task's stated objective and source/test surface. Directly required test fixes are allowed; ambiguous architecture, material scope expansion and destructive operations are blockers.
- Never deploy or restart production runtimes, alter credentials, or write/delete historians without separate explicit approval.
- Run all task-required checks; record commands, actual pass/fail/skip outcomes and evidence. Do not represent mocks as live integrations.
- Stop discovery once the required implementation seam is found. Stop after repeated identical failures; no blind retries or unverified installs.

## Completion — initialization-only direct main
1. After verified acceptance, `git mv` the task file to `archive/active-work/`; update the current-task pointers and `handoff.md` in the same commit as code and evidence.
2. Recheck remote `main` before publishing. Commit/push to `origin/main` without force, only when there are no conflicting changes or another writer.
3. Verify the remote commit/tree contains the implementation, evidence and archived task. Only then count the task durably complete and consider the next human-promoted task.

## Completion — regular development via PR
1. Implement and verify on a dedicated task branch. Stage a **provisional** `git mv` from Active Work into the archive plus evidence and handoff/current-pointer updates in the same proposed changeset.
2. Push the task branch and open a PR targeting `main`. Never directly push application code to `main`. Report actual checks; await configured checks and human review.
3. The task remains authorized and incomplete on the human-approved promotion ref while the PR is open (`main` by default; Phase 0 promotion on `phase-0/foundation` is an explicit exception). Do not advance to another task while awaiting required review/merge. If rejected, preserve the task on the promotion ref; revise within scope or stop for human direction.
4. Only after merge, inspect `origin/main`: verify the task no longer exists in Active Work, its archive and implementation are present, and the pointers agree. Then mark it durably complete and advance. If the promoted work batch was not previously present on `main`, the merged result must still contain the verified archive/evidence and consistent task pointer; the PR history must demonstrate the human-authorized promotion and completion.

## Recovery
- Before resuming, inspect working tree, local commits, `origin/main`, task location, handoff pointers, PR/check/merge state (if applicable) and recorded verification.
- Before verification: task stays in Active Work; document blocker. Verified but uncommitted: validate evidence and staged archive move before checkpointing.
- Committed but unpublished: compare remote ancestry and working tree; do not repeat implementation or force-push. Published direct-main but unconfirmed: inspect remote tree before advancing.
- PR open/rejected/checks failing: do not advance; preserve branch and task on `main`. PR merged: confirm archive, code, evidence and task absence on `main` before advancing.
- Concurrent remote changes, ambiguous state, missing authorization, or unreconcilable pointers: stop and report exact facts for human resolution.
- When Active Work is empty, stop. Never use Planning, ICC, archives or handoff as an alternative task queue.

## Human ownership
Humans approve task promotion, workflow phase changes, architectural exceptions, privileged actions and normal development PR merges. An agent never broadens its own authorization.
