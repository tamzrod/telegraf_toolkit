# P3-005 — Portable canonical schema and mapping profile persistence

> **State: HUMAN-AUTHORIZED CURRENT — architecture approved by human on 2026-10-28; execute when `operation cwal` is invoked**. Branch `phase-3/canonical-mapper`. Execute only when designated CURRENT by both CWAL pointers. Authorized sequence P3-001 → P3-006. This packet does not approve any proposed ADR or unsafe capability expansion.

## Approved architecture (human, 2026-10-28)

Keep canonical schemas and source-specific mapping profiles as separate versioned resources. Approved revisions are immutable. Support explicit draft and approval states, revision history, and rollback by creating a new revision (never rewrite history). Use the existing SQLite transactional metadata-store approach with role checks, concurrency and audit protections. JSON import/export is deterministic and versioned and must contain no secrets, tokens, database endpoint URLs or credential references; only safe scope/connection identities as already authorized. Preserve no historian writes/deployments. This approval resolves the prior P3-005 decision gate; do not ask again unless genuinely new architecture/security authority is needed. Acceptance and tests remain mandatory.

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


## P3-007 remediation (2026-10-28)
- Enforced approved revision immutability at SQLite repository boundaries (content changes to approved revisions rejected).
- Made put/approve operations atomic with revision history and audit records via existing transaction infrastructure; history/audit errors are not discarded.
- No changes to original acceptance requirements.
