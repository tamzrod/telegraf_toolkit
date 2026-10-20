# Developer setup and isolated fixtures

This guide covers reproducible local setup for the Telegraf Toolkit Phase 0
scaffold. Everything runs **offline with isolated fixtures**: no database
credentials, live historian endpoints, Docker socket access or production
runtime operations are used or required.

## Prerequisites (pinned)

| Tool | Pinned version | Notes |
| --- | --- | --- |
| Go | 1.27.3 | `backend/go.mod` declares `go 1.27.3` |
| Node.js | 24.21.0 | CI and local toolchain |
| npm | 11.x | Ships with Node 24 |

Install Go from the official release for your platform and verify the
published checksum before use. Do not substitute a floating version.

## Clean environment creation

From a fresh clone:

```bash
# Backend: no external module downloads are required (standard library only).
cd backend
go vet ./...
go build ./...
go test ./...

# Frontend: clean, lockfile-exact install.
cd ../ui
npm ci
```

`npm ci` is used instead of `npm install` so the committed
`ui/package-lock.json` is authoritative and the install is reproducible.

## Exact check commands

```bash
make check          # backend + frontend, mirrors CI
make backend-check  # gofmt, vet, build, test
make frontend-check # npm ci, lint, build, test
make run-api        # run the scaffold API on :8080
```

Individually:

```bash
# backend
cd backend && test -z "$(gofmt -l .)" && go vet ./... && go build ./... && go test ./...

# frontend
cd ui && npm run lint && npm run build && npm test
```

## Isolated mock fixtures

Fixtures are deterministic and test-only. They never contact a real system and
are explicitly labeled.

### Backend (`backend/internal/fixtures`)

Mock InfluxDB capability fixture shapes:

| Mock profile | Version | Scope shape | Query dialects | Schema discovery |
| --- | --- | --- | --- | --- |
| `mock-influx-1x` | 1.x | `database/retention-policy` | `influxql` | yes |
| `mock-influx-2x` | 2.x | `organization/bucket` | `flux`, `influxql` | yes |
| `mock-influx-3x` | 3.x | `database` | `sql`, `influxql` | no (probed) |
| `mock-influx-3x-sql-only` | 3.x | `database` | `sql` | no |

Every profile sets `mock: true` and a `MOCK …` display name. Credentials appear
only as `credential_ref_id` references (for example `mock-cred-readonly-1x`);
no secret values exist in the fixtures.

### Frontend (`ui/src/fixtures`)

`profiles.ts` mirrors the same shapes for UI tests; `syncState.ts` mirrors the
block state vocabulary.

## Fixture test coverage

Backend (`backend/internal/fixtures`, `backend/internal/synccontract`):

- Distinct source/destination scopes are required; identical selections are rejected.
- Unsupported capabilities (for example InfluxQL against a SQL-only profile, or
  writing to a profile with no write protocol) are reported as errors.
- Capabilities are per connection, not inferred from the version label.
- `COUNT_MATCH` is never verified; `VERIFIED` requires a stronger policy.
- Fixture JSON contains no secret-like keys.

Frontend (`ui/src/fixtures`):

- Every profile is labeled mock and test-only.
- Source/destination selection requires both sides and rejects non-distinct pairs.
- `COUNT_MATCH` and `VERIFIED` labels remain distinct and count-only is never verified.

Run them:

```bash
cd backend && go test ./...
cd ui && npm test
```

## CI without external services

`.github/workflows/ci.yml` runs backend, frontend and policy jobs. It needs only
the pinned Go and Node toolchains and the committed lockfile. No database,
Docker or network service is started, so CI exercises the fixtures only. This is
deliberate: the scaffold has no live integration, and the checks do not claim
integration coverage.

## Portability limits

- Tests run on Linux amd64 (CI and development target). Other platforms are not
  yet validated.
- Fixtures model capability **shapes**, not real InfluxDB responses. They do not
  verify cross-version query behavior.
- No test in Phase 0 proves live connectivity, historian integrity or point-level
  equivalence. Those require later phases with real isolated environments.
- TypeScript is pinned to 6.0.3 because `typescript-eslint` rejects newer majors;
  see [`DEVELOPMENT.md`](DEVELOPMENT.md).

## Scaffold boundary

The API and UI are scaffolds. The health payload reports `scaffold: true` and
`connected: false` so no consumer mistakes them for live connections. Metadata
store, secret store, host-agent interface and packaging remain **Proposed** ADRs
(`docs/adr/0002`–`0005`), not implemented.
