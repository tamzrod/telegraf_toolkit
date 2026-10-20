# P2-003 — Per-connection capability probes and compatibility fixtures

**State:** HUMAN-AUTHORIZED ACTIVE WORK — explicitly approved mock/interface-only Phase 2 batch. ADR-0002 and ADR-0003 remain Proposed; no persistence, live secrets or real historian connections.
**Proposed branch:** `phase-2/database-registry`.
**Sequence:** P2-003 within a potential human-approved P2 work package.

## Objective and bounded scope
Define protocol-neutral capability probe interfaces, safe mocked error/unsupported result variants, explicit InfluxDB 1.x/2.x/3.x deployment/edition fixture matrix, TLS and deadline expectations. Do not perform real connectivity or assume every deployment of a version has the same dialect support.

## Acceptance criteria
Tests cover version/edition variance, unsupported query/write/schema capability, timestamp/scope metadata, timeouts and sanitized errors; mock evidence clearly separated from integration results; make check passes.

## Dependencies and prohibitions
- Read relevant accepted repository architecture and BLACK SHEEP WALL context. Do not infer that ADR-0002 or ADR-0003 is accepted merely because it contains a recommendation.
- Architecture gate: persistent profile storage, real secret storage/key custody and any live credential resolution require separate human acceptance of the relevant ADR. If acceptance has not occurred, deliver only isolated interfaces/mocks/tests and stop at the genuine dependency.
- No live InfluxDB historian reads/writes or Docker/Telegraf control; no credential harvesting, secret display or unsafe TLS bypass. Keep capabilities per connection and mock responses distinguishable from live probes.
- Run acceptance tests and `make check`, record actual evidence and limitations. Upon authorization and verification, `git mv` this task into `archive/active-work/`, update `handoff.md` and Active Work pointers, commit/push and remote-confirm before advancing.
- Never self-promote or create/open/approve/merge a PR. Stop for conflict, an unresolved required architecture decision, unverified checks or a real-system privilege requirement.

## Verification evidence (recorded 2026-10-20)

**Environment:** branch `phase-2/database-registry`; Node.js 24.21.0 / npm 11.19.1; Go 1.27.3 from the official tarball. No package versions changed.

**Deliverables:**
- `backend/internal/registrycontract/probe.go` — protocol-neutral `CapabilityProber`-compatible probe surface: `ProbeOutcome` (`supported`/`unsupported`/`unknown`), `Edition`, `TLSExpectation` (verify-by-default with explicit exception note), `ProbeRequest` (positive deadline required), `CapabilityReport` (mock-flagged, sanitized error field), an isolated `CompatibilityMatrix` keyed by edition, and `EditionProber` with an injectable clock.
- `backend/internal/registrycontract/probe_test.go` — 10 tests covering edition variance, unsupported dialect/schema cases, unknown-never-support, deadline/TLS request invariants, sanitized timeout, unknown-edition rejection, no-secret report JSON and fixture-profile projection.
- `docs/contracts/registry.md` — documented the three-valued outcome vocabulary and edition-keyed variance.

**Commands and actual outcomes:**
- `gofmt -l .` → clean; `go vet ./...` → pass; `go test ./internal/registrycontract/ -count=1` → `ok`, 27 tests passing in the package.
- `make check` → exit 0 (backend gofmt/vet/build/test; frontend `npm ci` + lint + build + `vitest run`; `policy-check.sh` → PASS).

**Acceptance mapping:**
- Version/edition variance: `TestMatrixEncodesEditionVarianceNotVersionParity` proves 2.x OSS and 2.x Cloud share a major version but differ (InfluxQL supported vs unknown).
- Unsupported query/write/schema capability: `TestMatrixCoversUnsupportedDialectAndSchemaCases` (3.x Serverless InfluxQL unsupported; 3.x Core schema discovery unsupported).
- Timestamp/scope metadata: report carries `ProbedAt` (injected clock asserted) and `Edition`/`ConnectionID`; scope completeness is covered by P2-002 validation.
- Timeouts: `TestProbeRequestRequiresDeadlineAndTLSExpectation` requires a positive deadline; `TestProbeDeadlineProducesSanitizedTimeout` proves an expired context yields a sanitized error.
- Sanitized errors: every failure path returns a `*SanitizedError`; `TestProbeReportJSONContainsNoSecretMaterial` scans the serialized report.
- Mock separated from integration: every report is `mock: true`; unknown editions/connections are rejected rather than guessed.

**Limitations:** the matrix is a mock expectation fixture, not a live probe; no real InfluxDB 1.x/2.x/3.x connectivity was performed (blocked by ADR-0003 for credentials). Edition labels are fixtures, not a validated support matrix (ADR-E unproposed). The deadline check is exercised via context state, not a real slow network call.
