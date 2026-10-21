# P2C-006 — Verify complete Registry workflows and prepare Phase 3 handoff

> **State: HUMAN-AUTHORIZED ACTIVE WORK.** Approved six-task sequence on `phase-2/registry-completion`. Current task is selected by `handoff.md` and `workflow/active_work/README.md`; execute sequentially P2C-001 → P2C-006 and never skip dependencies.

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

## Refined execution and acceptance contract (P2C-006)

- Perform a reproducible integrated test from fresh checkout against isolated synthetic InfluxDB 2 OSS and 3 Core: profile create/list/delete, privileged credential enrollment/binding, denied roles, rotation failure/recovery, probe evidence, UI source/destination selection, restart and backup/restore.
- Confirm security regression coverage from P2L-006/007 and P2C-002/005: DNS rebinding, peer pinning/TLS, SSRF, cross-profile credential theft, transactional integrity, redaction and revocation.
- Publish a **per-edition evidence table** with commands, pass/fail/unknown, limitations and provenance. InfluxDB 1.x remains **unverified** unless an independently authorized real test setup is available.
- Reconcile Phase 2 checklist without overclaiming: mark each item complete, partial or blocked based on real evidence; identify remaining token lifecycle, production TLS/packaging, dependency inspection, fine-grained authorization, write-capability uncertainty and UI accessibility limitations as applicable.
- Produce a bounded Phase 3 Canonical Mapper entry contract describing source/destination identity, scope, available schema/query evidence and explicit unknown states; no mapper implementation or data writes.
- Run make check, Go race, policy checks and available isolated harness scripts; document skipped or unavailable tests, never invent passing results.
- Archive P2C-006, set both CWAL pointers to queue-empty, commit/push and remote-verify; report WORK_PACKAGE_READY or TASK_BLOCKED. No PR actions.

## Stop conditions
Stop and report `TASK_BLOCKED` for unsafe network/credential access, unresolved architecture, or required testing unavailable. No weakening tests or replacing working behavior with mocks.

## Git lifecycle
Human-promoted and authorized as part of the six-task queue. Execute only when designated CURRENT; advance one task at a time. Complete tests, `git mv` to `archive/active-work/`, update aligned CWAL pointers, commit/push and verify remote checkpoint. Human controls PR initiation/review/merge.
