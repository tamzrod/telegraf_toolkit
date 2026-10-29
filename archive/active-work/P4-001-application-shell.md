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

## Verification appended 2026-10-29

The shell is full width. The primary nav is the left column, anchored to the viewport edge, sticky at desktop height. Wide screens use Collapse sidebar / Expand sidebar (`aria-expanded`, `aria-controls="app-sidebar"`). Narrow screens keep the Menu button, a dismiss control, and Escape to close the drawer and return focus to Menu. Text fields, selects and textareas inside main are capped at 40rem. Checkboxes and radios are not capped.

Commands, all exit 0, from `ui/`:
- `npm run lint`
- `npm run build` (`tsc -b && vite build`)
- `npm test`: Test Files 25 passed, 1 skipped (26); Tests 271 passed, 1 skipped (272). The skip is `src/api/schema.live.integration.test.ts` because `TTK_LIVE_*` was unset.

Headless Chrome screenshots of the production preview at `127.0.0.1:4173`:
- 1440×900: Overview, Databases, Canonical Mapper, and an unknown route. Sidebar sits on the left edge; content uses the remaining width; unknown route shows Page not found.
- 390×844: Overview and Pipelines. Sidebar is hidden, Menu is visible, title stays on one line, and the pages stack without a horizontal page overflow in the captured view.

The preview had no API process, so Overview and Databases showed the existing unreachable-backend state. That is not a live registry or historian connection. No deployment, restart, or historian write was performed.