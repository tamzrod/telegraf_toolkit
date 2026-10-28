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
