# Work Package P4-007-FU1 — Pre-PR Validation and UI Safety

**Branch:** `phase-4/pipeline-designer`
**Base audit:** `853b187716fbb8a1566f2be87d0573eec78ac9bd`
**Scope:** One bounded follow-up to the Phase 4 independent pre-PR audit.
**Authority:** Human-authorized in `workflow/active_work/P4-007-FU1-pre-pr-validation-and-ui-safety.md`.
**Integration:** Branch checkpoint only. PR creation, review, approval and merge are human-only.

This report is verification evidence. The authoritative workflow state is in
`handoff.md` and `workflow/active_work/README.md`.

## Acceptance matrix

### 1. Manual TOML syntax validity — PASS

- `validateManual` parses the manual body with the pinned `influxdata/toml`
  parser before applying credential checks.
- A syntax failure returns the dedicated `invalid-toml` code with the `parse`
  check class. The diagnostic is sanitized: it names the line and the problem
  class but never echoes the source line or value.
- The implementation documents that TOML syntax validity is not Telegraf
  1.40.1 plugin or option semantic validation. Well-formed TOML outside the
  graph catalog remains valid.
- Coverage:
  - `TestManualTOMLSyntaxIsValidated` — malformed table, duplicate key,
    duplicate table and unclosed string are rejected with `invalid-toml`,
    `parse` class, a line number and no source echo.
  - `TestManualTOMLSyntaxAcceptsPluginsOutsideTheCatalog` — valid manual TOML
    using `inputs.exec` and `outputs.warp10` is accepted.
  - `TestMalformedManualTOMLIsRejectedAtSaveAndApprove` — the service rejects
    malformed manual TOML at save, leaves the draft generation unchanged, and
    refuses to approve a legacy malformed draft.
  - `TestViewRefusesMalformedLegacyManualExport` — the API view does not export
    stored malformed manual TOML; it reports an export issue instead.

### 2. Unsaved UI approval — PASS

- `documentEquivalenceKey` produces a stable key for the authoritative
  document. Object key order is normalized and canvas `layout` is excluded
  because it is presentation-only.
- `PipelinesPage` records the key of the document that was actually saved.
  Approval is disabled while the current graph or manual document differs from
  that saved key, and the panel shows a clear unsaved-changes message.
- `approveDraft` also refuses a dirty document as defence in depth, so the
  client check is not the only protection.
- Server generation and state invariants remain authoritative:
  `SaveDraft` and `ApproveDraft` compare `expected_generation` and revision
  state in the metadata store.
- Coverage:
  - `blocks graph approval after an unsaved edit and allows it after saving again`
  - `blocks manual approval after an unsaved edit and allows it after saving again`
  - Existing save/approve regression tests continue to pass.

### 3. Manual credential safety — PASS

- Credential checking is structural where possible: the parsed TOML tree is
  walked and non-empty string leaves under known credential-like keys are
  rejected unless they are opaque `@{toolkit:...}` secret-store references.
- Known inline forms are refused: credential-bearing URL userinfo, credential
  query parameters, PEM private key blocks, and credential-like key aliases.
- The assurance limits are documented next to `validateManual`: this is a
  bounded heuristic, not a comprehensive secret scanner. Unknown key names,
  non-string leaves and free-text secrets can still pass.
- Error messages are generic and do not echo rejected credential material.
- Approval revalidates the stored document, and export refuses malformed manual
  TOML instead of returning it as a valid export.
- Coverage:
  - `TestManualCredentialAssuranceLimits` — query parameters, nested credential
    tables, private key options, `pass` aliases and PEM blocks are rejected
    without echoing the literal; toolkit references, empty credential options
    and unrelated free text are accepted.
  - Existing `TestSecretReferencesRejectLiterals` and
    `TestManualModeKeepsSecretStoreReferences` continue to pass.

## Verification evidence

Environment:

- Go `go1.27.3` (`go env GOVERSION`, auto toolchain)
- Node `v24.16.0`
- npm `11.13.0`
- Vitest `5.0.5`

Commands run from the repository root unless noted:

```text
cd backend && go test -race ./...
  PASS — all backend packages

make check
  PASS — backend gofmt/vet/build/test, UI npm ci/lint/build/vitest, policy-check

cd backend && go test ./internal/pipelinecontract ./internal/pipelineservice ./internal/pipelineapi -count=1
  PASS

cd ui && npx vitest run src/features/pipelines/PipelinesPage.test.tsx
  PASS — 14 tests
```

Full UI suite from `make check`:

```text
Test Files  28 passed | 1 skipped (29)
Tests       300 passed | 1 skipped (301)
```

The skipped test is `src/api/schema.live.integration.test.ts`, an opt-in live
integration test that requires an external service. It is not a regression in
this work package.

`bash scripts/policy-check.sh` passes. The policy check is a narrow regex
heuristic, not comprehensive secret scanning.

## Limitations and unresolved nonblocking decisions

- Manual TOML syntax validation is not Telegraf plugin or option semantic
  validation. Manual mode can still contain a well-formed but unsupported
  Telegraf configuration.
- Credential detection is intentionally bounded. It does not prove arbitrary
  TOML is secret-free, and it does not inspect non-string leaves or infer
  secrets from unrelated key names.
- Canvas layout changes are excluded from the unsaved-approval comparison
  because layout is presentation-only. A layout-only change does not block
  approval.
- No live Telegraf runtime, deployment or historian test was performed. UI
  tests use mocked fetch; backend tests use the metadata store in a temporary
  directory.
- The work is a verified branch checkpoint, not integration. A human must open
  and merge the PR.

## Changed files

- `backend/internal/pipelinecontract/contract.go`
- `backend/internal/pipelinecontract/validate.go`
- `backend/internal/pipelinecontract/contract_test.go`
- `backend/internal/pipelineservice/service_test.go`
- `backend/internal/pipelineapi/api.go`
- `backend/internal/pipelineapi/api_test.go`
- `ui/src/features/pipelines/PipelinesPage.tsx`
- `ui/src/features/pipelines/PipelineModePanel.tsx`
- `ui/src/features/pipelines/PipelinesPage.test.tsx`
- `ui/src/features/pipelines/pipelineDocument.ts`
- `docs/WP_P4_007_FU1_WORK_PACKAGE_REPORT.md`
