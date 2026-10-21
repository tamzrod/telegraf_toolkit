# Telegraf Toolkit — CWAL handoff

## CURRENT OPERATION
**Workflow:** Phase 2 Registry completion.
**Designated branch:** `phase-2/registry-completion`.
**Current human-authorized task:** `workflow/active_work/P2C-003-evidence-based-capability-probes.md`.
**Authorized sequence:** P2C-001 → P2C-002 → P2C-003 → P2C-004 → P2C-005 → P2C-006.

## Human authorization
The entire P2C-001 through P2C-006 workstream is explicitly authorized for sequential Operation CWAL execution. No intermediate approval is required after a task passes acceptance and its remote checkpoint is verified. Only the designated CURRENT task may be executed at a time; on completion archive that task with `git mv`, commit/push, verify the remote checkpoint, and advance CURRENT to the next authorized task in both handoff pointers.

## Current task
**P2C-003 — Evidence-based capability probes.** Read its active task and complete acceptance requirements.

## Completed tasks
- P2C-001 (roadmap, contract and architecture reconciliation with merged PR #6
  and P2L-001–007) — verified and archived.
- P2C-002 (transactional Registry enrollment, bindings and audit; migration
  version 6 adds binding referential integrity) — verified and archived.

## Restrictions and stop conditions
ADR-0002 and ADR-0003 are Accepted. ADR-0004/0005 remain Proposed. No production historians/credentials, uncontrolled probes, historian writes, Telegraf/Docker runtime management, or PR create/review/merge. Preserve P2L-006 pinned-peer TLS/SSRF controls and P2L-007 administrator-only credential attachment/binding at every step. If implementation requires unresolved architecture, privileged access, unsafe operation, or mandatory live tests unavailable, report `TASK_BLOCKED` with evidence and stop without skipping the task.

## Verification and final checkpoint
Follow each task's verification criteria, `make check`, applicable Go race tests and isolated InfluxDB 2/3 integration tests. Record actual executed outcomes, not assumptions. Use only synthetic test credentials and loopback test policy in isolated harness. After P2C-006, archive it, leave `workflow/active_work/` with README only, synchronize both pointers to empty, push and independently verify remote HEAD. Report `WORK_PACKAGE_READY` and stop. Human owns all PR actions.
