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

## Verification appended 2026-10-29

A pipeline has one open draft. Each revision is either graph or manual, never both. Switching mode stores the previous document as a snapshot and requires the digest from a difference preview. Edited manual TOML that does not round-trip through the preview dialect is refused and the manual revision stays. Restoring the snapshot puts the graph document back and keeps the manual document as the new snapshot. Approval sets `state=approved` without changing the document. A later save is refused. Rollback copies an approved revision into a new draft and leaves the approved row unchanged. `pipeline.read` is held by every role. `pipeline.write` is configurator and administrator. `pipeline.approve` is administrator only. Audit rows for create, save, transition, restore, approve, and open-draft are written in the same transaction as the revision change. An invalid save writes neither the document nor an audit row.

`make check` exited 0.
- Backend gofmt check, `go vet`, `go build`, and `go test ./...` passed.
- `go test -race` on `pipelineservice`, `pipelineapi`, and `metadatastore` passed. Two concurrent saves of the same generation produced one winner, one conflict, and one `pipeline.save` audit.
- UI `npm ci` added 272 packages and reported 0 vulnerabilities. Lint and build passed. Vite reported one chunk above 500 kB. The build still exited 0.
- UI tests: 28 files passed, 1 skipped; 297 tests passed, 1 skipped. The skip is `src/api/schema.live.integration.test.ts` because `TTK_LIVE_*` was unset.
- `scripts/policy-check.sh` printed `policy-check: PASS (limited regex checks only)`.

Headless Chrome against `vite preview` at `http://127.0.0.1:4173/pipelines`:
- 1440×900: Save draft and Approve revision were disabled. The page said nothing is deployed. No Deploy control. Settings aside left was 1081 and the canvas column right was 1065. Document `scrollWidth` equalled `clientWidth` (1425). One `h1`.
- Review switch to manual showed a difference review containing `[agent]`. Confirm opened an editable Manual TOML field and removed the canvas. Appending a comment and reviewing a switch back to graph showed the refusal that edited manual TOML cannot be converted back. Review restore snapshot showed Proposed, and confirm brought the graph preview back.
- 390×844 on the restored graph: the settings column stacked below the canvas (aside top 2145, canvas column bottom 2129). Menu was `display: block`. Document `scrollWidth` equalled `clientWidth` (390). One `h1`.

Limitations: the browser refuses a manual-to-graph conversion unless the text still matches the snapshotted preview. The server also accepts manual TOML that round-trips through the preview dialect. The canvas still does not edit filters or global tags. Save sends the canvas document, or the snapshot graph plus the manual body, and does not reload a server document that the canvas cannot represent. Live InfluxDB was not contacted. No historian write, runtime restart, or deploy was performed. `ICC/context/project.md` was not rewritten.