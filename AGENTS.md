# Telegraf Toolkit — Agent Router

This is a short, always-on entry point for OpenHands and other development agents. It is not a project diary.

## Source of truth
- `IDENTITY.md` defines product scope, accepted constraints and delivery order.
- Accepted ADRs and domain specifications under `docs/` govern their respective subjects. If they conflict, do not invent a resolution: stop and request a human architectural decision.
- Git repository source is authoritative; ICC is a verified context cache, never execution authority.
- Distinguish documentation, mock behavior, and tested implementation. Never claim mocks contact live systems.

## Command routing
- `Operation CWAL` => read `operation cwal.md` and obey its task boundary.
- `BLACK SHEEP WALL` => read `BLACK_SHEEP_WALL.md`; use the minimum relevant ICC context.
- For normal review or planning, read the relevant specs and source without treating planning as permission to implement.

## Safety invariants
- Official Telegraf 1.40.1 is the initial baseline. No fork or floating dependency versions; use exact versions, lockfiles and verified production image digests.
- Preserve independent Telegraf runtimes, an independent sync engine and explicit approval before deploy, backfill or any write.
- Equal historian counts are not verified equality. Preserve measurement, tags, timestamp precision and field types.
- Keep secrets server-side; never print, export, commit or expose privileged database tokens, Docker socket control or runtime secrets to browsers.
- Never change production systems, credentials or historian data unless separately and explicitly authorized.

## Operating posture
- Follow the active task's whitelist; one task at a time, with bounded context.
- Prefer small coherent changes, tests and evidence. Do not modify unrelated work.
- During initial development, execute authorized CWAL tasks directly on `main` and push one verified checkpoint per task. Do not use task branches/PRs unless the human changes this policy. Report actual test results.
- Keep changes and documentation consistent. Missing tests, architectural contradictions and unverifiable acceptance are blockers, not successes.
