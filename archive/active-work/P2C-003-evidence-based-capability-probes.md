# P2C-003 — Report capability observations without edition-based assumptions

> **State: HUMAN-AUTHORIZED ACTIVE WORK — COMPLETE, CHECKPOINTED 2026-10-21** (archived to `archive/active-work/`). Capability outcomes are now backed by recorded observations against the real, isolated InfluxDB 2 OSS and 3 Core containers; no capability is asserted from the edition label.

## Intent
Review backend/internal/liveprobe/probe.go, docs/LIVE_PROBING.md and UI transformations. Implement bounded nonmutating query/schema capability checks where supported, preserving pinned peer DNS protection, server-authorized profile selection and sanitized outputs.

## Starting state
Base branch: `phase-2/registry-completion`, created from `main` following merged PR #6. The operational SQLite Registry, envelope secret store, authorized APIs, secure live probing and partial live UI are implemented, with P2L-006/007 security controls. ADR-0002/0003 Accepted, ADR-0004/0005 Proposed.

## Required result
Actual read-only observations drive supported/unsupported/unknown; unknown on ambiguous or unsafe checks, no capabilities asserted solely by edition; label write protocols unverified unless separately proven by an authorized isolated write check.

## Execution boundary
- Allowed: relevant `backend/`, `ui/`, `docs/` and isolated `testing/influxdb/` artifacts as needed for this task only.
- Dependencies: P2C-001, P2C-002.
- Explicit non-goals: No production historian writes; no uncontrolled remote targets; do not infer InfluxDB 1.x; write probes are NOT allowed in read-only route.
- No unauthorized privileged or destructive actions; no production historian or credential access.

## Acceptance criteria
- [ ] Actual read-only observations drive supported/unsupported/unknown; unknown on ambiguous or unsafe checks, no capabilities asserted solely by edition; label write protocols unverified unless separately proven by an authorized isolated write check.
- [ ] Preserve server-side RBAC, administrator-only credential binding, P2L-006 pinned-peer/TLS/SSRF controls, and secrets/redaction invariants.
- [ ] Evidence corresponds to actual tested behavior; unsupported/unverified remains explicitly labelled.
- [ ] No new secrets, deployment or production historian writes.

## Verification evidence
Isolated InfluxDB 2 OSS and 3 Core probes plus malicious endpoint/timeout/negative auth tests; make check and race tests.

## Refined execution and acceptance contract (P2C-003)

- Replace edition-derived supported/unsupported capability assertions with **specific nonmutating observations**. Record for each dialect/schema check what was actually attempted, status, and supported/unsupported/unknown semantics.
- Do not treat an HTTP 200, edition label, a 401/403, or an unrecognized endpoint response as proof of query/write functionality. Auth errors, timeouts, malformed/ambiguous data, or untested functionality yield **unknown**.
- Prober remains GET/read-only; never issue test writes or requests to arbitrary user-selected endpoints. Any write-protocol capability without an independently authorized synthetic test is **unknown**, not supported or unsupported.
- Preserve profile-based destination selection, approved credential binding and one-time pinned peer/TLS/SSRF checks on **every** outgoing request; prevent redirects/proxies and bound response size, time and request budget.
- Ensure API wire format, UI three-state mapping and docs agree (unknown must never collapse to unsupported or supported).
- Verify against synthetic isolated InfluxDB 2 OSS / 3 Core; test untrusted/mixed DNS, auth denied, timeouts, redirects, inconclusive responses and edition variation. InfluxDB 1.x is unverified.
- Deliver a capability evidence matrix and exact executed test output; run make check and race tests.

## Stop conditions
Stop and report `TASK_BLOCKED` for unsafe network/credential access, unresolved architecture, or required testing unavailable. No weakening tests or replacing working behavior with mocks.

## Git lifecycle
Human-promoted and authorized as part of the six-task queue. Execute only when designated CURRENT; advance one task at a time. Complete tests, `git mv` to `archive/active-work/`, update aligned CWAL pointers, commit/push and verify remote checkpoint. Human controls PR initiation/review/merge.

## Verification evidence (recorded 2026-10-21)

**State:** COMPLETE, CHECKPOINTED. The edition-derived capability table is
replaced by bounded, read-only **observations**: every outcome is backed by the
check that was attempted, the status the endpoint answered and the reason. The
observations were measured against the real, isolated InfluxDB 2 OSS v2.9.1 and
InfluxDB 3 Core v3.12.1 containers.

**Environment:** branch `phase-2/registry-completion`; Go 1.27.3; Node.js 24.15.0
/ npm 11.12.1; Windows development host with a WSL Linux runtime; the isolated
containers ran under the WSL Docker daemon (29.3.1) with the same digest-pinned
images the harness uses.

### What changed

- `backend/internal/liveprobe/probe.go`: the edition switch that assigned
  `supported`/`unsupported` from the edition label is gone. A **check
  catalogue** of bounded read-only GET checks now decides where to look, and the
  observed response decides the outcome. Each check is recorded as an
  `Observation` (capability, check, method, path, status, outcome, detail) and
  returned with the result.
- Classification rules (documented in `docs/LIVE_PROBING.md`): 200 with the
  expected shape → `supported`; 200 with another shape → `unknown`; 404 on a
  **listing** check (and on every listing candidate) → `unsupported`; 404 on a
  **query** check → `unknown`; 405, 401/403, 400, 5xx, timeout, malformed body
  and an exhausted budget → `unknown`.
- `flux` and `line-protocol` are **not attempted** and stay `unknown` with the
  reason recorded ("needs a request body" / "a read-only probe never writes").
  A write protocol is only provable by an independently authorized synthetic
  write check, which this route does not perform.
- `backend/internal/registryapi/registryapi.go`: the probe response carries the
  `observations`; the request budget was raised to 10 so the catalogue fits
  (identification + up to six checks), still bounded.
- `ui/src/api/registry.ts`: `ApiProbeResult` carries the observations and
  `isApiProbeResult` validates them (a secret-like observation is rejected).
- `ui/src/features/databases/registryWorkflow.ts`: `probeCapabilityRows` maps a
  probe result to the three-state rows; `unknown` is carried through unchanged
  and never collapsed into `supported` or `unsupported`.
- `docs/LIVE_PROBING.md`, `docs/REGISTRY_API.md`, `docs/ROADMAP.md`: the
  edition projection is replaced by the check catalogue, the classification
  rules and the measured evidence matrix.

### Measured evidence matrix (real containers)

The integration test ran inside a container that shares the target's network
namespace, so it reached the server on its own loopback interface. The synthetic
token was read from a file, never from an argument or an environment value. Both
tests passed.

**InfluxDB 2 OSS v2.9.1** (4 requests, `edition=influxdb-2.x-oss`):

| Capability | Check | Path | Status | Outcome |
| --- | --- | --- | --- | --- |
| influxql | `influxql-v2-query` | `/api/v2/query?q=SELECT%201` | 405 | unknown — the endpoint does not serve GET |
| sql | `sql-v3-query` | `/api/v3/query/sql?query=SELECT%201` | 200 (HTML page) | unknown — unexpected response shape |
| schema_discovery | `schema-v2-buckets` | `/api/v2/buckets` | 200 | **supported** — the buckets listing was observed |
| flux | `flux-query` | not attempted | — | unknown — the check needs a request body |
| line-protocol | `line-protocol-write` | not attempted | — | unknown — a read-only probe never writes |

**InfluxDB 3 Core v3.12.1** (7 requests, `edition=influxdb-3.x-core`):

| Capability | Check | Path | Status | Outcome |
| --- | --- | --- | --- | --- |
| influxql | `influxql-v2-query` | `/api/v2/query?q=SELECT%201` | 404 | unknown — not served for GET |
| influxql | `influxql-v3-query` | `/api/v3/query/influxql?query=SELECT%201` | 404 | unknown — not served for GET |
| sql | `sql-v3-query` | `/api/v3/query/sql?query=SELECT%201` | 404 | unknown — not served for GET |
| sql | `sql-v2-query` | `/api/v2/query/sql?query=SELECT%201` | 404 | unknown — not served for GET |
| schema_discovery | `schema-v2-buckets` | `/api/v2/buckets` | 404 | unsupported — the path is not served |
| schema_discovery | `schema-v3-databases` | `/api/v3/configure/database?format=json` | 200 | **supported** — the databases listing was observed |
| flux | `flux-query` | not attempted | — | unknown — the check needs a request body |
| line-protocol | `line-protocol-write` | not attempted | — | unknown — a read-only probe never writes |

The previous behaviour claimed `influxql`/`flux`/`sql` support and
`line-protocol` support from the edition label. The measured result is that a
GET-only probe can **observe** the listing capability and nothing else; the
dialects and the write protocol stay **unknown** with the reason recorded.

### Tests

- `backend/internal/liveprobe/observation_test.go` (new): every check is
  recorded; the edition label never grants a capability; an auth failure, a
  malformed body, a server error, a timeout and an exhausted budget are all
  `unknown`; a write protocol is never proven; no observation carries a
  credential.
- `backend/internal/liveprobe/probe_test.go`: the 2.x and 3.x fixtures now
  answer like the real deployments and assert the evidence-based outcomes.
- `backend/internal/liveprobe/integration_test.go`: the live assertions now
  require the dialects and the write protocol to be `unknown` and
  `schema_discovery` to be `supported`, and the test prints the observation
  matrix.
- `ui/src/features/databases/probePresentation.test.ts` (new): the three states
  are carried through, only an observed capability is confirmed, and the
  observations behind a capability are exposed.
- `ui/src/api/registry.live.test.ts`: the probe-result validator now covers the
  observations and rejects a malformed one.

### Commands and actual outcomes

- `gofmt -l .` → clean; `go vet ./...` → pass; `go vet -tags integration ./...`
  → pass; `go build ./...` → pass.
- **linux/amd64 (the CI platform), full backend suite → every package `ok`:**
  `authz`, `fixtures`, `httpapi`, `liveprobe`, `metadatastore`, `registryapi`,
  `registrycontract`, `registryservice`, `secretstore`, `synccontract`.
- `go test -race ./internal/liveprobe/...` → `ok` (the liveprobe package passes
  in full under the race detector).
- Frontend: `npm run lint` → pass; `npx tsc -b` → pass; `npm test` → **16 files
  / 155 tests passed** (was 15 / 152).
- `scripts/policy-check.sh` → **PASS** (exit 0).
- Relative markdown link check over every changed `.md` → **0 missing links**.
- Live integration test against the real containers → **PASS** for InfluxDB 2
  OSS and InfluxDB 3 Core (matrix above).

### Isolation level

- The probe is unchanged in its safety envelope: GET only, no write, the
  endpoint resolved server-side from the stored profile, the credential resolved
  server-side and used only to authenticate, the peer pinned to the validated
  addresses with TLS verification and no proxy, redirects refused, and the
  response size, deadline and request budget bounded.
- The verification used only the isolated, synthetic InfluxDB containers with
  loopback ports and throwaway credentials. No production historian, credential
  or Docker privilege was touched; the containers and the generated secrets were
  removed after the run.

### Refined contract compliance (P2C-003)

| Refined requirement | How it is met |
| --- | --- |
| Replace edition-derived assertions with specific nonmutating observations, recording what was attempted, the status and the outcome | The check catalogue + `Observation` records; the edition switch is removed |
| A 200, an edition label, a 401/403 or an unrecognized response is not proof | 200 without the expected shape → `unknown`; the edition grants nothing; 401/403 → `unknown` |
| Auth errors, timeouts, malformed/ambiguous data and untested functionality yield `unknown` | All four map to `unknown` with the reason recorded |
| The prober stays GET/read-only; a write protocol without an authorized synthetic write check is `unknown` | The prober is GET-only; `flux` and `line-protocol` are not attempted and stay `unknown` |
| Preserve profile-based selection, the approved binding and the pinned peer/TLS/SSRF checks on every request | Unchanged; the probe still dials only the validated peers and re-verifies the binding before resolving a credential |
| The API wire format, the UI three-state mapping and the docs agree | The response carries `observations`; the UI validator and the presentation helper keep the three states; the docs carry the catalogue, the rules and the matrix |
| Verify against synthetic isolated InfluxDB 2 OSS / 3 Core; test untrusted/mixed DNS, auth denied, timeouts, redirects, inconclusive responses and edition variation | The live matrix above plus the unit tests for mixed DNS, auth denial, timeouts, redirects and inconclusive responses; 1.x stays unverified |

### Limitations

- **A GET-only probe cannot prove a query dialect.** The verified editions serve
  a query endpoint only for POST, so the dialects stay `unknown`. A read-only
  query check (a POST carrying a read query) could prove them, but changing the
  probe's method set is a separate, explicit decision.
- **A write protocol is never proven by this route.** It stays `unknown` until
  an independently authorized synthetic write check exists.
- The schema-discovery evidence is a namespace-level listing (buckets on 2.x,
  databases on 3.x), not a measurement-level schema listing.
- The isolated harness scripts could not run on this host: the environment's
  Docker CLI has no Compose v2 plugin, so the containers were started directly
  with `docker run` using the same digest-pinned images, the same loopback
  ports and synthetic credentials. The repository harness was not modified.
- The `go test -race` run is Windows-only; a race build for linux/amd64 needs a
  Linux C toolchain that this host does not provide.
- InfluxDB 1.x remains unverified; ADR-0004 and ADR-0005 remain **Proposed**.
