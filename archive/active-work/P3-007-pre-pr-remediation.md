# P3-007 — Phase 3 pre-PR audit remediation

> **State: HUMAN-AUTHORIZED CURRENT.** Branch `phase-3/canonical-mapper`. Execute on `operation cwal` immediately after verifying the local branch is synced with remote. This is a new bounded remediation packet; P3-001–P3-006 remain completed and archived.

## Goal
Correct findings F-01–F-05 from the independent final pre-PR audit at commit `64bd122c9799b19b802545e9bc522e971ebdffa8`. Preserve the approved P3-005 architecture and Phase 3 functionality. No PR action.

## Work and acceptance
1. **F-01 approved revision immutability (high):** enforce at SQLite repository and service/API mutation boundaries that approved canonical schema and mapping-profile revisions cannot be rewritten. Allow only the authorized draft-to-approved transition; new edits and rollback use new revisions. Verify unauthorized state changes and concurrent bypass attempts are rejected.
2. **F-02 transaction integrity (high):** make create/update/approve/rollback and their revision-history and audit records atomic using the existing transaction infrastructure. Never discard history/audit errors. Add injected-write-failure and concurrency regression tests that prove no partial commit.
3. **F-03 acceptance evidence (high):** inspect local `/tmp/p3-006-evidence.md` if available, verify against actual test outputs, and commit sanitized per-edition InfluxDB 2 OSS / 3 Core acceptance evidence with commands, results, limitations and skips. Update historical P3-005/P3-006 archive packets by appending accurate implementation/evidence sections, without rewriting their original acceptance requirements. Do not invent unavailable test results.
4. **F-04 documentation (medium):** correct stale Phase 3 status in `docs/ROADMAP.md`, `docs/CANONICAL_MAPPING.md`, entry contract, README and appropriate handoff/ICC records. Keep InfluxDB 1.x unverified, and historian write, compilation, deployment and sync outside Phase 3.
5. **F-05 regression tests (medium):** add targeted repository/service/API tests for immutable approved content, revision collisions, draft approval, rollback races, history/audit failures, import validation/redaction and deterministic JSON, role denial and stale scope/connection references. Identify any already-covered cases precisely.

## Execution / verification
- Before edits, inspect current branch, remote SHA, worktree and previous audit; explicitly fetch `origin refs/heads/phase-3/canonical-mapper` (remote fetch configuration was stale previously). Fast-forward safely when possible. If diverged, preserve history and reconcile without force-push or deleting work.
- Implement narrowly and add tests. Run format/vet, `go test ./...`, `go test -race ./...` (supported Linux), `make check`, policy/secret checks, frontend tests/build/lint/typecheck, and documentation link checks. Run isolated per-edition tests where available. Record actual outcomes and any unexecuted browser/screen-reader checks; don't claim unavailable results.
- Verify changes and security invariants; append a remediation evidence section with the exact checks.
- Once all mandatory checks pass, `git mv` this packet to `archive/active-work/`, clear both CURRENT pointers, commit and push to the **same** branch using normal push, verify exact remote SHA with `git ls-remote`, and report `PRE_PR_REAUDIT_READY`.
- If safety gates, unresolvable divergence, missing mandatory verification or a genuinely new architecture decision block work, report `TASK_BLOCKED` with evidence and keep CURRENT.

## Prohibitions
No force-push, PR creation/approval/merge, changes to `main`, production historian access or writes, deployment, synchronization runtime, secret exposure, unrelated changes or unauthorized ADR decisions.
