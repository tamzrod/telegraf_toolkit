# Operation CWAL — Telegraf Toolkit

Autonomous **work-package execution** with verified microtask checkpoints and human-controlled PR integration. This document is a workflow, not an installation package.

## Authority, authorization and work ref
- Humans authorize a bounded work package, designate its work branch and promote its microtasks into `workflow/active_work/` in an explicit order. `README.md` is metadata, not an executable task; agents must not select Planning tasks or create new authorizations.
- Maintain exactly one persistent `workflow/active_work/` directory per ref; never rename or delete it.
- The designated branch and human promotion are authoritative for that package. Normally tasks can be promoted on `main` before a work branch is created; explicitly approved work-branch promotions (such as `phase-0/foundation`) are equally valid **on that branch**. Do not infer authorization from `main` being empty.
- `handoff.md` and the Active Work README must agree on the designated ref, current task and queue. If inconsistent, stop for human reconciliation. Inspect queued filenames only, not their bodies.
- One writer per task/work surface. An unknown or conflicting writer is a blocker.
- `handoff.md` declares a human-owned workflow phase. The sole direct-`main` exception is explicitly approved repository initialization/governance, not application code. Normal work uses the human-designated branch.

## Task entry and bounded context
1. Read `AGENTS.md`, this operation, handoff and Active Work pointers, then **only** the current authorized task.
2. Apply BLACK SHEEP WALL using `ICC/INDEX.md`, and verify relevant facts against `IDENTITY.md`, accepted ADRs, specs and actual source.
3. Fetch remote refs; validate designated branch and task/pointers, inspect working tree and pre-existing changes. Never discard unrelated edits or overwrite concurrent remote changes.
4. Check the task objective, allowed surface, acceptance criteria, dependencies and stop conditions. Missing architecture or unsafe/unapproved actions require a human decision.

## Autonomous microtask loop
1. Implement **only** the current task's authorized objective. Required directly related test repairs are allowed; substantial scope expansion is not.
2. Run acceptance checks and record commands, real outcomes, evidence, skips and limitations. Never claim mocks contact live systems.
3. On success, `git mv` the task into `archive/active-work/`, advance the queue pointer to the next **already authorized** task (or none), and update `handoff.md` together with code/evidence.
4. Commit and push to the designated work branch; never force-push. Confirm the remote commit/tree includes implementation, evidence, archive and consistent next-task pointers.
5. Record `TASK_CHECKPOINTED`. **Automatically continue** with the next queued, human-authorized task. Do not wait for a PR, human review or merge between tasks.
6. On failure, retain the task in Active Work, record `TASK_BLOCKED` and stop if a safe within-scope repair is not possible. Never archive a failing task.

## Work package completion and integration
- After the last microtask is checkpointed, rerun package-level checks and validate all requested deliverables, evidence, archive files, clean Git status and the remote branch SHA.
- Only when required checks pass, record/report `WORK_PACKAGE_READY` with branch, final SHA, verification, unresolved **nonblocking** decisions, risks and limitations. Stop execution; do not open a PR.
- **Only a human may create/open/submit, review/approve or merge a PR.** Human-initiated PRs must pass configured CI, architecture/security/scope review and required approvals before merge. No agent may merge or self-approve.
- After human merge, confirm the intended changes and archives landed on `origin/main` before declaring `INTEGRATED`. Branch checkpoints are verified work, not integration.
- An already-open draft PR (including Phase 0 PR #2) is a human-controlled integration artifact, **not** a per-task pause condition. Agents must not create a duplicate or change PR state. Authorized scoped review fixes can be made on the branch; human retains PR control.
- For explicitly authorized direct-`main` initialization documentation, the confirmed remote checkpoint itself is integrated; no feature/code work is eligible for this exception.

## Recovery and stop conditions
- Resume by checking current branch, remote head, dirty tree, task presence/archive, queue pointers, previous checkpoint/evidence, and any human-initiated PR state. Never repeat a checkpoint blindly.
- Verified but uncommitted: revalidate evidence and staged changes. Committed but not pushed: reconcile remote history safely. Pushed but unconfirmed: inspect remote tree. After confirmed checkpoint: advance without requesting human review.
- If tests cannot be verified, architecture approval is necessary for the next task, a dependency cannot be satisfied safely, a security/production privilege is required, or concurrent changes conflict: stop, preserve evidence and report exactly what is blocking. Proposed ADRs may be drafted without human approval, but must not be marked Accepted.
- When no authorized task remains, do only final package checks, report readiness/blockers and stop. Planning, ICC, handoff and archives never independently authorize new work.
- Do not install unverified tools, elevate permissions, refactor outside scope, retry identical failures indefinitely, or expose secrets.

## Ownership summary
**Human:** work authorization and branch, task promotion, phase/architecture changes, privileged actions, PR initiation/review/approval/merge.  
**Agent:** bounded implementation, evidence, verified branch checkpoint, automatic progression through already-authorized tasks, and final readiness report.


## Hardened checkpoint and completion guardrails (all runners, including Jr Qwen)
These checks **strengthen**, not replace, existing human authorization, scope boundaries and autonomous progression. All status claims must be grounded in the designated remote branch, not a local summary.

1. **Acceptance matrix first.** For CURRENT, enumerate each acceptance criterion and its specific code change, test or inspection evidence. Passing unrelated tests or reporting a count is not proof that each criterion passes. Do not silently downgrade a requirement.
2. **Truthful verification.** Record exact commands, exit codes, test passes/failures/skips, runtime/toolchain versions, and explicit limitations. Distinguish syntax validation from Telegraf semantic validation, mocked tests from live integration, and tests run now from previously reported runs.
3. **Transactional checkpoint gate.** Do not claim `TASK_CHECKPOINTED` until criteria and mandated checks pass; the task is moved from `workflow/active_work/` to `archive/active-work/`; both pointers match the next authorized task; relevant changes and evidence are committed/pushed; and the **remote SHA and remote tree** confirm implementation, archive and pointers. A commit or test pass by itself is insufficient.
4. **Protected `WORK_PACKAGE_READY`.** Require the entire authorized queue to be archived; `CURRENT: none`, `NEXT: none` and `WORK_PACKAGE_READY` in both pointer documents; package-level checks pass; remote commit and paths verified; local tree clean; unresolved limitations disclosed. Otherwise report `IN_PROGRESS` or `TASK_BLOCKED`—never `READY`.
5. **Challenge protocol.** If a claim is questioned, stop repeating it. Re-read remote HEAD, handoff, Active Work README, CURRENT file and archive; compare each acceptance requirement. Correct inaccurate claims, then resume outstanding authorized work if within runner permissions.
6. **Runner roles are binding.** A test-only runner may verify and report but may not checkpoint implementation without authority. An implementation runner must actually complete and checkpoint the CURRENT task. Never self-grant permissions or claim another runner's work as independently verified.
7. **Safe failure statement.** If remote verification is unavailable, report `REMOTE_CHECKPOINT_UNVERIFIED — COMPLETION NOT CLAIMED`. If a criterion fails or required verification cannot run, report `TASK_BLOCKED` with specific evidence and preserve existing changes.
8. **Final evidence record.** Include designated branch, full verified remote SHA, exact task name, acceptance-by-criterion PASS/FAIL, commands/results/skips, verified archive path, exact CURRENT/NEXT pointers, local tree status, and remaining limitations.

**Absolute gate: NO verified remote checkpoint = NO completion. NO acceptance evidence = NO pass. NO archived task plus synchronized pointers = NO `WORK_PACKAGE_READY`.**

Do not interpret these guardrails as a new CWAL implementation task or as authorization to alter an active queue; the currently designated, human-authorized task remains CURRENT until genuinely checkpointed.
