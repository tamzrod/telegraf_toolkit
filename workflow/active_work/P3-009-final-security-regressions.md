# P3-009 — Final security regression corrections

State: HUMAN-APPROVED CURRENT, 2026-10-28. Branch: phase-3/canonical-mapper.

Implement atomic SQLite enforcement of immutable approved canonical schema and mapping profile revisions, covering every protected column including notes and blocking concurrent bypasses. Preserve authorized draft-to-approved transitions. Add committed repository/service/API regression tests for overwrites, concurrency, rollback history/audit errors, RBAC and redaction. Update docs/PRE_PR_EVIDENCE_P3_PHASE.md with actual test names/results and explicitly distinguish live InfluxDB tests from mock/unit tests. Do not invent evidence.

Run Go tests/race, make check, policy and documentation checks. Record results. On passing acceptance, archive this packet, clear both CWAL pointers, commit, push normally and verify remote SHA; report PRE_PR_REAUDIT_READY. Stop with TASK_BLOCKED for genuine blockers.

No PR operations, force push, historian writes, deployment or main branch changes.
