# Telegraf Toolkit — CWAL handoff

## CURRENT OPERATION
**Workflow phase:** `initialization` (human-owned). Direct-`main` exception applies only to approved repository setup and workflow/governance documents; ordinary application development always uses branch + PR. Transition to `development` requires human authorization.

## Current Active Task
None. `workflow/active_work/` is empty.

## Checkpoints
- Root OpenHands/CWAL workflow is established.
- Git policy distinguishes temporary initialization commits from regular task branches, reviewed PRs and merged completion.
- Human-promoted Active Work files alone authorize execution; handoff is a mirror, not authority.
- One CWAL writer per task/work surface; one direct-`main` writer during initialization.

## Recovery
For interrupted work, inspect local changes, `origin/main`, task presence, current pointers, verification, and any task branch/PR before continuing. Never force-push or advance until a durable remote checkpoint is confirmed. Archived changes on an unmerged branch are provisional.
