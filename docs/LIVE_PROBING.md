# Live Capability Probing

Status: **Implemented (read-only, consented, SSRF-protected, observation-based).**
Tasks: P2L-004 (live prober), P2L-006 (DNS/peer pinning hardening), P2L-007 (administrator-only credential binding), P2C-003 (observation-based capabilities).
Merged into `main` via PR #6 on 2026-10-21 (merge commit `0ba4827`); the observation-based capability model is added by P2C-003.

This document describes the live capability prober that replaces version-based
guessing with real, read-only observation. It covers the endpoint policy, DNS and
peer checks, TLS defaults, request budget, the capability check catalogue and
the exact coverage that has been verified against real InfluxDB containers.

## Principle

The running **edition** is identified from real read-only responses for each
connection, never from a caller-supplied version label
([`DATABASE_REGISTRY.md`](DATABASE_REGISTRY.md)). A probe outcome is one of
three values — `supported`, `unsupported` or `unknown` — and `unknown` is never
treated as support.

**No capability is asserted from the edition label.** Every outcome is backed by
an **observation**: the check that was attempted, the HTTP status the endpoint
answered and the reason for the outcome. The observations are returned with the
probe result (`observations`), so every claim can be audited.

**What the read-only probe can and cannot prove (P2C-003).** A GET-only probe
can prove a capability when a *listing* endpoint answers with the expected
response shape. It cannot prove a **query** dialect: the verified editions serve
a query endpoint only for POST, so a GET answer (405, 404, an HTML page) shows
that the read-only check could not be completed — never that the dialect is
absent. A **write protocol** is never proven, because a read-only probe does not
write. Both cases are reported as `unknown` with the observation attached, never
as `supported` or `unsupported`.

> **Correction under review (P3-001, 2026-10-26).** The statement above — that
> the verified editions serve a query endpoint only for POST — is **not accurate
> for every route**. A read-only GET survey of the pinned containers found that
> InfluxDB 2 OSS v2.9.1 answers the 1.x-compatibility `GET /query` (with the DBRP
> database name) and InfluxDB 3 Core v3.12.1 answers `GET /api/v3/query_influxql`
> and `GET /api/v3/query_sql`, returning real metadata. The catalogue checks
> different paths (`/api/v3/query/sql`, `/api/v3/query/influxql`), which return
> 404, so its `unknown` outcome is a path defect rather than proof that the
> endpoint is POST-only. The capability model is unchanged until the catalogue is
> corrected under explicit authority; see
> [`docs/decisions/phase-3-schema-discovery-gate.md`](decisions/phase-3-schema-discovery-gate.md).

## Read-only guarantee

A probe issues **GET requests only**. It never writes, deletes, updates or
configures anything on the target. The integration tests assert the method of
every request the prober makes.

## Endpoint policy (SSRF)

The zero-value `Policy` is deny-by-default:

- scheme must be `http` or `https`; the URL must be absolute and must not embed
  credentials (`user:pass@`);
- `https` is required unless `AllowInsecureTLS` is set, and disabling
  verification always requires a documented `OverrideReason`;
- loopback, link-local, unspecified and private addresses are denied unless
  `AllowLoopback`/`AllowPrivate` is set, which also requires a documented
  `OverrideReason`;
- cloud metadata hosts (`metadata.google.internal`, `169.254.169.254`, …) are
  denied by default.

### DNS and peer checks (hardened, P2L-006 F-01)

Before any request, the hostname is **resolved once** and **every** returned
address is classified. A hostname that resolves to even one denied address is
refused.

The probe then builds a per-probe HTTP client bound to the **exact validated
addresses**:

- the dialer connects only to a pinned IP and **never re-resolves** the
  hostname, so a DNS answer that changes after validation cannot redirect the
  connection (closes the rebinding gap);
- a `net.Dialer.Control` hook re-validates the concrete peer address immediately
  before the socket connects, so a mis-pinned dial still fails closed;
- TLS uses the endpoint hostname for SNI and certificate verification, which is
  **never disabled** (`InsecureSkipVerify` is never set);
- the transport uses **no proxy**, so `HTTP_PROXY` cannot tunnel the request to
  an unvalidated host;
- redirects are refused.

The resolver is injectable, so this is unit tested without real DNS, including a
resolver that returns a different (denied) answer on a second call.

### Redirects

The HTTP client **does not follow redirects**. A validated endpoint therefore
cannot redirect the probe to a different, unvalidated host.

### Test-only exception

`TestPolicy()` permits loopback and plain http **only** for the isolated
`testing/influxdb/` harness, and carries an explicit override reason. It is never
a production default. The backend enables it only when the operator passes
`-allow-test-probe-policy`, and logs that the test policy is active.

## Bounds

- Every probe requires a **positive deadline** and a **positive request budget**.
- The API caps the deadline at 30 seconds and defaults to 10.
- Response bodies are bounded to 1 MiB.

## Authorization and credential handling

- The probe route (`POST /api/v1/registry/probe`) requires the `profile.probe`
  permission. A viewer is refused; an operator, configurator or administrator is
  allowed. An anonymous request is refused with 401.
- The request body carries **only a connection profile ID** (and an optional
  deadline). The endpoint and the credential reference are resolved
  **server-side from the stored profile**; the caller cannot supply either, so a
  principal with `profile.probe` cannot forward a stored secret to an origin of
  its choosing (P2L-006 F-02). An unknown field such as `endpoint` or
  `credential_ref_id` is rejected.
- The credential is resolved **server-side** through the ADR-0003 secret store
  and used only to authenticate the request. It is never returned, logged or
  embedded in an error or a probe result.
- A successful probe is recorded as an audited `profile.probe` action attributed
  to the acting principal.

## Protected authorization verification (P2C-007)

`Prober.VerifyCredential` proves a candidate credential is **authorized** by a
deployment, using a protected, read-only listing - the same listings the
capability catalogue uses for schema discovery - as the authorization check:

| Edition | Protected check | Request |
| --- | --- | --- |
| `influxdb-2.x-oss` | `schema-v2-buckets` | `GET /api/v2/buckets` |
| `influxdb-3.x-core` | `schema-v3-databases` | `GET /api/v3/configure/database?format=json` |

The procedure, all read-only GETs:

1. The endpoint policy, DNS resolution and pinned-peer controls are applied
   exactly as a capability probe applies them, so a verification cannot be
   redirected to a host the policy denies.
2. The running edition is identified first (the table above); the protected
   check is chosen from what the deployment actually is. A deployment that does
   not identify as a supported edition cannot be verified at all.
3. An **anonymous** request to the protected listing must be refused (401/403).
   A listing served to anonymous callers authenticates nothing, so no answer it
   gives can prove a candidate is authorized - an anonymous answer is a
   fail-closed verdict, not a pass.
4. The **candidate** request must be accepted (200 with the expected body
   shape). A denied or inconclusive answer is a fail-closed verdict.

The verification is what the credential replacement lifecycle uses to validate
a staged candidate before any swap (see [`REGISTRY_API.md`](REGISTRY_API.md)).
An error is returned only when the check could not be attempted at all; a
completed check reports a verdict, and only `Authorized == true` means the
candidate proved itself. A public `/health` edition identification is
deliberately **not** among the positive evidence: on InfluxDB 2 it succeeds
without a credential and previously let an unauthorized replacement pass
validation.

What the verification proves is **authenticated read access**. Whether a
candidate also holds write or admin privileges cannot be proven read-only
(that needs a write, which no verification ever performs), so a replacement
never claims more than authenticated read access.

## Edition identification and capability checks (P2C-003)

Identification uses read-only responses, not a version guess:

| Observation | Edition |
| --- | --- |
| `/health` 200 with `"name":"influxdb"` and `"version":"v2…"`, or `"status":"pass"` | `influxdb-2.x-oss` |
| authenticated `/health` returns plain text `OK`, or JSON with `"version":"v3…"` | `influxdb-3.x-core` |
| anonymous `/health` 401 with a credential supplied, confirmed via `/api/v3/configure/database` | `influxdb-3.x-core` |

The edition label identifies the product. It grants **no** capability.

### The check catalogue

Capabilities are decided by a catalogue of **bounded, read-only GET checks**.
The catalogue decides *where* to look; the observed response decides the
outcome. A check that answers conclusively ends that capability's checks, which
keeps the request budget small.

| Capability | Check | Request | Positive evidence |
| --- | --- | --- | --- |
| `influxql` | `influxql-v2-query` | `GET /api/v2/query?q=SELECT%201` | 200 with `"results"` |
| `influxql` | `influxql-v3-query` | `GET /api/v3/query/influxql?query=SELECT%201` | 200 with `"results"` |
| `sql` | `sql-v3-query` | `GET /api/v3/query/sql?query=SELECT%201` | 200 with `"results"` |
| `sql` | `sql-v2-query` | `GET /api/v2/query/sql?query=SELECT%201` | 200 with `"results"` |
| `schema_discovery` | `schema-v2-buckets` | `GET /api/v2/buckets` | 200 with `"buckets"` |
| `schema_discovery` | `schema-v3-databases` | `GET /api/v3/configure/database?format=json` | 200 with `iox::database` |
| `flux` | `flux-query` | not attempted | the check needs a request body |
| `line-protocol` | `line-protocol-write` | not attempted | a read-only probe never writes |

### How an answer is classified

| Observed answer | Outcome | Why |
| --- | --- | --- |
| 200 with the expected shape | `supported` | the capability was observed |
| 200 with another shape (for example an HTML page) | `unknown` | the answer is not evidence of the capability |
| 404 on a **listing** check, and every listing candidate answered the same way | `unsupported` | the deployment exposes no such listing |
| 404 on a **query** check | `unknown` | the verified editions serve a query endpoint only for POST, so a GET 404 does not prove the dialect is absent |
| 405 | `unknown` | the endpoint does not serve this method |
| 401 / 403 | `unknown` | the probe credential was denied |
| 400, 5xx, timeout, malformed body | `unknown` | the check could not be completed |
| the request budget was exhausted | `unknown` | the check was not attempted |

Every check is recorded as an **observation** (`capability`, `check`, `method`,
`path`, `status`, `outcome`, `detail`) and returned with the probe result, so a
claim can always be traced to the answer that produced it.

Anything that does not identify as a supported deployment is reported as a
sanitized `endpoint-rejected` error, not guessed.

## Security hardening (P2L-006)

Two HIGH-priority pre-PR findings were fixed and are regression-tested:

- **F-01 — DNS rebinding / peer mismatch.** The prober previously validated one
  DNS lookup but let the HTTP client resolve again at dial time. It now resolves
  once, pins the validated addresses, dials only those addresses, re-validates
  the concrete peer in a `Control` hook, pins TLS SNI/hostname verification, uses
  no proxy and refuses redirects. Regression tests include a resolver that
  changes its answer between validation and dial, multi-answer responses with a
  denied address, an unpinned-peer dial, TLS hostname verification and
  proxy/redirect paths.
- **F-02 — credential forwarding.** The probe route previously accepted a
  caller-supplied `endpoint` and `credential_ref_id` independently, so a
  principal with `profile.probe` could send a stored secret to an arbitrary host.
  The route now accepts only a connection profile ID and resolves both the
  endpoint and the credential reference server-side from that stored profile. A
  caller-supplied `endpoint` or `credential_ref_id` is rejected as an unknown
  field. Regression tests cover an attacker-chosen endpoint, cross-profile
  credential substitution, an unknown profile, role-based authorization
  (anonymous/viewer/operator/administrator) and absence of any leaked token in
  responses.

Production-default SSRF denial (private, metadata, loopback) is unchanged; the
loopback/insecure-TLS exception remains opt-in for the isolated test harness
only.

### Credential-to-profile binding (P2L-007)

P2L-006 stopped a caller from overriding the probe endpoint/credential, but
profile creation still accepted an arbitrary `credential_ref_id` together with an
arbitrary endpoint. A `configurator` (which can list references and probe) could
therefore attach another principal's stored secret to an endpoint it controlled
and cause credential forwarding.

The fix binds a credential to a trusted profile identity **server-side**:

- attaching a credential reference requires the administrator-only
  `credential.attach` permission; a profile carrying a reference is refused with
  403 for any other principal;
- an approved binding records the credential reference, connection ID, endpoint
  and canonical scope, and is enforced at profile creation and re-verified at
  probe time;
- a binding that does not exactly match the profile's endpoint/scope is rejected,
  so a create/delete/recreate with a changed endpoint or scope, a stale
  reference, or forged scope fields cannot reuse an approval;
- a profile may exist unbound (no credential reference) and then resolves no
  credential.

This does not depend on browser checks or the public-host SSRF policy alone. The
adversarial regression (`registryapi/binding_test.go`) reproduces the exact path
— a configurator enumerates a reference, attempts to attach it to an attacker
endpoint, and probes — and asserts the create is refused (403), the attacker host
receives **zero** Authorization headers, and the profile is not created.

## Coverage and unverified editions

- **Verified live:** InfluxDB **2 OSS v2.9.1** and **3 Core v3.12.1** against the
  PR #5 pinned containers in `testing/influxdb/`.
- **Unverified:** **InfluxDB 1.x has no real instance and is explicitly
  unverified.** The prober does not claim 1.x coverage. Adding 1.x would require
  a real isolated 1.x instance and a separate decision.

## Verification evidence (recorded 2026-10-21)

**Environment:** branch `phase-2/live-registry-implementation`; Go 1.27.3;
Docker Engine 29.9.1 with Compose v5.6.0. Pinned images unchanged:
`influxdb:2.9.1@sha256:cdd3dbde…716eaaac` and
`influxdb:3.12.1-core@sha256:1c8200f7…ab6bd406`.

**Commands and actual outcomes:**
- `gofmt -l .` → clean; `go vet ./...` (and `-tags integration`) → pass;
  `go build ./...` → pass.
- `go test ./... -count=1` → all packages `ok` (liveprobe unit + security tests
  plus the rest).
- `go test -race ./... -count=1` → all packages `ok`.
- `make check` → exit 0 (backend + frontend + policy).
- `./scripts/run-all.sh --teardown` (harness) → exit 0; InfluxDB **16 passed /
  0 failed**, Toolkit smoke 2 passed / 1 not registered. The third smoke check
  is an unauthenticated `GET /api/v1/registry/profiles`: the route exists in
  source since PR #6, but the registry data routes are **not registered unless
  an authentication token file is configured**, so the request answers 404. The
  authenticated surface is covered by `toolkit-live-registry.sh`.
- `./scripts/live-probe.sh` (read-only probes, re-run after the P2L-006
  hardening) → exit 0:
  - InfluxDB 2 → `edition=influxdb-2.x-oss`, `dialects=[flux influxql sql]`,
    `write=[line-protocol:supported]`, `schemaDiscovery=supported`, requests=1.
  - InfluxDB 3 → `edition=influxdb-3.x-core`, `dialects=[flux influxql sql]`,
    `write=[line-protocol:supported]`, `schemaDiscovery=unsupported`, requests=1.
  - Default (production) policy on the loopback endpoint → refused, proving the
    exception is test-only.
- `./scripts/toolkit-live-registry.sh` (e2e) → exit 0 (8 checks): anonymous 401;
  admin create 201; listing; viewer 403; live probe `edition=influxdb-2.x-oss`
  with `mock=false`; caller-supplied endpoint rejected 400; profile survives
  restart.

**Acceptance mapping (P2L-004):**
- Real read-only authenticated probes against pinned 2 OSS and 3 Core containers
  → the integration tests above.
- Explicit unsupported/unverified 1.x → no 1.x instance exists; coverage is not
  claimed.
- DNS/IP peer checks, TLS defaults, tight timeout/budget → unit tests
  (`TestProbeRefusesMixedDNSAnswers`, `TestDefaultPolicyDeniesPrivateAndInsecure`,
  `TestProbeHonoursRequestBudget`, `TestProbeHonoursDeadline`,
  `TestProbeDoesNotFollowRedirects`).
- No silent remote/private host allowance → `TestProbeRejectsPrivateTargetUnderDefaultPolicy`
  and the live default-policy rejection test.

**Acceptance mapping (P2L-006):**
- F-01 rebinding/peer → `TestDialIsPinnedToValidatedPeer`,
  `TestDialRejectsUnpinnedPeer`, `TestProbeRefusesMultiAnswerWithDeniedPeer`,
  `TestProbeRefusesUnspecifiedAndLinkLocal`, `TestProbeRefusesRedirectToDeniedHost`,
  `TestPinnedClientUsesEndpointSNI`, `TestPinnedClientTLSHostnameVerification`,
  `TestPinnedClientHasNoProxy`.
- F-02 credential forwarding → `TestProbeUsesProfileOwnEndpointAndCredential`,
  `TestProbeCannotForwardCredentialToArbitraryHost`,
  `TestProbeCannotSubstituteAnotherProfilesCredential`,
  `TestProbeRouteIgnoresCallerSuppliedEndpoint`, `TestProbeAuthorizationByRole`,
  `TestDeniedProbeLeaksNoCredential`, `TestProbeResultCarriesNoCredential`.

**Acceptance mapping (P2C-003):**
- Observations replace the edition projection → `liveprobe/observation_test.go`
  (`TestObservationsRecordWhatWasAttempted`,
  `TestEditionLabelNeverGrantsACapability`, `TestAuthDeniedIsUnknown`,
  `TestMalformedBodyIsUnknown`, `TestServerErrorIsUnknown`, `TestTimeoutIsUnknown`,
  `TestWriteProtocolIsNeverProven`, `TestBudgetExhaustionIsRecorded`,
  `TestObservationCarriesNoCredential`).
- The API, the UI and the docs agree on the three states → the probe response
  carries `observations`; `ui/src/api/registry.ts` validates them and
  `ui/src/features/databases/registryWorkflow.ts` maps them without collapsing
  `unknown` (`ui/src/features/databases/probePresentation.test.ts`).

### Measured evidence matrix (P2C-003, real containers)

The integration test was executed against the **real, isolated** InfluxDB 2 OSS
v2.9.1 and InfluxDB 3 Core v3.12.1 containers (the same digest-pinned images
the harness uses). The test binary ran inside a container that shares the
target's network namespace, so it reached the server on its own loopback
interface; the synthetic token was read from a file, never from an argument or
an environment value.

**InfluxDB 2 OSS v2.9.1** — 4 requests, `edition=influxdb-2.x-oss`:

| Capability | Check | Path | Status | Outcome |
| --- | --- | --- | --- | --- |
| influxql | `influxql-v2-query` | `/api/v2/query?q=SELECT%201` | 405 | unknown — the endpoint does not serve GET |
| sql | `sql-v3-query` | `/api/v3/query/sql?query=SELECT%201` | 200 (HTML page) | unknown — unexpected response shape |
| schema_discovery | `schema-v2-buckets` | `/api/v2/buckets` | 200 | **supported** — the buckets listing was observed |
| flux | `flux-query` | not attempted | — | unknown — the check needs a request body |
| line-protocol | `line-protocol-write` | not attempted | — | unknown — a read-only probe never writes |

**InfluxDB 3 Core v3.12.1** — 7 requests, `edition=influxdb-3.x-core`:

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

Both integration tests passed. The matrix is the honest result of a GET-only
probe: only the listing capability could be **observed**; every query dialect
stayed **unknown** because the verified editions serve a query endpoint only for
POST, and the write protocol stayed **unknown** because the probe never writes.

### Protected authorization verification against the real containers (P2C-007, recorded 2026-10-21)

`Prober.VerifyCredential` was exercised against the same digest-pinned,
isolated containers. It issues plain read-only GETs on the deployment's own
loopback address; no historian was written.

| Check | InfluxDB 2 OSS v2.9.1 | InfluxDB 3 Core v3.12.1 |
| --- | --- | --- |
| edition identification | `influxdb-2.x-oss` | `influxdb-3.x-core` |
| protected check | `schema-v2-buckets` | `schema-v3-databases` |
| anonymous request to the protected listing | **401** | **401** |
| request with the provisioned admin token | **200 → authorized** | **200 → authorized** |
| request with an invalid token | **401 → not authorized** ("the protected endpoint denied the candidate credential") | **fails closed** at edition identification — the deployment authenticates every request |
| production default policy against the harness endpoint | refused (`tls-validation-failed`) | refused (`tls-validation-failed`) |

The anonymous refusal is the load-bearing observation: it proves the listing
actually authenticates its callers, so the candidate's acceptance is evidence
of authorization rather than of reachability. On InfluxDB 2 the public
`/health` endpoint identifies the edition without any credential, which is
exactly why an edition label alone is not authorization.

**Commands and actual outcomes:** `gofmt -l .` → clean; `go vet ./...` and
`go vet -tags integration ./...` → pass; `go build ./...` and
`go build -tags integration ./...` → pass; `go test ./... -count=1`
(linux/amd64) → every package `ok`; `go test -race ./... -count=1`
(linux/amd64) → every package `ok`; `npm ci && npm run lint && npm run build
&& npm test` → 0 vulnerabilities, lint and build clean, 20 files / 188 tests
passed; `bash scripts/policy-check.sh` → **PASS**.

**Acceptance mapping (P2C-007):** `TestVerifyAcceptsAnAuthorizedCandidate`,
`TestVerifyRefusesAWrongCandidate`,
`TestVerifyFailsClosedWhenTheProtectedListingServesAnonymous`,
`TestVerifyFailsClosedWhenTheProtectedListingIsInconclusive`,
`TestVerifyAgainstAuthenticatedInfluxDB3`,
`TestVerifyRejectsUnsupportedAndInvalidEndpoints`,
`TestVerifyAppliesTheProductionEndpointPolicy`,
`TestVerifyFailsClosedWhenTheBudgetIsExhausted`,
`TestVerificationCarriesNoSecretValue`, plus the gated
`TestIntegrationVerifyCredentialAgainstInfluxDB2` and
`TestIntegrationVerifyCredentialAgainstInfluxDB3`.

**Environment limitation:** Docker 29's loopback port publishing is broken in
this WSL runtime (host traffic to a published loopback port is routed into the
special `loopback0` interface and dropped), so the gated tests ran from inside
each container's own network namespace, where `127.0.0.1:<service-port>` is the
service itself. The harness scripts and compose file are unchanged, the
test-only loopback policy is unchanged, and every request is still a plain
loopback GET.

## Limitations

- The probe budget counts HTTP requests, not wall-clock operations.
- Edition identification relies on the `/health` shape; an edition that changes
  its health payload would be reported as unidentified rather than misidentified.
- InfluxDB 1.x is unverified (no isolated instance).
- The capability checks are a small, explicit catalogue, not a full
  compatibility matrix (ADR-E is not yet proposed).
- **A GET-only probe cannot prove a query dialect as catalogued.** The dialects
  stay `unknown` in the capability model. P3-001 measured that the pinned
  editions answer *other* GET metadata query routes, so the catalogue and this
  conclusion are under correction (see the P3-001 note above); changing the
  catalogue changes published capability outcomes and needs explicit
  authorization.
- **A write protocol is never proven by this route.** `line-protocol` stays
  `unknown` until an independently authorized synthetic write check exists.
- The schema-discovery evidence is a **namespace-level listing** (buckets on 2.x,
  databases on 3.x), not a measurement-level schema listing.
- Only Linux amd64 is verified.
