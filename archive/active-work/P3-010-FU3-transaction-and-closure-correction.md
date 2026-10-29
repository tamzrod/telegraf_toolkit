# P3-010-FU3 — Transaction and CWAL closure correction

**HUMAN-AUTHORIZED CURRENT** on `phase-3/canonical-mapper`. This corrects the false PRE_PR_REAUDIT_READY at `5f229060`; resume FU2 then FU1 only if still needed. Do not redo passing fixes.

## Required outcome
1. **Restore transaction correctness.** Both repository `Put` methods currently treat lack of `BeginTx` on an externally owned `*sql.Tx` as fatal. Distinguish this legitimate path from a genuine `BeginTx` failure; preserve outer rollback/atomic history and never nest a transaction. Add focused tests: external `*sql.Tx` Put + rollback; injected BeginTx error propagates with no write. Inspect both canonical and mapping.
2. **Prove safety, don't merely report PASS.** Verify approved revisions reject later edits with exact persisted content/hash/notes; ensure race tests themselves have no shared-write races. Run `cd backend && go test ./...`, `cd backend && go test -race ./...`, focused repeated tests and `make check`; document real results and UNRUN live checks. Any failure keeps task active.
3. **Repair one authoritative CWAL state.** The branch currently has ACTIVE uppercase `workflow/active_work/README.md` and `handoff.md`, original active FU1/FU2 packets, plus contradictory lowercase `workflow/active_work/readme.md` and copy-like archive markers. Reconcile safely: preserve actual task history, eliminate misleading duplicate metadata, archive real completed packets through the canonical `archive/active-work/` path prescribed by `operation cwal.md`, and clear CURRENT in BOTH authoritative pointers only after every active task meets acceptance. Do not treat copy-only archives as closure.
4. **Git checkpoint.** Commit and push normally on the designated attached branch; fetch and confirm exact remote SHA and file tree. Preserve prior authorized work; no force push or destructive reset.

If FU3 passes, archive it and evaluate FU2/FU1 against their original criteria. If already satisfied, close them correctly in the same run; otherwise execute remaining in-scope work. Report PRE_PR_REAUDIT_READY only after all gates and consistent remote closure. On failure report TASK_BLOCKED, retaining CURRENT. No PR, main changes, historian writes or deployment.

## Verification appended 2026-10-29

Fast-forwarded `5f229060..3056e611` from `origin/phase-3/canonical-mapper` before edits. Restored uncommitted local deletions of the FU1 and FU2 packets so the authorized queue stayed intact.

Both `Put` methods now join an externally owned `*sql.Tx` without `BeginTx`, `Commit` or `Rollback`. A genuine `BeginTx` error returns before any write. Focused tests cover external rollback, injected begin failure, outer history rollback, and exact content/hash/notes after a rejected post-approval edit, for canonical schemas and mapping profiles.

Commands, all exit 0: `cd backend && go test ./...`; `cd backend && go test -race ./...`; `cd backend && go test -race -count=10 -timeout 300s ./internal/metadatastore/ -run 'TestCanonical|TestMapping|TestApproved|TestConcurrent|TestSchema'` (`ok`, 17.742s); `make check` (UI 270 passed, 1 skipped; policy-check PASS). The skipped UI file is the live schema smoke, unset `TTK_LIVE_*`. Live InfluxDB 2 OSS and 3 Core UNRUN. InfluxDB 1.x unverified.

`-race` success is not the SQLite immutability proof. The external-transaction and exact-field tests are. Details are in `docs/PRE_PR_EVIDENCE_P3_PHASE.md`.

FU2 and FU1 criteria that this fix covers are closed in the same run. Copy-only markers under `workflow/archive/` and `workflow/active_work/readme.md` are removed. Canonical archive path is `archive/active-work/`. `d68c3c9443ae82c88cf54ccdf3320b4e875b13cc` still contains the non-transactional begin fallback and was not restored. No force-push.
