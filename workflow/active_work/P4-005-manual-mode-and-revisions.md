# P4-005-manual-mode-and-revisions — Explicit TOML ownership and safe revision lifecycle

**STATE:** AUTHORIZED  
**BRANCH:** `phase-4/pipeline-designer`  
**ORDER:** 5 of 7  
**DEPENDENCY:** P4-004 checkpointed  
**DESIGN AUTHORITY:** `docs/PHASE_4_PLAN.md` and existing accepted ADRs/contracts.

## Objective and allowed scope
Manual editor, draft APIs/storage/authorization and tests; no deploy/apply.

## Required implementation
Exactly one authoritative graph or manual TOML mode; explicit mode transition with recoverable snapshots and difference review, deny unsupported TOML-to-graph conversions; draft save/reload, history, approval immutable and rollback via new revision; transactional/concurrent-safe auditing and RBAC.

## Acceptance and evidence
Mode transition/loss-prevention tests, approval immutability and transaction/race tests, authorization failures; `make check` and appropriate race tests; Save and Approve never deploy.

## Safety / stop conditions
Do not edit `main`, open PRs, force-push, deploy, restart Telegraf, write historians, start synchronization, expose secrets, or self-expand the work package. If required contracts conflict or a privileged action is necessary, record blocker rather than invent permissions.

## CWAL checkpoint
Run real checks; report limitations; archive this task under `archive/active-work/`, commit and push evidence and advance both handoff pointers to the next authorized task automatically. Do not request intermediate approval.