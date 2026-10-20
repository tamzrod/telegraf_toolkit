# Telegraf Toolkit — CWAL handoff

## CURRENT OPERATION
**Workflow phase:** `development`; human-designated branch: `phase-2/database-registry`.
**Authorized package:** P2-001 through P2-006, explicitly human-approved as a **mock/interface-only** Phase 2 batch.

## Current Active Task
**P2-001:** `workflow/active_work/P2-001-architecture-and-contract-gate.md`.

## Authorized queue (strict sequence)
1. P2-001 — ADR readiness and Registry contracts (**current**)
2. P2-002 — mock Registry backend domain and in-memory interfaces
3. P2-003 — isolated per-connection capability fixtures
4. P2-004 — secret-reference boundaries and endpoint/TLS/SSRF validation tests
5. P2-005 — Registry UI mock workflows
6. P2-006 — mock quality gate, evidence and handoff

Human promotion is complete on this branch. Agent may proceed automatically between **confirmed verified branch checkpoints**, but never read future task bodies before the task is current. Update this handoff and Active Work README atomically with each verified task archive.

## Architectural constraints
ADR-0002 (metadata store) and ADR-0003 (secret store/key custody) remain **Proposed**. The human approved **only interfaces, in-memory/mock adapters, isolated capability fixtures, validation/security-boundary tests, and mock UI**. No real persistence, credential values/resolution, live InfluxDB connectivity, historian writes, Telegraf or Docker control. No ADR may be marked Accepted or silently implemented.

ADR-0004 and ADR-0005 also remain Proposed. Phase 0 PR #2 and Phase 1 PR #3 are integrated into main.

## PR authority and recovery
Only humans initiate, review, approve or merge PRs. After completing all six tasks, run final checks and report `WORK_PACKAGE_READY` without opening a PR. Before resuming, inspect remote branch, active/archived tasks, Git status and verification evidence; stop for genuine blockers, never self-promote or force-push.
