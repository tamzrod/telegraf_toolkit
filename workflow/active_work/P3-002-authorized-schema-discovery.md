# P3-002 — Authorized source/destination schema discovery backend

> **State: HUMAN-AUTHORIZED ACTIVE WORK — queued**. Branch `phase-3/canonical-mapper`. Execute only when designated CURRENT by both CWAL pointers. Authorized sequence P3-001 → P3-006. This packet does not approve any proposed ADR or unsafe capability expansion.

## Purpose
Implement the approved safe, bounded, read-only backend schema discovery path using stored Registry profile identity and scope; reuse credential approval verification, role permissions and pinned peer/TLS/SSRF controls. For InfluxDB 2/3 discover actual measurements, field keys and tag keys distinctly where the approved dialect permits, with precise unsupported/unknown results. Query POST allowed only if P3-001 explicitly authorizes its read-only threat-model revision; no writes even as a probe.

## Acceptance criteria
Profile-only endpoint (no arbitrary URL/credential from browser), strong input/query injection defenses, bounded pagination/results/timeouts/request budget, cancellation, error redaction, role denial, scope change invalidation. Each returned observation must cite check/method/edition; unknown stays unknown; InfluxDB 1.x remains unverified.

## Verification
Go auth/SSRF/query-negative tests + isolated synthetic InfluxDB 2/3 end-to-end metadata checks, go race, make check.

## Dependency and boundaries
- Dependencies: P3-001 through P3-001.
- Existing Phase 2 Registry is authoritative; registered connection IDs, canonical scope and administrator-bound credential references are used server-side.
- Preserve P2L-006/007, P2C-007/008 SSRF, TLS/pinned peer, credential RBAC, lifecycle fencing and redaction.
- Only synthetic, isolated InfluxDB 2/3 instances; no production access, historian writes, pipeline deployment, Telegraf or sync runtime operations.
- Unknown/unverified is never presented as supported. InfluxDB 1.x not established.
- Any new architecture requiring human approval: record the option and report TASK_BLOCKED; do not self-approve ADR decisions.

## CWAL lifecycle
Execute only CURRENT. On verified acceptance, archive with `git mv` into `archive/active-work/`, update `handoff.md` and Active Work README to designate the next task, commit/push and verify remote SHA. Continue without intermediate approval when within existing approved boundaries. Stop with TASK_BLOCKED when blocked. On P3-006 success clear both pointers and report WORK_PACKAGE_READY. **No agent PR creation, approval or merge.**
