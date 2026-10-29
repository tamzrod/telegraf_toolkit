# P4-006-mapping-compatibility — Phase 3 approved mapping integration

**STATE:** AUTHORIZED  
**BRANCH:** `phase-4/pipeline-designer`  
**ORDER:** 6 of 7  
**DEPENDENCY:** P4-005 checkpointed  
**DESIGN AUTHORITY:** `docs/PHASE_4_PLAN.md` and existing accepted ADRs/contracts.

## Objective and allowed scope
Mapping reference/compilation integration and tests; no historian or runtime writes.

## Required implementation
Resolve and verify approved mapping revision only, preserve mapping semantics, emit supported Telegraf processing config or fail closed with diagnostics; stale/revoked/missing references handled; never mutate approved mapper artifacts.

## Acceptance and evidence
Tests for supported/unsupported transformations, revision changes, no silent approximation or secret leak; `make check`.

## Safety / stop conditions
Do not edit `main`, open PRs, force-push, deploy, restart Telegraf, write historians, start synchronization, expose secrets, or self-expand the work package. If required contracts conflict or a privileged action is necessary, record blocker rather than invent permissions.

## CWAL checkpoint
Run real checks; report limitations; archive this task under `archive/active-work/`, commit and push evidence and advance both handoff pointers to the next authorized task automatically. Do not request intermediate approval.
## Verification appended 2026-10-29

`mappingcompile` resolves only an approved mapping revision whose revision and content hash match the graph pin, then requires the cited canonical schema revision to be approved. Missing, draft, retired, stale and hash-mismatched references fail closed with a secret-free code. Unit scaling, a residual factor, tag filters, type changes without a transformation, and unknown JSON fields fail the whole mapping; no partial or approximated processor is returned. Identity rules emit no processor. `Generate` and `ExecutionPlan` refuse a document that still carries an unresolved pin, and preview/mode transition compile a copy while the stored graph keeps the pin. Tests compare the stored mapping and schema rows before and after compile, preview and refusal, so approved artifacts are not mutated.

Reported trailing-JSON concern confirmed: `json.Decoder.More()` returns false after a complete top-level value when the next byte is `]` or `}`, so `...}]` and `...}}` were accepted. `TestParseRulesRejectsTrailingJSON` reproduced both failures, then passed after `parseRules` changed to require `io.EOF` from a second decode. Trailing whitespace still parses; a second object, `null`, comma, or garbage is rejected.

Commands and real results:
- `go test -count=1 ./internal/mappingcompile/ ./internal/pipelineservice/ ./internal/pipelinecontract/ ./internal/pipelineapi/` exited 0 (0.021s, 0.023s, 0.006s, 0.009s).
- `go test -race -count=1 ./internal/mappingcompile/` exited 0 (1.258s).
- `make backend-check` exited 0: gofmt check, `go vet`, `go build`, `go test ./...`; all backend packages passed.
- `make check` exited 0: UI `npm ci`, lint, build, and `vitest run`; 28 test files passed and 1 skipped, 297 tests passed and 1 skipped. The skip is `src/api/schema.live.integration.test.ts` because `TTK_LIVE_*` was unset. `scripts/policy-check.sh` printed `policy-check: PASS (limited regex checks only)`.
- `git fetch origin --prune` initially failed with `fatal: couldn't find remote ref refs/heads/phase-3/canonical-mapper`. The stale duplicate `remote.origin.fetch` entry was removed locally with `git config --unset-all --fixed-value ...`; the wildcard refspec remains and the subsequent fetch exited 0. No remote ref or history was altered.

Limitations: unit scaling and tag filters remain unsupported by the verified processor subset, so they fail rather than compile. The pipelines page does not yet edit a mapping pin; integration is exercised through the service and API preview. No collector was deployed or restarted, no historian was written, and no live mapping/schema store outside the test metadata store was contacted.
