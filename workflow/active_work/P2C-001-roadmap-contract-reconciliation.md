# P2C-001 — Reconcile merged Registry roadmap, contracts and handoff

> **State: HUMAN-AUTHORIZED ACTIVE WORK.** Approved six-task sequence on `phase-2/registry-completion`. Current task is selected by `handoff.md` and `workflow/active_work/README.md`; execute sequentially P2C-001 → P2C-006 and never skip dependencies.

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
