# Telegraf Toolkit — Agent Router

Short, always-on entry point for OpenHands and development agents; not a project diary.

## Authority and commands
- `IDENTITY.md` sets product scope and constraints; accepted ADRs and relevant `docs/` specifications govern their subjects. Surface conflicts for human decision.
- Repository source is authoritative; ICC is a context cache, never execution authorization. Distinguish planned, mocked, implemented and verified.
- `Operation CWAL` → read `operation cwal.md` and execute its authorized work-package lifecycle.
- `BLACK SHEEP WALL` → read `BLACK_SHEEP_WALL.md` and the minimum relevant ICC context.
- Only humans authorize work packages, designate work branches and promote `workflow/active_work/` microtasks. Planning does not authorize implementation.

## Safety invariants
- Initial baseline is official Telegraf 1.40.1, not a fork. Exact dependency pins and lockfiles; verified production image digests; never floating tags.
- Isolate collector runtimes and historical sync. No implicit deploy, repair, restart or write.
- Count equality does not prove historian integrity; preserve timestamps, series tags and field types.
- Keep privileged credentials server-side; never commit, log, export or expose secrets or browser Docker control.
- Production operations, historian writes/deletes, credential access and architectural exceptions require separate explicit human authorization.

## Autonomous work and Git authority
- Agents execute already human-authorized microtasks **sequentially** on the designated work branch, with actual verification, archive, commit, push and remote checkpoint after each task. After a confirmed `TASK_CHECKPOINTED` state, automatically advance to the next authorized microtask; no intermediate human review or merge gate.
- Once all tasks pass package-level verification, report `WORK_PACKAGE_READY` with branch, SHA, evidence and limitations; **stop**. An unmerged branch checkpoint is not integrated into `main`.
- **PR authority is human-only.** Agents must never create, open, submit, approve or merge a pull request. Humans initiate PRs, enforce review/checks, approve and merge. Existing open PRs do not impose intermediate task gates.
- Direct `main` commits are a narrow **explicitly human-approved initialization/governance exception** only. Application scaffolding, code, features, fixes and tests use an authorized work branch, even during initialization.
- No self-promotion, automatic work-package expansion, force-push, overwrite of concurrent work or change of workflow phase. One writer per work surface, and only one direct-`main` writer.

## Operating posture
- Keep changes within the active task boundaries, with compact context and real test evidence. Missing verification, ambiguous authority, blocking decisions or privileged operations are stop conditions, not successes.
