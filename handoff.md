# Telegraf Toolkit — CWAL handoff

## CURRENT OPERATION
**Workflow:** Phase 2 Registry completion, independent pre-PR lifecycle race hardening.
**Designated branch:** `phase-2/registry-completion`.
**Current human-authorized task:** `workflow/active_work/P2C-008-pre-pr-lifecycle-race.md`.
**Authorized queue:** P2C-008 only.

## Checkpoints
P2C-001–P2C-007 archived, completed and pushed at `e6ce84d44e6f1f4672b86ca311ca68e3b54e88e8`. The latest review confirmed P2C-007's protected-credential verification, ciphertext CAS and transactional revoke checks, but identified a replacement-versus-revocation race across the network-validation window.

## Execution and stop
Execute the sole active P2C-008 task, preserve all P2L-006/007 and P2C-007 protections, verify deterministic concurrency and failure cases, run supported Linux race and isolated synthetic InfluxDB 2/3 lifecycle checks. Archive via `git mv`, clear both pointers, commit/push/verify remote checkpoint, report WORK_PACKAGE_READY. Report TASK_BLOCKED for unmet security conditions. Do not create a PR or access production.

ADR-0002/0003 Accepted; ADR-0004/0005 Proposed. No historian writes, runtime control or unapproved privilege changes.
