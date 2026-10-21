# Live Capability Probing

Status: **Implemented (read-only, consented, SSRF-protected).**
Task: P2L-004.

This document describes the live capability prober that replaces version-based
guessing with real, read-only observation. It covers the endpoint policy, DNS and
peer checks, TLS defaults, request budget and the exact coverage that has been
verified against real InfluxDB containers.

## Principle

Capabilities are **probed per connection** and never inferred from a version
label ([`DATABASE_REGISTRY.md`](DATABASE_REGISTRY.md)). A probe outcome is one of
three values — `supported`, `unsupported` or `unknown` — and `unknown` is never
treated as support.

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

## Edition identification and capability mapping

Identification uses read-only responses, not a version guess:

| Observation | Edition |
| --- | --- |
| `/health` 200 with `"name":"influxdb"` and `"version":"v2…"`, or `"status":"pass"` | `influxdb-2.x-oss` |
| authenticated `/health` returns plain text `OK`, or JSON with `"version":"v3…"` | `influxdb-3.x-core` |
| anonymous `/health` 401 with a credential supplied, confirmed via `/api/v3/configure/database` | `influxdb-3.x-core` |

The capability vocabulary is then projected from the identified edition, never
from a version string alone:

| Edition | influxql | flux | sql | line-protocol | schema discovery |
| --- | --- | --- | --- | --- | --- |
| 2.x OSS | supported | supported | unsupported | supported | supported |
| 3.x Core | supported | unsupported | supported | supported | unsupported |

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
  0 failed**, Toolkit smoke 2 passed / 1 not implemented.
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

## Limitations

- The probe budget counts HTTP requests, not wall-clock operations.
- Edition identification relies on the `/health` shape; an edition that changes
  its health payload would be reported as unidentified rather than misidentified.
- InfluxDB 1.x is unverified (no isolated instance).
- Capability mapping is a small, explicit table, not a full compatibility matrix
  (ADR-E is not yet proposed).
- Only Linux amd64 is verified.
