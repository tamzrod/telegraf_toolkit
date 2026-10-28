# P3-006 — Phase 3 integrated acceptance and Phase 4 handoff

> **State: HUMAN-AUTHORIZED QUEUED (automatically executable once CURRENT)**. Branch `phase-3/canonical-mapper`. Execute only when designated CURRENT by both CWAL pointers. Authorized sequence P3-001 → P3-006. This packet does not approve any proposed ADR or unsafe capability expansion.

## Purpose
Run complete isolated source/destination mapping journeys for both InfluxDB 2 OSS and 3 Core; verify read-only schema discovery, explicit selection, drafts, validation, version/save/import/export/rollback and permission denial. Reconcile roadmap honestly; document Phase 4 mapping compilation handoff and Phase 3 open limitations.

## Acceptance criteria
Publish per-edition evidence grid with actual results, no claims for InfluxDB 1.x or unproven query/write capabilities, no production historian writes, no Telegraf deployment. Archive all completed tasks, clear both CWAL pointers and report WORK_PACKAGE_READY only if all acceptance checks satisfied.

## Verification
make check, go test -race ./... on Linux, test harness against isolated 2/3, policy/secret scans, frontend tests; disclose any skipped visual/screen-reader tests.

## Dependency and boundaries
- Dependencies: P3-001 through P3-005.
- Existing Phase 2 Registry is authoritative; registered connection IDs, canonical scope and administrator-bound credential references are used server-side.
- Preserve P2L-006/007, P2C-007/008 SSRF, TLS/pinned peer, credential RBAC, lifecycle fencing and redaction.
- Only synthetic, isolated InfluxDB 2/3 instances; no production access, historian writes, pipeline deployment, Telegraf or sync runtime operations.
- Unknown/unverified is never presented as supported. InfluxDB 1.x not established.
- Any new architecture requiring human approval: record the option and report TASK_BLOCKED; do not self-approve ADR decisions.

## CWAL lifecycle
Execute only CURRENT. On verified acceptance, archive with `git mv` into `archive/active-work/`, update `handoff.md` and Active Work README to designate the next task, commit/push and verify remote SHA. Continue without intermediate approval when within existing approved boundaries. Stop with TASK_BLOCKED when blocked. On P3-006 success clear both pointers and report WORK_PACKAGE_READY. **No agent PR creation, approval or merge.**
