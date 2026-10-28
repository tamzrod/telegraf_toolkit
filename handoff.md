# Telegraf Toolkit — Operation CWAL handoff

## OPERATION CWAL — EXECUTION DIRECTIVE

**Branch:** `phase-3/canonical-mapper`
**CURRENT:** `workflow/active_work/P3-007-pre-pr-remediation.md`
**State:** HUMAN-AUTHORIZED — execute when operator enters `operation cwal`.

Phase 3 P3-001 through P3-006 were completed and archived at checkpoint `64bd122c9799b19b802545e9bc522e971ebdffa8`; both pointers were previously cleared. A subsequent independent pre-PR audit identified F-01–F-05 and the human requested a bounded correction on the same branch. This new P3-007 task is authorized now; do not reopen the earlier packets or ask again to begin.

**Action:** Sync local branch with the remote, read the P3-007 packet, implement approved-revision immutability, atomic metadata/history/audit writes, verified acceptance evidence, document reconciliation and focused regression tests. Run all required checks. Only after acceptance, archive P3-007 via git mv, clear both pointers, commit, normal-push, verify remote SHA, report `PRE_PR_REAUDIT_READY`. If blocked, report TASK_BLOCKED with evidence, leaving pointer intact.

**Security:** Maintain original Phase 3 boundaries. No historian writes, deployment, production access, secret disclosure, force pushes, PR actions, or changes to main. Stop for any genuinely new architecture/security gate or mandatory-check failure. Human owns the eventual PR and merge.
