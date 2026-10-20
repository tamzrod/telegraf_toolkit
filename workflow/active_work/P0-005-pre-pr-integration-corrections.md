# P0-005 — Pre-PR integration corrections and independent verification

**Authorization:** Human-promoted pre-PR correction microtask, to execute only on `phase-0/foundation`.
**Purpose:** Address findings of the Phase 0 pre-PR audit without reopening P0-001–P0-004 or expanding product scope.

## Starting state and facts to recheck
- `phase-0/foundation` contains four previously verified task checkpoints, archived as P0-001–P0-004, and a Phase 0 Go/UI scaffold, fixtures, proposed ADRs and CI.
- At audit time, the branch was ahead of and behind `main`; PR #2 was draft/open and GitHub returned `mergeable: false`. **Do not assume this proves a merge conflict**: inspect the actual merge status and Git history.
- Approved autonomous CWAL policy on `main` requires verified task checkpoints and **human-only PR initiation, review, approval and merge**. Retain that policy when reconciling.

## Authorized work surface
- Git history/integration of `origin/main` into the designated `phase-0/foundation` branch (without rewriting shared history).
- `AGENTS.md`, `operation cwal.md`, `handoff.md`, `workflow/active_work/README.md` **only as needed** to preserve human authorization, correct pointers, and approved CWAL policy after reconciliation.
- `docs/ROADMAP.md`: clarify `Proposed` ADRs are authored but **not accepted architecture decisions**.
- `Makefile`: make backend formatting validation non-mutating and consistent with CI.
- `.github/workflows/ci.yml` and relevant development/security documentation: document existing secret/version scan limitations; strengthen narrowly if feasible and testable without unrelated infrastructure work.
- Directly necessary test/fixture fixes solely to restore successful verification after integrating `main`.
- This task file and verified archive entry.

## Procedure and acceptance criteria
1. [ ] Fetch `origin/main` and `origin/phase-0/foundation`, inspect dirty state and compare ancestry, PR mergeability, and conflicting paths. Record actual findings; preserve all existing branch commits and the four archived task checkpoints. Never reset/force-push or discard unrelated work.
2. [ ] Reconcile current `main` into `phase-0/foundation` using a safe non-rewriting integration (merge preferred). Resolve any real conflicts deliberately; retain approved CWAL autonomy/human-only PR governance, completed Phase 0 artifacts and work-branch task state. If reconciliation needs an architectural choice or risks losing work, stop for human decision.
3. [ ] Clarify `docs/ROADMAP.md`: P0-002 delivered **Proposed** ADR-0002–0005, but their acceptance is outstanding. Do not mark any ADR Accepted or claim final architectural decisions are resolved.
4. [ ] Change local Go format **check** so it does not mutate files, and align its semantics with the CI `gofmt -l` check.
5. [ ] Verify policy checks actually cover the declared scope; explicitly document that regex-based secret/floating-version checks are limited and do not constitute exhaustive security assurance. Do not claim comprehensive coverage without implementing it.
6. [ ] Run `make check` and applicable policy/security checks on the reconciled branch; report commands, actual outcomes and environment constraints. Inspect new GitHub Actions runs and record their actual conclusion if accessible; if unavailable, report unverified rather than PASS.
7. [ ] Recheck final branch diff versus `main`, staged/unstaged status, secret exposure risks, pinned dependency declarations, archive contents (P0-001–P0-004), and pointer consistency. Verify that P0-005 can be archived with `git mv` after acceptance and the queue returns to empty.
8. [ ] Commit/archive P0-005 with the corrections and handoff/current-pointer updates; push to `origin/phase-0/foundation`, confirm remote checkpoint and report updated SHA, audit findings, verification evidence and any remaining PR readiness blocker.
9. [ ] Report `WORK_PACKAGE_READY` **only if** all required verification succeeds and integration readiness blockers are resolved. Otherwise report exact blocker without mislabeling readiness.

## Non-goals and prohibitions
- No new platform features, database adapters, runtime controls, secret-store selection, packaging decisions or real historian access.
- Do not redo previously checkpointed tasks without a directly demonstrated integration regression.
- No direct application pushes to `main`; no force pushes or destructive Git history edits.
- **Do not create/open/submit, mark ready, close, approve or merge any PR.** Existing PR #2 is human-managed; do not modify its state.
- Do not mark ADR-0002 through 0005 Accepted. Architectural acceptance is human-owned.

## Stop conditions
Stop and report if safe Git reconciliation fails, conflicts require a design decision, test/CI requirements cannot be validated, authorization/pointers become ambiguous, or sensitive/privileged access is needed. Preserve diagnostic evidence and the active task until verifiably completed.
