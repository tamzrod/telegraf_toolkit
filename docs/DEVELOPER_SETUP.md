# Developer setup and isolated fixtures

This guide covers reproducible local setup for the Telegraf Toolkit through
Phase 4. Everything runs **offline with isolated fixtures**: no database
credentials, live historian endpoints, Docker socket access or production
runtime operations are used or required. Optional live checks are skipped unless
their environment variables are set explicitly.

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
# Backend: modules are pinned in go.mod/go.sum and downloaded once.
cd backend
go vet ./...
go build ./...
go test ./...

# Frontend: clean, lockfile-exact install.
cd ../ui
npm ci
```

The backend has one third-party dependency, the pinned pure-Go SQLite driver
`modernc.org/sqlite v1.60.1` (no CGO toolchain required). The first build
downloads it; subsequent builds are offline.

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

## Pipeline designer and mapping coverage

Backend (`backend/internal/pipelinecontract`, `mappingcompile`,
`pipelineservice`, `pipelineapi`):

- Typed graph and manual documents are validated against a verified Telegraf
  1.40.1 plugin subset, then encoded as deterministic TOML. Graph TOML cannot
  be generated while an unresolved mapping pin remains.
- `mappingcompile` resolves only an approved pinned mapping revision and its
  approved canonical schema revision. Missing, draft, retired, stale or
  hash-mismatched references fail closed. Unit scaling, tag filters, unsupported
  transforms and unknown JSON fields return a diagnostic; the compiler never
  approximates them or mutates the stored mapping.
- The P4-007 acceptance test drives the HTTP surface with the same compiler
  wiring as `cmd/api`: RBAC, design validation, save/reload, stale generation,
  graph-to-manual transition, approval, rollback draft, audit, credential
  rejection and approved-mapping immutability.
- Unit and integrated tests use a temporary metadata store. They do not contact
  a real mapping store, deploy Telegraf or write a historian.

Frontend (`ui/src/features/pipelines`):

- The page edits the supported catalog, keeps canvas coordinates out of
  processor order, generates a browser-local read-only TOML preview, reviews
  graph/manual transitions and sends save/approve requests. It does not edit a
  mapping pin and does not display the server-compiled export.
- jsdom does not apply Tailwind CSS or media queries, so tests assert layout
  classes and landmarks rather than rendered geometry. Verify wide/narrow
  layout manually or with a real browser.

Unsupported or unverified in Phase 4: production deployment/restart, historian
writes, synchronization, unit and tag-filter mapping compilation, mapping-pin
editing in the UI, InfluxDB 1.x and non-Linux platforms.

## CI without external services

`.github/workflows/ci.yml` runs backend, frontend and policy jobs. It needs only
the pinned Go and Node toolchains and the committed lockfile. No database,
Docker or network service is started, so CI exercises unit and HTTP acceptance
tests against temporary stores only. Live container checks and
`src/api/schema.live.integration.test.ts` are not run by default; that test is
skipped unless `TTK_LIVE_API_URL`, `TTK_LIVE_API_TOKEN` and
`TTK_LIVE_PROFILE_ID` are supplied by an isolated harness. CI does not claim
live integration coverage.

## Policy check scope and limitations

`scripts/policy-check.sh` (also `make policy-check`, and the CI `policy` job,
which invokes the same script so local and CI behavior cannot drift) checks:

- floating/non-exact dependency specifiers in `ui/package.json`
  (`latest`, dist-tags, `^`/`~`/range operators, `1.x`/`1.2.*` wildcards, bare majors);
- the `go` directive in `backend/go.mod` pins a patch version;
- third-party GitHub Actions are pinned to full 40-hex commit SHAs;
- `image: …:latest` in YAML manifests;
- high-signal secret patterns (private-key headers, AWS access-key IDs,
  GitHub tokens, Slack tokens).

**These checks are narrow, regex-based heuristics. A PASS means those specific
patterns were not found; it is not exhaustive security assurance.** Known
limitations:

- Regex matching can miss obfuscated, split or encoded secrets, and cannot detect
  secrets in binary files or in formats not covered by the include list.
- Only `ui/package.json` is inspected for version pinning; transitive
  dependencies are governed by `ui/package-lock.json`, not by this check.
- No dependency vulnerability/CVE audit is performed here (use `npm audit` and
  Go's vulnerability tooling separately).
- No verification that a pinned SHA or digest is genuine, current or trusted.
- The image-tag check covers YAML only; other manifest formats are not scanned.
- Documentation that *discusses* secrets is not distinguished from real secrets
  beyond the pattern list above.

Treat a clean run as a guardrail, not a security guarantee. Dependency auditing,
secret-scanning services and human review remain necessary.

## Responsive and accessibility verification

Phase 1 automated coverage is in `ui/src/app/accessibility.test.tsx` and runs as
part of `npm test` / `make check`. It checks, for every registered route:

- exactly one `main` landmark, one `banner` and one labelled primary `navigation`;
- exactly one level-1 heading and no skipped heading levels;
- no duplicate element `id`s;
- the skip link is the first tab stop and targets `#main-content`;
- every primary navigation link is reachable by keyboard and activates with Enter;
- the responsive menu toggle exposes `aria-controls`/`aria-expanded` and is
  keyboard-operable;
- each route renders without error and the unknown route reaches the not-found page.

**What jsdom cannot verify.** Automated tests run in jsdom, which does not apply
Tailwind CSS or media queries, so these checks confirm semantics and state, not
visual layout. Verify responsiveness manually:

1. Run the SPA: `cd ui && npm ci && npm run dev`.
2. In the browser, use device emulation at ~375 px (narrow) and ~1280 px (wide).
3. Confirm the sidebar collapses to the **Menu** toggle below the `md` breakpoint
   and expands above it, that content does not overflow horizontally, and that
   tabbing still reaches every navigation link at both widths.
4. Enable reduced motion if relevant and confirm focus outlines are visible on
   links, buttons and the skip link.

Real-browser and screen-reader validation remains a manual step; automated tests
are a guardrail, not proof of accessibility conformance.

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

The API and UI remain scaffolds in their user-facing behavior. The health
payload reports `scaffold: true` and `connected: false` so no consumer mistakes
them for live connections. The **control-plane metadata store** (persistent
SQLite per [ADR-0002](adr/0002-metadata-store-selection.md)), the
**envelope-encrypted secret store** (per [ADR-0003](adr/0003-secret-store-integration.md))
and an **authorized Registry HTTP API** (see [`REGISTRY_API.md`](REGISTRY_API.md))
are now real; see also [`METADATA_STORE.md`](METADATA_STORE.md) and
[`SECRET_STORE.md`](SECRET_STORE.md). A **read-only live capability probe** also
exists (see [`LIVE_PROBING.md`](LIVE_PROBING.md)); it performs GET requests only
and never writes to a historian. The host-agent interface and packaging
remain **Proposed** ADRs (`docs/adr/0004`–`0005`), not implemented.

The Phase 4 pipeline contract, mapping compiler, revision service and
`/api/v1/pipelines` routes are implemented and checkpointed on
`phase-4/pipeline-designer`, not merged. They write configuration metadata and
audit rows only; they do not deploy, restart or write a historian.

The backend accepts three optional flags: `-store` (metadata DB path), `-key-file`
(master key for the secret store) and `-token-file` (authentication tokens). With
no token file the Registry and pipeline data routes are not registered, so there
is no anonymously reachable data surface.

The token file and the master key file must be **owner-only (0600)**; a
group- or other-readable file is refused at startup. On a filesystem that
reports no POSIX permission bits (Windows), both files are refused, the
Registry and pipeline data routes stay unregistered and the secret store stays
unattached — run the Registry locally on Linux.
