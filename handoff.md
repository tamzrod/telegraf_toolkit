# Telegraf Toolkit — CWAL handoff

## CURRENT OPERATION
**Workflow:** Phase 2 Registry completion — pre-PR credential lifecycle security correction.
**Designated branch:** `phase-2/registry-completion`.
**Current human-authorized task:** `workflow/active_work/P2C-007-pre-pr-credential-lifecycle-hardening.md`.
**Authorized queue:** P2C-007 only.

## Prior checkpoints
P2C-001–P2C-006 completed and archived at remote checkpoint `f7d17abe72652190bbddee20b09d38f43722e003`. Their reported tests do not constitute pre-PR clearance. The independent audit identified credential replacement and revocation vulnerabilities requiring P2C-007.

## Execution
Execute **only** P2C-007 per its security acceptance criteria, including protected authorization verification for replacement, independent concurrent staging/CAS, and revocation dependency integrity. Preserve P2L-006/007 protections. Test solely against synthetic isolated InfluxDB 2/3 endpoints. On verified completion, archive via `git mv`, set both pointers to queue empty, commit/push/verify remote SHA and report WORK_PACKAGE_READY. Otherwise report TASK_BLOCKED with evidence. No agent PR create/review/merge.

## Architecture and boundaries
ADR-0002/0003 Accepted; ADR-0004/0005 Proposed. No production endpoints/credentials, historian writes, Telegraf runtime control or unapproved architecture changes.
