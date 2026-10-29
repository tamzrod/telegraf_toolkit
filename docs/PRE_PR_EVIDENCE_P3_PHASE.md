# Pre-PR evidence — Phase 3 (P3-001–P3-010 follow-ups)

Branch: `phase-3/canonical-mapper`  
Recorded: 2026-10-29, after P3-010-FU3.  
The earlier `PRE_PR_REAUDIT_READY` claim at `5f229060` is withdrawn. This file records commands that were run in this closure.

## Per-edition acceptance (InfluxDB 2 OSS / 3 Core)

| Check | InfluxDB 2 OSS | InfluxDB 3 Core | Evidence/Notes |
|---|---|---|---|
| Read-only schema discovery (schema.read) | UNRUN live. Unit and API tests cover the fixed GET `SHOW` path. | UNRUN live. Same tests. | No passthrough queries and no historian writes in these checks. |
| Explicit source/destination selection | UI/API tests and contracts. | UI/API tests and contracts. | Registered connection IDs and scope stay server-side. |
| Mapping drafts/validation | CanonicalMapperPage tests (mock/local). | Same tests. | Local validation only. |
| Version/save/import/export/rollback (P3-005) | Repository and service tests on SQLite. | Same tests. | Approved revisions reject later puts. Exports are covered by existing secret-boundary tests. |
| Permission denial / RBAC | authz and API tests. | Same tests. | Role checks in the suite. |
| Immutable approved revisions | SQLite guarded `UPDATE ... WHERE state = 'draft'`, then exact field reads. | Same tests. | `TestCanonicalEditBeforeApprovalAndRejectsLaterEdits` and `TestMappingEditBeforeApprovalAndRejectsLaterEdits` assert content, hash, notes and identity after a rejected edit. |
| Transactional history/audit + rollback | `WithinTransaction` put + history. | Same tests. | A forced history error rolls the edit back. See transaction tests below. |

Live InfluxDB 2 OSS and InfluxDB 3 Core were not started. InfluxDB 1.x remains unverified.

## Transaction proof (not inferred from `-race`)

`go test -race` reports data races in the test process. It does not prove that an approved SQLite row rejects a later edit. These tests do:

| Test | What it checks |
|---|---|
| `TestCanonicalPutOnExternalTxRollsBack` | `Put` on an externally owned `*sql.Tx` leaves `Rollback` usable. After rollback the schema row is absent. |
| `TestMappingPutOnExternalTxRollsBack` | Same for mapping profiles. |
| `TestCanonicalBeginTxErrorWritesNothing` | Injected `BeginTx` error is returned. No schema row is written. |
| `TestMappingBeginTxErrorWritesNothing` | Same for mapping profiles. |
| `TestCanonicalPutJoinsOuterTransactionWithHistory` | Put and history commit together. A later history error rolls the edit back and preserves content, hash and notes. |
| `TestMappingPutJoinsOuterTransactionWithHistory` | Same for mapping profiles, including source connection id. |
| `TestCanonicalEditBeforeApprovalAndRejectsLaterEdits` | Draft edit then approval stores that content, hash and notes. A later put leaves those fields and identity unchanged. |
| `TestMappingEditBeforeApprovalAndRejectsLaterEdits` | Same for mapping profiles, including connection and scope fields. |

An unguarded upsert after approval would fail the last two tests. `Put` joins an existing `*sql.Tx` and does not call `BeginTx`, `Commit` or `Rollback` on it. A real `BeginTx` failure returns before any write.

## Commands and observed results (2026-10-29)

All of these exited 0.

`cd backend && go test ./...`

```
ok  internal/authz
ok  internal/fixtures
ok  internal/httpapi
ok  internal/liveprobe
ok  internal/metadatastore
ok  internal/registryapi
ok  internal/registrycontract
ok  internal/registryservice
ok  internal/secretstore
ok  internal/synccontract
```

`cmd/api` and `internal/version` have no test files. The command printed `?` for those two packages.

`cd backend && go test -race ./...`

Same packages, all `ok`. metadatastore 3.875s, registryapi 4.689s, registryservice 5.187s, httpapi 1.184s. No race was reported.

`cd backend && go test -race -count=10 -timeout 300s ./internal/metadatastore/ -run 'TestCanonical|TestMapping|TestApproved|TestConcurrent|TestSchema'`

`ok github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore 17.742s`

`make check` from the repo root ran `gofmt` check, `go vet`, `go build`, `go test ./...`, `npm ci`, `npm run lint`, `npm run build`, `npm test`, and `scripts/policy-check.sh`.

UI result from that run: `Test Files  25 passed | 1 skipped (26)`, `Tests  270 passed | 1 skipped (271)`. The skipped file is `src/api/schema.live.integration.test.ts` because `TTK_LIVE_API_URL`, `TTK_LIVE_API_TOKEN` and `TTK_LIVE_PROFILE_ID` were unset. `policy-check: PASS (limited regex checks only)`.

Concurrent repository tests write each goroutine's error into its own result slot. `TestSchemaConcurrentRaceWithFinalStateVerification` keeps the original content, hash and notes. Approval-versus-edit tests accept only the original triple or the single edited triple, then reject a further edit when the row is approved.

## Limitations

- Live InfluxDB 2 OSS: UNRUN.
- Live InfluxDB 3 Core: UNRUN.
- InfluxDB 1.x: unverified.
- No historian writes, deployment, runtime sync, or production access.
- Browser screen-reader checks were not repeated outside the existing Vitest accessibility tests.
- `d68c3c9443ae82c88cf54ccdf3320b4e875b13cc` is not an ancestor of this branch. Its parent object is absent from this clone. That snapshot still uses the non-transactional `BeginTx` fallback. It was not merged and its fallback was not restored.

