# P3-008 — Final pre-PR security and evidence corrections

> **State: HUMAN-AUTHORIZED CURRENT — execute on `operation cwal`.** Work on existing `phase-3/canonical-mapper` only. This is a bounded re-audit remediation following `93fb49895819362d38cee80bef37a118f7bb4ddf`; P3-001–P3-007 remain archived. Do not merely report status when invoked.

## Objective
Close outstanding independent re-audit issues **F-01, F-02, F-03 and F-05**. Preserve approved Phase 3 architecture; do not expand functionality or reopen decisions. P3-007's earlier completion report is historical, not proof of these checks.

## Findings and acceptance

### F-01 — Approved revision immutability (HIGH)
Current `backend/internal/metadatastore/canonical_repository.go` checks only old `content_json` when a row is already approved, then performs unconditional `ON CONFLICT ... DO UPDATE`. This may permit changes to approval state, logical key, revision/version, hash, approver/creator metadata, and mapping identity/scope fields, as well as pre-check/update races.

Enforce complete approved-revision immutability **atomically at the SQLite write boundary**. Only a controlled draft→approved transition may modify approval-state fields without changing revision identity/content. Direct repository calls and concurrent updates must not bypass this. Reject an approved→draft or approved→retired overwrite unless a separately approved, explicitly modelled transition exists; prefer new revisions for changes. Validate all protected columns, not just content. Test both canonical schemas and mapping profiles, service/API paths and concurrent attempts.

### F-02 — Transactional revision history and audit (HIGH)
`backend/internal/registryservice/service.go` still ignores `repos.RevisionHistory.Record` failures in BOTH rollback methods. Propagate these errors so the whole SQLite transaction rolls back. Verify creation, update, draft approval and rollback all atomically persist revision, history and audit. No committed mutation may lack required history/audit. Exercise injected history and audit write failures and collision/race cases.

### F-03 — Per-edition acceptance evidence (HIGH)
The P3-006 archive only claims that `/tmp/p3-006-evidence.md` was verified; actual per-edition results were not committed. Inspect available local file and real logs; **commit sanitized evidence in a repository-local document** with matrix for InfluxDB 2 OSS and InfluxDB 3 Core, exact tested versions, commands, observed results, pass/fail/skips, limitations and reproducibility notes. Mark any missing/unrun tests explicitly; never invent or infer execution. Append links and acceptance notes to P3-005/P3-006/P3-007 archives without rewriting original historical criteria.

### F-05 — Regression tests (HIGH for release gate)
The P3-007 commit introduced no test-file changes. Add focused Go repository/service/API regression tests proving all newly corrected invariants: approved-row overwrite attempts for all protected columns, draft→approved transition, revision collisions, concurrent writes, rollback history/audit failure rollback, import/export validation and secret redaction, RBAC denial, stale connection/scope behavior. Existing tests may be cited precisely, but run and record them; no unsupported claim of coverage.

## Workflow and verification
1. Inspect `git status`, exact branch, remote SHA and audit evidence. Explicitly fetch `origin refs/heads/phase-3/canonical-mapper`, accounting for stale fetch configuration, then fast-forward safely if possible. If diverged, preserve both histories and reconcile safely; never force-push/reset away commits.
2. Implement narrow corrections and tests. Inspect any proposed change for security regressions; use existing SQLite transactional metadata-store conventions and authorization boundaries.
3. Run `gofmt`, `go vet ./...`, `go test ./...`, `go test -race ./...` (backend, supported Linux), `make check`, frontend tests/lint/typecheck/build, policy and secret checks, doc-link validation, and genuinely available isolated InfluxDB 2 OSS/3 Core acceptance tests. Record exact commands and results. Disclose missing browser/screen-reader checks and InfluxDB 1.x as unverified.
4. Only after passing mandatory acceptance, append real verification/evidence to this packet; archive via `git mv` to `archive/active-work/`, clear both CURRENT pointers, commit and normally push the same branch; independently verify exact remote SHA using `git ls-remote`.
5. Report `PRE_PR_REAUDIT_READY` only when F-01/F-02/F-03/F-05 are actually closed and verified. Otherwise report `TASK_BLOCKED`, with reasons and pointers intact.

## Boundaries
No PR creation, approval or merge; no main change, force-push, production historian access/write, deployment, synchronization runtime changes, secret disclosure or self-approved new architecture. Human owns PR.
