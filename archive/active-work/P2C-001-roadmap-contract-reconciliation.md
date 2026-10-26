# P2C-001 — Reconcile merged Registry roadmap, contracts and handoff

> **State: HUMAN-AUTHORIZED ACTIVE WORK — COMPLETE, CHECKPOINTED 2026-10-21** (archived to `archive/active-work/`). Roadmap, contract and architecture documents now describe the merged live Registry truthfully.

## Intent
Align docs/ROADMAP.md, Registry API and architecture handoff with merged PR #6 and P2L-001–007. Correct stale claims of non-implementation, unmerged branch and inconsistent checked Phase 2 exit items.

## Starting state
Base branch: `phase-2/registry-completion`, created from `main` following merged PR #6. The operational SQLite Registry, envelope secret store, authorized APIs, secure live probing and partial live UI are implemented, with P2L-006/007 security controls. ADR-0002/0003 Accepted, ADR-0004/0005 Proposed.

## Required result
Roadmap, docs and handoff truthfully distinguish merged live backend, partial UI, static token authentication, inferred capability results and unverified InfluxDB 1.x. Keep ADR-0004/0005 Proposed.

## Execution boundary
- Allowed: relevant `backend/`, `ui/`, `docs/` and isolated `testing/influxdb/` artifacts as needed for this task only.
- Dependencies: None.
- Explicit non-goals: No new app functionality or ADR acceptance.
- No unauthorized privileged or destructive actions; no production historian or credential access.

## Acceptance criteria
- [ ] Roadmap, docs and handoff truthfully distinguish merged live backend, partial UI, static token authentication, inferred capability results and unverified InfluxDB 1.x. Keep ADR-0004/0005 Proposed.
- [ ] Preserve server-side RBAC, administrator-only credential binding, P2L-006 pinned-peer/TLS/SSRF controls, and secrets/redaction invariants.
- [ ] Evidence corresponds to actual tested behavior; unsupported/unverified remains explicitly labelled.
- [ ] No new secrets, deployment or production historian writes.

## Verification evidence
Documentation links and status match main; empty CWAL queue recorded only after task execution and archive.

## Stop conditions
Stop and report `TASK_BLOCKED` for unsafe network/credential access, unresolved architecture, or required testing unavailable. No weakening tests or replacing working behavior with mocks.

## Git lifecycle
Human-promoted and authorized as part of the six-task queue. Execute only when designated CURRENT; advance one task at a time. Complete tests, `git mv` to `archive/active-work/`, update aligned CWAL pointers, commit/push and verify remote checkpoint. Human controls PR initiation/review/merge.

## Verification evidence (recorded 2026-10-21)

**State:** COMPLETE, CHECKPOINTED. The stale non-implementation, unmerged-branch
and ADR-status claims are corrected; the merged live backend, the partial UI,
static token authentication, edition-projected capability results and unverified
InfluxDB 1.x are now distinguished everywhere in the documentation set.

**Environment:** branch `phase-2/registry-completion` (base `main` at `0ba4827`,
the PR #6 merge); Go 1.27.3; Node.js 24.15.0 / npm 11.12.1; Windows development
host.

### What changed (18 files, documentation plus one harness script)

- `docs/ROADMAP.md` — status line, Phase 2 table row and integrated-status note
  now name PR #6 / P2L-001–P2L-007 as **merged into `main`**; the live-Registry
  checklist item is closed with its remaining gaps; the capability item states
  that outcomes are projected from the detected edition; Phase 2 exit criteria
  carry an explicit status; the stale "ADR-0002–0005 remain Proposed" note after
  PR #3 was corrected.
- `docs/decisions/phase-2-integration-handoff.md` — re-labelled a historical
  batch record; the "still blocked" table replaced by the current delivery
  status; limitations split into historical and current.
- `docs/decisions/phase-2-architecture-gate.md` — Gate 1 (ADR-0002) and Gate 2
  (ADR-0003) closed by human acceptance on 2026-10-21; ADR-0004/0005 remain
  Proposed; agent stop conditions updated (agents must not mark an ADR).
- `docs/contracts/README.md`, `docs/contracts/registry.md` — status lines, the
  "no contract is implemented" claim, the ADR candidate tables and the
  "ADR-0003 secret handling is not implemented" claim corrected; Registry-slice
  implementation state recorded; per-capability evidence and 1.x recorded open.
- `docs/DATABASE_REGISTRY.md` — status line plus an implementation-status table
  mapping every design area to implemented / not implemented.
- `docs/LIVE_PROBING.md` — the principle now states that the capability
  vocabulary is **projected from the detected edition** and that write
  capability is asserted by that projection, not proven by a write; harness
  smoke evidence and limitations updated to match.
- `docs/REGISTRY_API.md` — task/merge header; the owner-only (0600) token and
  key requirement and its effect on hosts without POSIX permission bits.
- `docs/ARCHITECTURE.md`, `docs/DEVELOPMENT.md`, `docs/DEVELOPER_SETUP.md`,
  `docs/FRONTEND_ARCHITECTURE.md`, `docs/UI_DESIGN.md`, `README.md`,
  `IDENTITY.md` — status lines now distinguish the merged backend, the partial
  UI, static token authentication and unverified InfluxDB 1.x.
- `testing/influxdb/scripts/toolkit-smoke.sh`, `testing/influxdb/README.md` —
  the third smoke check no longer claims "no live Registry in source"; it reports
  **NOT REGISTERED** because the registry data routes require an authentication
  token file. Pass/fail semantics are unchanged.
- `ICC/context/project.md` — refreshed for the merged Registry state.

### Commands and actual outcomes

- `git diff --stat` → 18 files changed, 258 insertions(+), 111 deletions(-);
  documentation and one harness script only, no runtime behavior change.
- `bash -n testing/influxdb/scripts/toolkit-smoke.sh` → exit 0.
- `scripts/policy-check.sh` → **PASS** (exit 0).
- Backend: `gofmt -l .` → clean; `go vet ./...` → pass; `go build ./...` → pass;
  `go test ./...` → **fails only** in `secretstore` and `registryservice` with
  `master key file permissions 0666 are too open; require 0600`. Root cause is
  the host, not the change: `secretstore.LoadKeyring` enforces POSIX 0600
  (`backend/internal/secretstore/keyring.go:52`) and Windows reports 0666 for
  every file, so the keyring fails to load and the dependent test panics on a
  nil store. The same commits are green on the Linux CI runner
  (ubuntu-latest) — `backend (format, vet, build, test)` succeeded on `0ba4827`.
- Frontend: `npm run lint` → pass; `npm run build` → pass; `npm test` → **15
  files / 152 tests passed**.
- Relative markdown link check across every changed `.md` → **0 missing links**.
- Local behavior verification (loopback only; no historian, no Docker, no
  privileged access): with the backend started **without** a token file,
  `GET /api/v1/health` → **200** with `scaffold=true` / `connected=false`,
  `POST /api/v1/health` → **405**, `GET /api/v1/registry/profiles` → **404**
  (data routes are not registered without an authentication token file) — the
  three outcomes the updated smoke script asserts. With a token file on this
  host the file is refused (`authentication token file permissions 0666 are too
  open; require 0600`), so the authenticated surface stays unregistered locally;
  it is covered by the Linux harness (`toolkit-live-registry.sh`).

### Limitations

- The isolated InfluxDB 2/3 harness could not run on this host: the Windows
  Docker Desktop daemon is not running, and the bash environment's Docker CLI
  has no Compose v2 plugin (`docker compose` → unknown command), so
  `testing/influxdb/scripts/up.sh` cannot start the stack. **No containers were
  started** and no historian was contacted. P2C-001 changes no runtime behavior,
  so no live-database evidence is required for it; the P2L-006/P2L-007 live
  evidence remains recorded in those archived tasks.
- The WSL Go toolchain in the bash environment is older than 1.27.3 and rejects
  the patch-pinned `go` directive in `backend/go.mod`, so the harness scripts
  cannot run there; the Windows Go 1.27.3 toolchain is used for local checks.
- `go test ./...` cannot pass on a Windows host (see above); Linux amd64 is the
  verified platform.
- Authentication remains static file-provisioned bearer tokens; capability
  outcomes remain edition-projected; **InfluxDB 1.x remains unverified**;
  ADR-0004 and ADR-0005 remain **Proposed**. No ADR status was changed by this
  task.
