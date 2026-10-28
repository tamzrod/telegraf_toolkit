# Telegraf Toolkit — Operation CWAL handoff

## OPERATION CWAL — EXECUTION DIRECTIVE (binding)

**Branch:** `phase-3/canonical-mapper`
**CURRENT:** `workflow/active_work/P3-008-final-pre-pr-corrections.md`
**State:** HUMAN-AUTHORIZED ACTIVE; execute when operator enters `operation cwal`.

P3-001–P3-007 were completed and archived. After P3-007 commit `93fb49895819362d38cee80bef37a118f7bb4ddf`, independent re-audit identified remaining issues, so the human authorized this bounded P3-008 correction on the same branch. P3-007's earlier completion claims are historical; do not treat them as proof that the new acceptance requirements pass.

**Action now:** Sync local branch to remote, read CURRENT P3-008 packet, fix the remaining approved revision immutability bypasses (F-01), rollback history/audit atomicity (F-02), missing committed per-edition evidence (F-03), and insufficient focused regression tests (F-05). Execute and verify without requesting new approval for these bounded corrections. Only when all checks pass, append real evidence, archive P3-008 via `git mv`, clear both pointers, commit, normal-push, verify remote SHA and report `PRE_PR_REAUDIT_READY`. If any mandatory requirement cannot pass, leave pointer intact and report `TASK_BLOCKED` with evidence.

**Bounds:** No new architecture decision, historian writes, deployment, production access, secret disclosure, force-push, PR creation/approval/merge or changes to main. Human owns PR.
