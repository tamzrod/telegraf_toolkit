# P4-001-application-shell — Shared full-width application shell and left sidebar

**STATE:** AUTHORIZED  
**BRANCH:** `phase-4/pipeline-designer`  
**ORDER:** 1 of 7  
**DEPENDENCY:** Phase 3 merged baseline  
**DESIGN AUTHORITY:** `docs/PHASE_4_PLAN.md` and existing accepted ADRs/contracts.

## Objective and allowed scope
UI app/layout and related tests; no backend/domain changes.

## Required implementation
Anchor navigation to left viewport edge; remove global centered-width shell; preserve readable widths for individual forms; collapsible sidebar, mobile drawer, keyboard/focus access and accessible labels; ensure all existing routes work.

## Acceptance and evidence
Manual wide/narrow browser check documented; automated navigation/accessibility tests; `cd ui && npm run lint && npm run build && npm test`; no horizontal overflow or broken existing routes.

## Safety / stop conditions
Do not edit `main`, open PRs, force-push, deploy, restart Telegraf, write historians, start synchronization, expose secrets, or self-expand the work package. If required contracts conflict or a privileged action is necessary, record blocker rather than invent permissions.

## CWAL checkpoint
Run real checks; report limitations; archive this task under `archive/active-work/`, commit and push evidence and advance both handoff pointers to the next authorized task automatically. Do not request intermediate approval.