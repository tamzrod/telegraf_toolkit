# P2C-006 — Verify complete Registry workflows and prepare Phase 3 handoff

> **State: PLANNING ONLY — not authorized.** Promotion to `workflow/active_work/` requires explicit human authorization and updated CWAL handoff.

## Intent
Run full real isolated InfluxDB 2/3 acceptance of profile lifecycle, approved credential binding, read-only probing, UI workflows, restart/restore and security controls. Review Phase 2 exit checklist and specify Phase 3 source/destination/schema discovery contract.

## Starting state
Base branch: `phase-2/registry-completion`, created from `main` following merged PR #6. The operational SQLite Registry, envelope secret store, authorized APIs, secure live probing and partial live UI are implemented, with P2L-006/007 security controls. ADR-0002/0003 Accepted, ADR-0004/0005 Proposed.

## Required result
Publish evidence by exercised edition, pass/fail/unknown including InfluxDB 1.x unverified. Record remaining follow-up separately and report whether Phase 2 exit is fully satisfied; handoff clean and tasks archived.

## Execution boundary
- Allowed: relevant `backend/`, `ui/`, `docs/` and isolated `testing/influxdb/` artifacts as needed for this task only.
- Dependencies: P2C-001, P2C-002, P2C-003, P2C-004, P2C-005.
- Explicit non-goals: No Phase 3 mapper implementation, production deployment, historian repair, or claim of support not verified.
- No unauthorized privileged or destructive actions; no production historian or credential access.

## Acceptance criteria
- [ ] Publish evidence by exercised edition, pass/fail/unknown including InfluxDB 1.x unverified. Record remaining follow-up separately and report whether Phase 2 exit is fully satisfied; handoff clean and tasks archived.
- [ ] Preserve server-side RBAC, administrator-only credential binding, P2L-006 pinned-peer/TLS/SSRF controls, and secrets/redaction invariants.
- [ ] Evidence corresponds to actual tested behavior; unsupported/unverified remains explicitly labelled.
- [ ] No new secrets, deployment or production historian writes.

## Verification evidence
make check, go test -race, e2e harness, static secret/policy checks and documented manual browser/a11y checks or explicitly outstanding.

## Stop conditions
Stop and report `TASK_BLOCKED` for unsafe network/credential access, unresolved architecture, or required testing unavailable. No weakening tests or replacing working behavior with mocks.

## Git lifecycle
Execute only when human-promoted. Complete tests, `git mv` to `archive/active-work/`, update aligned CWAL pointers, commit/push and verify remote checkpoint. Human controls PR initiation/review/merge.
