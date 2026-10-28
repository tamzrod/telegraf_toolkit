# Telegraf Toolkit — Operation CWAL handoff

## OPERATION CWAL — EXECUTION DIRECTIVE

**Branch:** `phase-3/canonical-mapper`
**CURRENT:** `workflow/active_work/P3-010-sql-atomic-approved-immutability.md`
**State:** HUMAN-APPROVED ACTIVE; execute on `operation cwal`.

P3-001–P3-009 remain archived. Independent review of P3-009 found that old approved-revision SELECT checks followed by unconditional SQLite UPSERTs did not atomically enforce immutability. The human approved P3-010 as a bounded final correction. On invocation, sync the local branch, execute P3-010's SQL-level guarded writes, draft-to-approved transition, deterministic concurrent regression tests and accurate evidence update. Run mandatory verification. If all pass, archive P3-010, clear both pointers, commit, push normally and confirm exact remote SHA; report PRE_PR_REAUDIT_READY. Otherwise report TASK_BLOCKED and retain CURRENT.

No PR action, force push, changes to main, production historian access/write, deployment or new architecture authorization.
