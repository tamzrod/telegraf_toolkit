# P4-007-integrated-acceptance — Phase 4 end-to-end acceptance and handoff

**STATE:** AUTHORIZED  
**BRANCH:** `phase-4/pipeline-designer`  
**ORDER:** 7 of 7  
**DEPENDENCY:** P4-006 checkpointed  
**DESIGN AUTHORITY:** `docs/PHASE_4_PLAN.md` and existing accepted ADRs/contracts.

## Objective and allowed scope
Cross-layer tests, developer docs, evidence and closing queue; no production operations.

## Required implementation
Exercise full design→validate→preview→save→reload→manual transition→approval→export; wide/narrow layout, RBAC, revision races, secret safety and Phase 3 references. Update stale docs to accurately distinguish supported/unverified functionality. Archive P4-007 only after real checks.

## Acceptance and evidence
`make check`, backend `go test -race ./...`, appropriate UI and integration tests; explicit command outputs, skips, unsupported plugins and unrun live checks; package-level checkpoint readiness, human-only PR.

## Safety / stop conditions
Do not edit `main`, open PRs, force-push, deploy, restart Telegraf, write historians, start synchronization, expose secrets, or self-expand the work package. If required contracts conflict or a privileged action is necessary, record blocker rather than invent permissions.

## CWAL checkpoint
Run real checks; report limitations; archive this task under `archive/active-work/`, commit and push evidence and advance both handoff pointers to the next authorized task automatically. Do not request intermediate approval.