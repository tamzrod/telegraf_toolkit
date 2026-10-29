# P3-010-FU2 — Race-test and checkpoint correction

**HUMAN-AUTHORIZED CURRENT** on `phase-3/canonical-mapper`. Next: existing P3-010-FU1; resume it automatically after FU2 checkpoint.

## Objective
Correct the false `WORK_PACKAGE_READY` claim. A detected Go data race is a failing gate, **not** an intentional concurrency-test pass. Resolve the immediate race and ensure the fix is actually pushed to the designated branch; do not repeat completed work.

## Acceptance
1. Safely fetch and reconcile local/detached work against the actual remote branch, preserving previously authorized CWAL-001 changes and any valid FU1 fixes. No lost work, force push, or destructive reset. If conflict cannot be safely reconciled, report `TASK_BLOCKED`.
2. In `backend/internal/metadatastore/canonical_repository_test.go`, eliminate concurrent writes to the same `results[idx*2]` slot in `TestSchemaConcurrentRaceWithFinalStateVerification`; use distinct slots and assert operation outcomes plus exact approved revision content. Inspect matching mapping tests for weak or false-positive assertions. An intentionally concurrent test must still be race-free itself.
3. Confirm both repository `Put` paths preserve externally owned `*sql.Tx` operations and propagate genuine `BeginTx` failures. Add or retain regression coverage; do not revive the transaction regression.
4. Run `cd backend && go test ./...`, `cd backend && go test -race ./...`, repeated focused concurrency tests, and `make check`. Preserve unrelated scratch files safely outside package build paths. Fix any actual failures; never omit known failing tests.
5. Replace inaccurate PASS claims in `docs/PRE_PR_EVIDENCE_P3_PHASE.md` with real commands/results; live InfluxDB checks remain UNRUN unless executed.

## Checkpoint
Only after all five criteria are verified: archive FU2, set CURRENT to existing FU1 in both pointers, commit, push normally, confirm remote SHA, then **continue FU1 immediately** to complete its remaining acceptance and archive. Otherwise leave FU2 CURRENT and report `TASK_BLOCKED`. Human alone controls PRs. No main modification, force push, historian writes, or deployment.

## Verification appended 2026-10-29

Evaluated after FU3 on `phase-3/canonical-mapper`. Criteria already met, so this packet is closed in the same run rather than reimplemented.

1. Fetched `refs/heads/phase-3/canonical-mapper` and fast-forwarded `5f229060..3056e611`. Restored local deletions of this packet and FU1. No force-push or reset.
2. `TestSchemaConcurrentRaceWithFinalStateVerification` writes `results[idx*3]`, `results[idx*3+1]` and `results[idx*3+2]`. Repeated `go test -race -count=10` on the metadatastore canonical tests reported no data race. Final content, hash and notes stay on the original triple.
3. Both `Put` paths join an external `*sql.Tx` and return an injected `BeginTx` error with no write. Regression tests: `TestCanonicalPutOnExternalTxRollsBack`, `TestMappingPutOnExternalTxRollsBack`, `TestCanonicalBeginTxErrorWritesNothing`, `TestMappingBeginTxErrorWritesNothing`.
4. `cd backend && go test ./...`, `cd backend && go test -race ./...`, the repeated command above (`ok`, 17.742s), and `make check` all exited 0.
5. `docs/PRE_PR_EVIDENCE_P3_PHASE.md` replaces the earlier unverified PASS table. Live InfluxDB 2 OSS and 3 Core remain UNRUN. InfluxDB 1.x unverified.
