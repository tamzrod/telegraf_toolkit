# P4-006-mapping-compatibility — Phase 3 approved mapping integration

**STATE:** AUTHORIZED  
**BRANCH:** `phase-4/pipeline-designer`  
**ORDER:** 6 of 7  
**DEPENDENCY:** P4-005 checkpointed  
**DESIGN AUTHORITY:** `docs/PHASE_4_PLAN.md` and existing accepted ADRs/contracts.

## Objective and allowed scope
Mapping reference/compilation integration and tests; no historian or runtime writes.

## Required implementation
Resolve and verify approved mapping revision only, preserve mapping semantics, emit supported Telegraf processing config or fail closed with diagnostics; stale/revoked/missing references handled; never mutate approved mapper artifacts.

## Acceptance and evidence
Tests for supported/unsupported transformations, revision changes, no silent approximation or secret leak; `make check`.

## Safety / stop conditions
Do not edit `main`, open PRs, force-push, deploy, restart Telegraf, write historians, start synchronization, expose secrets, or self-expand the work package. If required contracts conflict or a privileged action is necessary, record blocker rather than invent permissions.

## CWAL checkpoint
Run real checks; report limitations; archive this task under `archive/active-work/`, commit and push evidence and advance both handoff pointers to the next authorized task automatically. Do not request intermediate approval.