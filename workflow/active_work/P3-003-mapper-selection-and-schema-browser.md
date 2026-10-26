# P3-003 — Live dual-profile selection and typed schema browser

> **State: HUMAN-AUTHORIZED ACTIVE WORK — queued**. Branch `phase-3/canonical-mapper`. Execute only when designated CURRENT by both CWAL pointers. Authorized sequence P3-001 → P3-006. This packet does not approve any proposed ADR or unsafe capability expansion.

## Purpose
Create a genuine Live-mode Canonical Mapper UI for explicit distinct source and destination Registry profiles with scope. Show their measurement, field-key and tag-key metadata in separate typed groups backed by P3-002. Support loading, empty, permission, unknown, unsupported and stale states without mock fallback. Preserve labeled Mock mode.

## Acceptance criteria
Ensure source/destination distinct; invalidate cache and dependent drafts whenever selected profile/scope/identity changes; never expose secret/ref values beyond safe identity; no implicit writes; keyboard navigation, accessible lists/errors, race-safe request cancel.

## Verification
Vitest/typed API tests, lint/typecheck/build, authenticated real-backend smoke and manual a11y evidence or explicit gap.

## Dependency and boundaries
- Dependencies: P3-001 through P3-002.
- Existing Phase 2 Registry is authoritative; registered connection IDs, canonical scope and administrator-bound credential references are used server-side.
- Preserve P2L-006/007, P2C-007/008 SSRF, TLS/pinned peer, credential RBAC, lifecycle fencing and redaction.
- Only synthetic, isolated InfluxDB 2/3 instances; no production access, historian writes, pipeline deployment, Telegraf or sync runtime operations.
- Unknown/unverified is never presented as supported. InfluxDB 1.x not established.
- Any new architecture requiring human approval: record the option and report TASK_BLOCKED; do not self-approve ADR decisions.

## CWAL lifecycle
Execute only CURRENT. On verified acceptance, archive with `git mv` into `archive/active-work/`, update `handoff.md` and Active Work README to designate the next task, commit/push and verify remote SHA. Continue without intermediate approval when within existing approved boundaries. Stop with TASK_BLOCKED when blocked. On P3-006 success clear both pointers and report WORK_PACKAGE_READY. **No agent PR creation, approval or merge.**
