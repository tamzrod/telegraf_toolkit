# P3-010-FU1 — Independent pre-PR audit follow-up

**State:** HUMAN-AUTHORIZED CURRENT. Execute upon `operation cwal` on `phase-3/canonical-mapper`. This is a correction to the P3-010 acceptance gate, not a new architectural phase. Keep the original P3-010 archive intact.

## Exact remaining findings

1. **Concurrency test weakness.** In `backend/internal/metadatastore/canonical_repository_test.go`, the canonical and mapping approval-versus-edit tests permit any final approved/draft state and discard both operation errors. Replace with deterministic, controlled interleaving tests that record both results and verify final persisted state/content, including the invariant that an edit after approval cannot modify any approved field. Demonstrate the tests would catch the former unguarded UPSERT race. Cover both repository types without relying on scheduler luck.
2. **Silent transaction-start fallback.** In `backend/internal/metadatastore/canonical_repository.go`, both `Put` methods currently treat `beginTx` errors as permission to fall back to a nontransactional execution path. Distinguish actual repositories that cannot begin a transaction because they are intentionally backed by an existing transaction from genuine BeginTx failures; propagate real failures, preserve service history/audit atomicity, and test failed-begin behavior. Do not silently weaken guarantees or introduce nested transactions.
3. **Missing P3-010 evidence.** Update `docs/PRE_PR_EVIDENCE_P3_PHASE.md` with new actual test names, exact commands and observed outcomes, implementation limits, and per-edition validation status (InfluxDB 2 OSS and 3 Core live tests must stay UNRUN unless actually executed). Append the final check results to this follow-up packet. Do not invent CI or test outcomes.

## Execution and exit
- Safely sync the same branch; preserve local work, original archives, and Git history; no destructive reset or force push.
- Make only narrow remediation and regression changes. Inspect guarded SQL and draft-to-approved behavior for regressions. Add fault-injection tests for transaction-begin failure where appropriate.
- Execute `cd backend && go test ./...`, `cd backend && go test -race ./...`, `make check`, relevant format/vet/policy/doc checks, and targeted tests. Record the exact outputs or accurate summaries. Any failure means TASK_BLOCKED with CURRENT retained.
- After all criteria pass, append results to this packet, archive it to `archive/active-work/`, clear both CURRENT pointers, commit, normal push and independently confirm remote SHA. Report PRE_PR_REAUDIT_READY for separate human-gated pre-PR audit.
- No PR creation/approval/merge, changes to main, production historian access/writes, sync or deployment, secrets exposure, or force push.
