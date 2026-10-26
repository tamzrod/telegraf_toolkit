# P3-005 — Portable canonical schema and mapping profile persistence

> **State: HUMAN-AUTHORIZED ACTIVE WORK — queued**. Branch `phase-3/canonical-mapper`. Execute only when designated CURRENT by both CWAL pointers. Authorized sequence P3-001 → P3-006. This packet does not approve any proposed ADR or unsafe capability expansion.

## Purpose
Implement separate versioned canonical schemas and source-specific mapping profiles, explicit approval/version states and revision history; import/export safe JSON devoid of credentials and endpoints. Preserve atomic storage/audit/security; plan schema migrations as required.

## Acceptance criteria
Validate import, rollback, schema conflicts, stale references, schema compatibility and deterministic serialization; scope/connection IDs only (no secrets); role checks for write operations and no runtime/DB deployment.

## Verification
Go SQLite migration/concurrency/fault tests, UI API tests, export redaction checks, go race, make check.

## Dependency and boundaries
- Dependencies: P3-001 through P3-004.
- Existing Phase 2 Registry is authoritative; registered connection IDs, canonical scope and administrator-bound credential references are used server-side.
- Preserve P2L-006/007, P2C-007/008 SSRF, TLS/pinned peer, credential RBAC, lifecycle fencing and redaction.
- Only synthetic, isolated InfluxDB 2/3 instances; no production access, historian writes, pipeline deployment, Telegraf or sync runtime operations.
- Unknown/unverified is never presented as supported. InfluxDB 1.x not established.
- Any new architecture requiring human approval: record the option and report TASK_BLOCKED; do not self-approve ADR decisions.

## CWAL lifecycle
Execute only CURRENT. On verified acceptance, archive with `git mv` into `archive/active-work/`, update `handoff.md` and Active Work README to designate the next task, commit/push and verify remote SHA. Continue without intermediate approval when within existing approved boundaries. Stop with TASK_BLOCKED when blocked. On P3-006 success clear both pointers and report WORK_PACKAGE_READY. **No agent PR creation, approval or merge.**
