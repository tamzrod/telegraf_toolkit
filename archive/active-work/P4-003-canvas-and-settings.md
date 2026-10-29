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

## Verification appended 2026-10-29

Added a local Layout A designer at `/pipelines`. The center is a React Flow canvas (`@xyflow/react` 12.12.0). The selected instance's settings sit in a right-hand column at the `lg` breakpoint and stack under the canvas below it. Catalog search is limited to the verified Telegraf 1.40.1 subset. Add, select, edit, delete, processor reorder, and undo are buttons; Ctrl/Cmd+Z undoes when focus is outside a field. Nudge buttons move presentation coordinates. Drawn connections are refused. Listed and drawn edges come from processor order, aggregator fan-out, and output delivery. Two inputs are not linked. Aggregators are not chained. Skipping both processor passes is refused. Credential text and URL userinfo are rejected and not stored.

`make check` exited 0.
- Backend gofmt check, `go vet`, `go build`, and `go test ./...` passed.
- UI `npm ci`, lint, and build passed. Vite reported one chunk above 500 kB after React Flow was bundled; the build still exited 0.
- UI tests: 27 files passed, 1 skipped; 290 tests passed, 1 skipped. The skip is `src/api/schema.live.integration.test.ts` because `TTK_LIVE_*` was unset.
- `scripts/policy-check.sh` printed `policy-check: PASS`.

Headless Chrome against `vite preview` at `http://127.0.0.1:4173/pipelines`:
- 1440×900: the settings column sat to the right of the canvas column (aside left 1081, canvas column right 1065, aside width 320). Collapse sidebar was visible and Menu was `display: none`. Document `scrollWidth` equalled `clientWidth` (1425). One `h1`.
- The same session clicked Add input cpu, set Alias to `edge-host`, added mem, reselected cpu, and read the alias back. It added two rename processors, observed `then: rename-1 to rename-2`, moved rename-2 earlier, and observed the reversed edge. `cpu-1 to mem-1` was absent. Nudge rename-1 right showed position `64, 150` and left the reversed processor edge in place. Pasting `user:placeholder-literal@host` into the influxdb_v2 credential reference left the field empty, showed the rejection, and did not put that literal in the document. No Deploy control was present. The flow surface measured about 815×478.
- 390×844 on that same document: the settings column stacked below the canvas column (aside top 1527, canvas column bottom 1511), both about 342 wide. Menu was visible. The alias still read `edge-host`. Document `scrollWidth` equalled `clientWidth` (390).

Limitations: the graph is browser-local and is not saved, rendered to TOML, or deployed. A pointer drag between handles was not performed; the canvas rejects connections and the page states that drawn connections are rejected. Dragging a node updates coordinates without an undo snapshot; keyboard nudge is undoable. The catalog remains the P4-002 subset. Live InfluxDB was not contacted. No historian write or runtime restart was performed.