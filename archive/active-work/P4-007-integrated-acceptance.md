# P4-007-integrated-acceptance — Phase 4 end-to-end acceptance and handoff

**STATE:** ACCEPTED and ARCHIVED (2026-10-29)
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

---

## Verification and checkpoint (2026-10-29)

**Outcome:** PASS. The task was archived, committed and pushed after the checks
below. No PR was created and `main` was not modified.

### Delivered

- `backend/internal/pipelineapi/acceptance_test.go` — one cross-layer HTTP test
  (`TestIntegratedAcceptanceHTTPLifecycle`) using the same API, service and
  `mappingcompile` wiring as `cmd/api` over a temporary SQLite store. It covers
  design, validation, preview, save, reload, stale generation, graph→manual
  transition, stale digest, approval, rollback draft, export, RBAC, audit,
  credential-material rejection, unsupported-mapping diagnostics and approved
  mapping immutability. Nothing is deployed.
- `ui/src/features/pipelines/PipelinesPage.test.tsx` — save → approve flow;
  asserts the `expected_generation` body, the approval confirmation and the
  absence of a deploy control.
- `backend/internal/registryapi/credential_lifecycle_test.go` — test-only
  determinism repair for the pre-existing `TestConcurrentReplacementsHaveOneWinner`
  flake (see below). Production code was not changed.
- Documentation refreshed for the real delivery state: `README.md`,
  `docs/ROADMAP.md`, `docs/DEVELOPER_SETUP.md`, `docs/contracts/README.md`,
  `docs/contracts/pipeline.md`, `docs/contracts/canonical-mapper.md` and
  `ICC/context/project.md`.

### Real command results

| Command | Actual outcome |
| --- | --- |
| `make check` | exit 0; backend format/vet/build/test; UI lint/build; Vitest **28 files passed / 1 skipped**, **298 tests passed / 1 skipped**; `policy-check: PASS (limited regex checks only)` |
| `cd backend && go test -race -count=1 ./...` (run 1) | exit 0; all packages `ok` |
| `cd backend && go test -race -count=1 ./...` (run 2) | exit 0; all packages `ok` |
| Targeted race test before repair | **6/10** runs failed with `racing replacements = 2 wins / 0 conflicts, want 1/1` |
| Targeted race test after repair | **50/50** runs passed (`-race -count=50`) |
| `PipelinesPage.test.tsx` | 12/12 tests passed |

Local toolchain: Go 1.26.8, Node v24.16.0, npm 11.13.0. CI pins Go 1.27.3, so CI
must confirm the repaired race test on the pinned toolchain.

### Race-suite flake and repair

`TestConcurrentReplacementsHaveOneWinner` failed intermittently and reproduced
on untouched `origin/main`; it is unrelated to Phase 4 behavior. Root cause was
test synchronization, not the credential compare-and-swap: the two HTTP
replacements sometimes ran sequentially, so the second observed the first
replacement's committed envelope and both legitimately succeeded, exercising no
conflict. The repair holds each candidate validation in the fake InfluxDB
listing server until both candidates have arrived, so both observations precede
either commit. Assertions are unchanged; no production code changed.

### Real-browser layout evidence

Headless Chrome against the built SPA at 1440×900 and 390×844:

- **Wide:** aside to the right of the canvas (aside left 1081, canvas right
  1065, aside width 320); navigation menu hidden.
- **Narrow:** panels stacked full width (both 24–366); menu visible.
- No horizontal overflow, one `h1`, read-only preview and no deploy control at
  either size.

### Limitations and not-run checks

- No live InfluxDB container or schema-discovery smoke for this task; the
  acceptance test uses approved mapping rows in a temporary store.
- The pipeline page does not edit a mapping pin and does not display the
  server-compiled export. Unit scaling and tag filters remain unsupported.
- InfluxDB 1.x remains unverified.
- The browser check measured layout and control presence only; no screen-reader
  session.
- No deployment, Telegraf restart, historian write, secret exposure, PR, merge,
  force push or `main` change occurred.