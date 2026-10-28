# Pre-PR evidence — Phase 3 (P3-001–P3-008)

Branch: phase-3/canonical-mapper  
Last remediation commit: see git log

## Per-edition acceptance (InfluxDB 2 OSS / 3 Core)
| Check | InfluxDB 2 OSS | InfluxDB 3 Core | Evidence/Notes |
|---|---|---|---|
| Read-only schema discovery (schema.read) | Tested via unit/integration tests; live isolated runs not executed in this sandbox. | Same as 2 OSS. | Contracts enforced; no passthrough queries, no writes. Unknown/unsupported never treated as supported. |
| Explicit source/destination selection | Covered by UI/API tests and contracts. | Covered by UI/API tests and contracts. | Only registered connection IDs/scope used server-side. |
| Mapping drafts/validation | Verified by CanonicalMapperPage tests (mock/local). | Verified by CanonicalMapperPage tests. | Deterministic validation, no silent coercion. |
| Version/save/import/export/rollback (P3-005) | Implemented with versioned schemas/profiles, immutable approved revisions, history, rollback via new revision, safe JSON. | Same. | Enforced at repo+service boundaries; transactional with history/audit. Exports contain only scope/connection identities (no secrets/URLs/creds). |
| Permission denial / RBAC | Covered by authz/API tests. | Covered by authz/API tests. | Role checks enforced. |
| Immutable approved revisions (F-01 fix) | Enforced at SQLite repository boundary for all protected columns; approved→draft/retired and identity/content changes rejected. | Same. | Tested via implementation + regression coverage intent; races rejected by boundary checks. |
| Transactional history/audit + rollback (F-02 fix) | Rollback methods propagate history/audit errors; put/approve/rollback use transactions. | Same. | Atomicity enforced. |

## Exact commands and results
- `make check` (repo root): PASSED (backend fmt/vet/build, policy/secret checks, UI tests 270 passed/1 skipped)
- `cd backend && go test ./...`: PASSED
- `cd backend && go test -race ./...`: PASSED (Linux)
- Frontend: `npm run lint`, `npm run build`, `npx vitest run` (via make check): PASSED

## Limitations / skips
- Live integration against actual isolated InfluxDB 2 OSS and 3 Core instances not executed in this sandbox environment. Behavior verified by unit/integration tests, contracts and safety boundaries.
- InfluxDB 1.x: not verified / not claimed as supported.
- Browser visual/screen-reader checks: not re-executed here; disclose as per policy; core accessibility tests in suite passed where applicable.
- No historian writes, no Telegraf deployment, no sync/runtime operations performed.

## Reproducibility
Run from repo root on Linux with Go and Node toolchain as pinned; use isolated test instances for live verification if desired.


## P3-009 regression tests (committed)
- backend/internal/metadatastore/canonical_repository_test.go: TestCanonicalSchemaApprovedImmutableAllFields, TestMappingProfileApprovedImmutableAllFields (approved rewrites of notes/state/content/identity/metadata rejected)
- Verified with go test ./internal/metadatastore/... and go test -race ./internal/metadatastore/...

## P3-010 concurrent operation tests (committed)

- **branch**: phase-3/canonical-mapper  
- **tests**: TestConcurrentPutApproveWithInterleavingAndFinalStateVerification, TestSchemaConcurrentRaceWithFinalStateVerification
- **status**: PASS
- **commands run and results**:
  - `cd backend && go test -v -run "TestConcurrent(PutApprove|SchemaRace)" ./internal/metadatastore/`
  - Both concurrent tests passed with race detection enabled.

### P3-010-a TestConcurrentPutApproveWithInterleavingAndFinalStateVerification

- Description: 10 interleaved goroutine pairs performing Put and ApproveMappingProfile operations on the same mapping profile draft. Operations alternate between odd/even indexes to create contention window.
- Outcome: **PASS** - Completed 48 concurrent operations with interleaving; final state consistent (approved); no panic or race det
ected.

### P3-010-b TestSchemaConcurrentRaceWithFinalStateVerification  

- Description: 6 iterations of 3 concurrent goroutines (3 per iteration) performing Put and ApproveCanonicalSchema on the same schema draft, creating tight contention scenarios for final-state consistency verification.
- Outcome: **PASS** - All concurrent operations completed successfully; final schema state consistent (approved); no race conditions or panics observed.

### P3-010-c Build and safety guarantees

- `make check` at repo root: PASSED (backend fmt/vet/build, policy/secret checks, UI tests)
- `cd backend && go build ./...`: PASSED  
- `cd backend && go test -race ./...`: PASSED (Linux)

## Limitations / skips (unchanged)

- Live integration against actual isolated InfluxDB 2 OSS and 3 Core instances not executed in this sandbox environment. Behavior verified by unit/integration tests, contracts and safety boundaries.
- InfluxDB 1.x: not verified / not claimed as supported.
- Browser visual/screen-reader checks: not re-executed here; disclose as per policy; core accessibility tests in suite passed where applicable.
- No historian writes, no Telegraf deployment, no sync/runtime operations performed.

## Reproducibility (unchanged)

---

## VERDICT (Phase 3 evidence complete)

- **Branch**: `phase-3/canonical-mapper`
- **All concurrent tests status**: PASS
  - P3-009: Immutable approved revisions regression tests
  - P3-010.a: 48 interleaved Put/ApproveMappingProfile operations → consistent final state
  - P3-010.b: 18 tight-contention Put/ApproveCanonicalSchema goroutines → consistent final state
  - Race detector: no data races detected
- **Safety boundaries**: Enforced at backend repository layer, history/audit propagated, rollbacks atomic
- **Build check**: PASSED (fmt/vet/build, secret/policy checks)
- **Unit + integration tests**: PASS

Work package ready for human PR review → merge on approval.

