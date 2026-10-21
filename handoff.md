# Telegraf Toolkit — CWAL handoff

## CURRENT OPERATION
**Workflow:** Phase 2 Registry completion — pre-PR credential lifecycle security correction.
**Designated branch:** `phase-2/registry-completion`.
**Current human-authorized task:** none — the authorized queue is empty; `WORK_PACKAGE_READY`.
**Authorized sequence:** P2C-007 (complete).

## Human authorization
The P2C-007 workstream — the pre-PR audit corrections to the credential replacement and revocation lifecycle — was explicitly authorized for Operation CWAL execution. It is complete, verified, archived and confirmed on the remote branch. No further task is authorized; planning, ICC, handoff and archives never authorize new work.

## Current task
None. P2C-007 (pre-PR credential lifecycle security hardening) is complete, verified and archived. The queue is empty; the agent reports `WORK_PACKAGE_READY` and stops.

## Completed tasks
- P2C-001 (roadmap, contract and architecture reconciliation with merged PR #6
  and P2L-001–007) — verified and archived.
- P2C-002 (transactional Registry enrollment, bindings and audit; migration
  version 6 adds binding referential integrity) — verified and archived.
- P2C-003 (observation-based capability probes with a measured evidence matrix
  from the real isolated InfluxDB 2/3 containers) — verified and archived.
- P2C-004 (browser create/delete of unbound profiles with confirmation,
  validation and visible permission states) — verified and archived.
- P2C-005 (administrator-only write-only credential enrolment, stage → validate
  → swap → revoke replacement, safe revocation) — verified and archived.
- P2C-006 (integrated acceptance against the real isolated containers, Phase 2
  checklist reconciliation, Phase 3 entry contract) — verified and archived.
- P2C-007 (pre-PR credential lifecycle security hardening: replacement
  validation must prove protected authorization; per-attempt staging with a
  compare-and-swap swap; refuse-and-preserve revocation with the secret erased
  only after the metadata transition commits) — verified and archived.

## Restrictions and stop conditions
ADR-0002 and ADR-0003 are Accepted. ADR-0004/0005 remain Proposed. No production historians/credentials, uncontrolled probes, historian writes, Telegraf/Docker runtime management, or PR create/review/merge. Preserve P2L-006 pinned-peer TLS/SSRF controls and P2L-007 administrator-only credential attachment/binding at every step. If implementation requires unresolved architecture, privileged access, unsafe operation, or mandatory live tests unavailable, report `TASK_BLOCKED` with evidence and stop without skipping the task.

## Verification and final checkpoint
Follow each task's verification criteria, `make check`, applicable Go race tests and isolated InfluxDB 2/3 integration tests. Record actual executed outcomes, not assumptions. Use only synthetic test credentials and loopback test policy in isolated harness. After the final task, archive it, leave `workflow/active_work/` with README only, synchronize both pointers to empty, push and independently verify remote HEAD. Report `WORK_PACKAGE_READY` and stop. Human owns all PR actions.
