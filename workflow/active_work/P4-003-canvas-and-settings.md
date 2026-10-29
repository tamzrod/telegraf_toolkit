# P4-003-canvas-and-settings — React Flow canvas and contextual settings

**STATE:** AUTHORIZED  
**BRANCH:** `phase-4/pipeline-designer`  
**ORDER:** 3 of 7  
**DEPENDENCY:** P4-002 checkpointed  
**DESIGN AUTHORITY:** `docs/PHASE_4_PLAN.md` and existing accepted ADRs/contracts.

## Objective and allowed scope
UI pipeline feature and tests; no runtime APIs.

## Required implementation
Layout A canvas with selected-node right settings panel, searchable focused catalog, add/select/edit/delete/reorder, undo, keyboard alternatives, responsive panel; graph edges reflect actual configured semantics; presentation coordinates not runtime meaning.

## Acceptance and evidence
UI tests for graph edits/state preservation/accessibility and rejected invalid relationships; lint/build/test; verify browser wide/narrow layout.

## Safety / stop conditions
Do not edit `main`, open PRs, force-push, deploy, restart Telegraf, write historians, start synchronization, expose secrets, or self-expand the work package. If required contracts conflict or a privileged action is necessary, record blocker rather than invent permissions.

## CWAL checkpoint
Run real checks; report limitations; archive this task under `archive/active-work/`, commit and push evidence and advance both handoff pointers to the next authorized task automatically. Do not request intermediate approval.