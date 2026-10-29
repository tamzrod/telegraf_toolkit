# P3-002 — Authorized source/destination schema discovery backend

> **State: HUMAN-AUTHORIZED ACTIVE WORK — COMPLETE, VERIFIED 2026-10-26** (archived to `archive/active-work/`). The approved Option A1 GET-only schema read is implemented for InfluxDB 2 OSS and 3 Core; the P2C-003 v3 catalogue correction is applied; InfluxDB 1.x remains unverified and the mapper UI is not implemented. This packet does not approve any proposed ADR or unsafe capability expansion.

## Purpose
Implement the approved safe, bounded, read-only backend schema discovery path using stored Registry profile identity and scope; reuse credential approval verification, role permissions and pinned peer/TLS/SSRF controls. For InfluxDB 2/3 discover actual measurements, field keys and tag keys distinctly where the approved dialect permits, with precise unsupported/unknown results. Query POST allowed only if P3-001 explicitly authorizes its read-only threat-model revision; no writes even as a probe.

## Acceptance criteria
Profile-only endpoint (no arbitrary URL/credential from browser), strong input/query injection defenses, bounded pagination/results/timeouts/request budget, cancellation, error redaction, role denial, scope change invalidation. Each returned observation must cite check/method/edition; unknown stays unknown; InfluxDB 1.x remains unverified.

## Verification
Go auth/SSRF/query-negative tests + isolated synthetic InfluxDB 2/3 end-to-end metadata checks, go race, make check.

## Dependency and boundaries
- Dependencies: P3-001 through P3-001.
- Existing Phase 2 Registry is authoritative; registered connection IDs, canonical scope and administrator-bound credential references are used server-side.
- Preserve P2L-006/007, P2C-007/008 SSRF, TLS/pinned peer, credential RBAC, lifecycle fencing and redaction.
- Only synthetic, isolated InfluxDB 2/3 instances; no production access, historian writes, pipeline deployment, Telegraf or sync runtime operations.
- Unknown/unverified is never presented as supported. InfluxDB 1.x not established.
- Any new architecture requiring human approval: record the option and report TASK_BLOCKED; do not self-approve ADR decisions.

## CWAL lifecycle
Execute only CURRENT. On verified acceptance, archive with `git mv` into `archive/active-work/`, update `handoff.md` and Active Work README to designate the next task, commit/push and verify remote SHA. Continue without intermediate approval when within existing approved boundaries. Stop with TASK_BLOCKED when blocked. On P3-006 success clear both pointers and report WORK_PACKAGE_READY. **No agent PR creation, approval or merge.**


## Resolution

P3-002 is complete and verified. It delivered the approved Option A1 read path
and the approved P2C-003 catalogue correction; it did not start the mapper
increment or any later queue task.

1. **Dedicated permission.** `authz.PermSchemaRead = "schema.read"` is granted to
   the operator, configurator and administrator roles and denied to viewer. It is
   distinct from `profile.probe`; the role matrix and distinctness are tested
   (`backend/internal/authz`).
2. **Bounded read adapter.** `liveprobe.ReadSchema`
   (`backend/internal/liveprobe/schema.go`) executes only the fixed statement
   allowlist over GET. It identifies the edition from an observed `/health`
   response, resolves a 2.x bucket/DBRP server-side with read-only
   `GET /api/v2/buckets` and `GET /api/v2/dbrps` (never creating a mapping) and
   queries `GET /query`; on 3.x it queries
   `GET /api/v3/query_influxql`. Field/tag reads require exact membership in the
   observed measurement list before the statement is built, and the identifier is
   escaped. The result carries the connection identity, canonical scope,
   observed edition, observations, truncation, request count and read time.
3. **Authorized HTTP surface.** `GET /api/v1/registry/schema/measurements`,
   `…/schema/fields` and `…/schema/tags` accept only `connection_id` (plus
   `measurement` for fields/tags). Unknown or repeated parameters are refused
   with 400. The endpoint, scope and credential come from the stored profile; the
   approved binding is re-verified at read time, and a configured but invalid or
   revoked credential fails closed with no anonymous fallback. One redacted
   `schema.read.<kind>` audit event references the profile identity.
4. **Catalogue correction.** The InfluxDB 3 catalogue checks now use the measured
   GET routes (`/api/v3/query_influxql?q=SHOW%20DATABASES`,
   `/api/v3/query_sql?db=_internal&q=SELECT%201`). The historical P2C-003 matrix
   is preserved in `docs/LIVE_PROBING.md`; the correction is appended, not
   rewritten. The 2 OSS `/api/v2/query` 405 remains `unknown`, and `sql-v2` is
   unchanged.
5. **Evidence and harness.** Unit, security and HTTP tests; a gated live test
   (`TestIntegrationSchemaDiscoveryAgainstInfluxDB2/3`); and
   `testing/influxdb/scripts/schema-discovery.sh`.

### Acceptance criteria → evidence

| Criterion | Implemented behaviour | Evidence |
| --- | --- | --- |
| Profile-only endpoint; no arbitrary URL/credential | The route fixes the kind; only `connection_id` and (fields/tags) `measurement` are accepted. Endpoint, scope and credential are read server-side from the stored profile. | `TestSchemaRouteRejectsCallerOverridesAndUnknownParameters`, `TestSchemaRouteUnknownProfile` |
| Strong input/query injection defenses | Fixed statement allowlist; exact measurement-membership check; InfluxQL identifier escaping; strict parameter parsing. No caller statement can reach a request. | `TestSchemaOnlyIssuesAllowlistedStatements`, `TestSchemaSelectIntoAttemptIsRefused`, `TestSchemaInjectionAttemptIsRefusedBeforeAnyStatement`, `TestSchemaRouteRefusesInjectionAndUnknownMeasurement` |
| Bounded results, timeouts, request budget, pagination | 15 s total deadline, 5 s per request, 8-request budget, 1 000-row cap, 1 MiB response cap, explicit `truncated` state. `SHOW` has no offset pagination; bounded caps and truncation are the implemented bound. | `TestSchemaTransportTimeoutIsUnknown`, `TestSchemaOversizedResponseIsTruncated`, `TestSchemaTruncationIsExplicit`, `TestSchemaBudgetExhaustionIsUnknown` |
| Cancellation | Every request derives from the request context and a bounded per-request deadline; the total read has a deadline. | `TestSchemaCancellationIsBounded` |
| Error redaction | Sanitized errors only; no credential value, credential-bearing URL or raw body is surfaced or logged. | `TestSchemaCarriesNoCredential`, `TestSchemaRouteDeniedWhenBindingMissing`, `TestSchemaRouteRevokedConfiguredCredentialFailsClosed`, `TestSchemaTLSVerificationFailureNeverLeaksAndIsNotSupport` |
| Role denial | `schema.read` required; viewer is refused 403; the check runs before the profile is read. | `TestSchemaRouteAuthorizationByRole` |
| Scope change invalidation | The approved binding is re-verified for the exact profile identity; the response repeats `connection_id`, canonical `scope` and observed `edition`; a concurrent scope deletion cannot yield a stale new-scope answer. | `TestSchemaRouteDeniedWhenBindingMissing`, `TestSchemaRouteScopeChangeDuringConcurrentReadsIsSafe`, `TestSchemaScopeEditionMismatchIsUnknown`, `TestSchemaScopeKindsMatchFixtures` |
| Each observation cites check/method/edition | Observations carry `check`, `method`, `path`, `status`, `edition`, `outcome` and sanitized `detail`; a non-request step carries `method: none` and `status: 0`. | `TestSchemaRouteReadsMeasurementsFromStoredProfile`, liveprobe outcome tests |
| Unknown stays unknown | 401/403, 404, 405, timeout, 5xx, transport failure, TLS failure, redirect, unknown edition and malformed bodies stay `unknown`; a query-route 404 is never `unsupported`. | `TestSchema404IsUnknownNotUnsupported`, `TestSchemaDeniedCredentialIsUnknown`, `TestSchemaUnknownEditionIsUnknownNotUnsupported`, `TestSchemaRedirectIsNotFollowed`, `TestSchemaTransportTimeoutIsUnknown` |
| InfluxDB 1.x remains unverified | A database/retention-policy scope returns `unknown` with `schema-1x-unverified` and makes no request. | `TestSchemaInfluxDB1xMakesNoRequest`, `TestSchemaRouteInfluxDB1xIsUnknownWithoutARequest` |
| Inherited SSRF/TLS/pinned-peer controls | Reuses `validateEndpoint`, one DNS resolution with peer classification, `newPinnedClient` (no proxy, refused redirects, verified TLS, pinned peers) and the configured policy. | `TestSchemaRouteRefusesDeniedEndpointUnderDefaultPolicy`, `TestSchemaRedirectIsNotFollowed`, `TestSchemaTLSVerificationFailureNeverLeaksAndIsNotSupport` |
| No writes | Only the three approved `SHOW` GET statements exist; assertions require every deployment request to be GET. | `TestSchemaRouteReadsMeasurementsFromStoredProfile`, `TestSchemaConcurrentReadsAreIsolated`, live integration |
| Redacted audit | One event per completed read: actor, `schema.read.<kind>`, profile target, succeeded outcome; no rows, values or credential. | `TestSchemaRouteRecordsARedactedAuditEvent` |

### P2C-003 catalogue correction (measured 2026-10-26)

| GET request | Status | Observed |
| --- | --- | --- |
| `/api/v2/query?q=SHOW%20MEASUREMENTS` (2 OSS) | 405 | `allow: OPTIONS, POST` — unchanged, dialect stays `unknown` |
| `/query?db=ttk-test-bucket&q=SHOW%20MEASUREMENTS` (2 OSS) | 200 | `environment`, `sensor` |
| `/api/v3/query_influxql?q=SHOW%20DATABASES` (3 Core) | 200 | `[{"iox::database":"_internal",…}]` |
| `/api/v3/query_sql?db=_internal&q=SELECT%201` (3 Core) | 200 | `[{"Int64(1)":1}]` |
| `/api/v3/query/influxql?…` (old catalogue path) | 404 | `Not found` |
| `/api/v3/query/sql?…` (old catalogue path) | 404 | `Not found` |

Applied result: on the pinned 3 Core, `influxql` and `sql` are observed as
`supported` through the corrected GET routes; on 2 OSS both remain `unknown`.
The probe still issues GET only, and a denial, 404, 405, timeout or unexpected
body stays `unknown`. `TestCatalogueUsesMeasuredV3QueryRoutes` and the updated
synthetic 3.x fixture pin the paths.

### Changed surfaces

- `backend/internal/liveprobe/schema.go` (new bounded read adapter)
- `backend/internal/liveprobe/schema_test.go`, `schema_security_test.go` (new)
- `backend/internal/liveprobe/probe.go`, `probe_test.go` (v3 catalogue correction, bounded-flag reader)
- `backend/internal/registryapi/registryapi.go` (three GET routes, handler, error mapping, audit)
- `backend/internal/registryapi/schema_test.go`, `schema_integration_test.go` (new)
- `backend/internal/registryapi/registryapi_test.go` (store-exposing test helper)
- `backend/internal/authz/authz.go`, `authz_test.go` (`schema.read` and role grants)
- `backend/internal/registrycontract/contract.go` (`measurement-not-found`)
- `backend/internal/registryservice/service.go` (`RecordSchemaRead`)
- `testing/influxdb/scripts/schema-discovery.sh`, `testing/influxdb/README.md`
- `docs/LIVE_PROBING.md` (marked correction + read contract)
- `docs/REGISTRY_API.md` (permission, routes, schema section)
- `docs/contracts/canonical-mapper.md`, `docs/contracts/README.md`
- `docs/ROADMAP.md`, `docs/CANONICAL_MAPPING.md`, `README.md`
- `ICC/context/project.md`
- `handoff.md`, `workflow/active_work/README.md` (pointer advance after archive)

### Verification (executed 2026-10-26)

| Check | Command | Result |
| --- | --- | --- |
| Format | `cd backend && gofmt -l .` | clean |
| Vet | `cd backend && go vet ./...` | PASS |
| Backend tests | `cd backend && go test ./... -count=1` | PASS (all packages) |
| Race | `cd backend && go test -race ./internal/liveprobe/ ./internal/registryapi/ ./internal/authz/ ./internal/registryservice/ -count=1` | PASS |
| Full repository gate | `make check` (gofmt, vet, build, tests, UI lint/build/test, policy check) | PASS; 188/188 UI tests in 20 files |
| Policy / secret scan | `bash scripts/policy-check.sh` | PASS (limited regex checks only) |
| Relative links | scripted check of the 9 touched docs | 96 relative links, 0 broken |
| Live editions | `GO_BIN=go ./testing/influxdb/scripts/schema-discovery.sh` | PASS both: 2.x and 3.x `measurements=[environment sensor] fields=1 tags=[host sensor_id] outcome=supported`; injection/absent → 404; viewer → 403 |
| Remote checkpoint | explicit-refspec push to `phase-3/canonical-mapper`, then `git ls-remote` | confirmed (SHA recorded in the checkpoint commit) |

The live read path issued GET requests only. The isolated containers hold
synthetic data and remained running with that state retained; nothing was
deployed and no historian was written.

### Limitations and honest notes

- **InfluxDB 1.x remains unverified.** A 1.x-scope profile is never contacted and
  returns `unknown` with an explicit `schema-1x-unverified` observation.
- **Only the fixed allowlist is executable.** There is no SQL or Flux statement
  read, no sample-point retrieval, no schema creation, no caller query and no POST
  route. "Unsupported" is not claimed from a negative observation; a 404 on a
  query route stays `unknown`.
- **No server pagination.** `SHOW` results are bounded by a row cap and an
  explicit truncation flag, not by offset pages.
- **Capability claims are edition/deployment-specific.** The 3 Core InfluxQL/SQL
  support is an observation on the pinned v3.12.1 container, not a version-wide
  guarantee. The 2 OSS query dialects remain `unknown`.
- **The catalogue correction is bounded to the tested routes.** It does not
  generalize to other editions or deployments.
- **No mapper feature.** The read path is implemented; the browser schema
  browser, mapping drafts, canonical-schema versioning, export and downstream
  consumption do not exist yet.
- **No PR.** The work is checkpointed on `phase-3/canonical-mapper`; PR creation,
  approval and merge remain human-only.
