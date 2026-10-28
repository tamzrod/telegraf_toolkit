# P3-010 — Atomic SQLite approved-revision immutability

> **HUMAN-APPROVED CURRENT** on `phase-3/canonical-mapper`. Execute upon `operation cwal`, after safely syncing the local branch. This is a narrowly bounded final pre-PR correction. Preserve P3-001 through P3-009 as archived.

## Problem and acceptance
Independent review of commit `5f7cc8ddbf59bceb4095c14969ae7bc77e89fc41` found that `backend/internal/metadatastore/canonical_repository.go` still performs a SELECT-based approved-state check followed by unconditional `INSERT ... ON CONFLICT DO UPDATE`, leaving a time-of-check/time-of-use race for both canonical schemas and mapping profiles. Earlier regression tests exercise only sequential overwrites.

1. Replace this with **atomic database-enforced immutability**. Ordinary draft edits must use a guarded write permitting only draft rows; new revisions must be insert-only. No ordinary write may modify an already approved revision, including state, content/hash, notes, identity, versions, timestamps, approval metadata or mapping connection/scope. Ensure failure is explicit (check affected rows), not silent.
2. Approval must be a separate, guarded draft-to-approved state transition that preserves content and identity and checks affected rows, within the existing SQLite transaction containing revision history and audit. Consider database triggers if necessary to enforce the invariant even for direct repository writes. Do not break legitimate first-time revision creation or approval.
3. Add committed tests for canonical schema and mapping profiles: all protected fields including notes; simultaneous approval versus editing and competing writers; revision collisions; approval-state races; history/audit failure rollback and resulting persisted state. Include a regression case that would fail the old check-then-unconditional-upsert design. Avoid timing-dependent flaky assertions.
4. Update `docs/PRE_PR_EVIDENCE_P3_PHASE.md` with specific tests, exact commands/results and honest limits. Do not assert concurrency protection merely because `go test -race` passes; demonstrate the SQLite race invariant. Existing live InfluxDB 2 OSS / 3 Core tests not rerun must remain explicitly labelled unrun; InfluxDB 1.x stays unverified.

## Workflow
- Verify branch, clean worktree, and remote SHA; explicitly fetch `origin refs/heads/phase-3/canonical-mapper` due to prior stale fetch config, safely fast-forward or reconcile non-destructively. Never force-push.
- Implement only these bounded changes; preserve RBAC, audit, secret isolation and no-historian-write constraints.
- Run targeted Go tests, `cd backend && go test ./...`, `cd backend && go test -race ./...`, `make check`, policy/secret and documentation checks; record actual results. If any required gate fails, report TASK_BLOCKED rather than claiming success.
- Upon verified acceptance, append evidence to this packet, archive by `git mv` to `archive/active-work/`, clear both CURRENT pointers, commit and normal-push on the same branch, and confirm exact remote SHA with `git ls-remote`. Report `PRE_PR_REAUDIT_READY` for independent review, not automatic PR-ready.

## Prohibited
No PR creation, approval or merge; no changes to main, historian writes, production access, deployment, runtime sync, secret exposure, force push, or self-approved architecture expansion.
