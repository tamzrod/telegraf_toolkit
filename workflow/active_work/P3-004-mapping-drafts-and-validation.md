# P3-004 — Editable typed mapping drafts and validation

> **State: HUMAN-AUTHORIZED ACTIVE WORK — queued**. Branch `phase-3/canonical-mapper`. Execute only when designated CURRENT by both CWAL pointers. Authorized sequence P3-001 → P3-006. This packet does not approve any proposed ADR or unsafe capability expansion.

## Purpose
Implement browser/local draft mapping of measurement, field and tag names, tag values, filters and explicit unit transforms as applicable to confirmed schema types. Validate type compatibility, duplicate destination names, collisions, unit transform declarations, missing fields, stale scopes and unknown capabilities. No write query, deployment or sync.

## Acceptance criteria
No silent coercion; deterministic validation diagnostics; preserving draft while editing; controlled drag/drop with keyboard alternative; preview of mapping plan only unless separately authorized real read-only sample query with proven dialect.

## Verification
Unit/property/UI tests of mapping rules, injection strings, scope invalidation, lint/build and policy.

## Dependency and boundaries
- Dependencies: P3-001 through P3-003.
- Existing Phase 2 Registry is authoritative; registered connection IDs, canonical scope and administrator-bound credential references are used server-side.
- Preserve P2L-006/007, P2C-007/008 SSRF, TLS/pinned peer, credential RBAC, lifecycle fencing and redaction.
- Only synthetic, isolated InfluxDB 2/3 instances; no production access, historian writes, pipeline deployment, Telegraf or sync runtime operations.
- Unknown/unverified is never presented as supported. InfluxDB 1.x not established.
- Any new architecture requiring human approval: record the option and report TASK_BLOCKED; do not self-approve ADR decisions.

## CWAL lifecycle
Execute only CURRENT. On verified acceptance, archive with `git mv` into `archive/active-work/`, update `handoff.md` and Active Work README to designate the next task, commit/push and verify remote SHA. Continue without intermediate approval when within existing approved boundaries. Stop with TASK_BLOCKED when blocked. On P3-006 success clear both pointers and report WORK_PACKAGE_READY. **No agent PR creation, approval or merge.**
