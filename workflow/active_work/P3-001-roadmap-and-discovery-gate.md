# P3-001 — Roadmap reconciliation and schema-discovery architecture gate

> **State: HUMAN-AUTHORIZED ACTIVE WORK — queued**. Branch `phase-3/canonical-mapper`. Execute only when designated CURRENT by both CWAL pointers. Authorized sequence P3-001 → P3-006. This packet does not approve any proposed ADR or unsafe capability expansion.

## Purpose
Reconcile docs/ROADMAP.md, README/ICC and Phase 3 entry contract with merged PR #7 (merge bba5a676) and P2C-001–008, without claiming Phase 2 exit fully met. Decide at an explicit documented gate between a bounded backend read-only metadata API using safe nonmutating query methods versus a narrower metadata-only GET path; clarify HTTP POST query is read-only semantically but changes P2C-003's GET-only probe boundary. Record authority, role, threat model, per-edition scope, limits, unknown states, tests and decision outcome; do not silently accept ADR-0004/0005 or authorize historian mutation.

## Acceptance criteria
Roadmap and product status accurate; all linked docs updated; documented decision with security implications and approval state. If a new protected POST-based read path or other materially new security architecture requires human decision, report TASK_BLOCKED at that gate rather than treating a proposal as accepted.

## Verification
Docs validation, link check, policy, backend/frontend quick baseline; record explicit evidence.

## Dependency and boundaries
- Dependencies: merged PR #7 and Phase 3 entry contract.
- Existing Phase 2 Registry is authoritative; registered connection IDs, canonical scope and administrator-bound credential references are used server-side.
- Preserve P2L-006/007, P2C-007/008 SSRF, TLS/pinned peer, credential RBAC, lifecycle fencing and redaction.
- Only synthetic, isolated InfluxDB 2/3 instances; no production access, historian writes, pipeline deployment, Telegraf or sync runtime operations.
- Unknown/unverified is never presented as supported. InfluxDB 1.x not established.
- Any new architecture requiring human approval: record the option and report TASK_BLOCKED; do not self-approve ADR decisions.

## CWAL lifecycle
Execute only CURRENT. On verified acceptance, archive with `git mv` into `archive/active-work/`, update `handoff.md` and Active Work README to designate the next task, commit/push and verify remote SHA. Continue without intermediate approval when within existing approved boundaries. Stop with TASK_BLOCKED when blocked. On P3-006 success clear both pointers and report WORK_PACKAGE_READY. **No agent PR creation, approval or merge.**
