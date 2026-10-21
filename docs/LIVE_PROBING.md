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

### DNS and peer checks

Before any request, the hostname is **resolved** and **every** returned address
is classified. A hostname that resolves to even one denied address is refused,
which closes the DNS-rebinding gap where one lookup returns an allowed address
and a later lookup a denied one. The resolver is injectable so this is unit
tested without real DNS.

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
- `go test ./... -count=1` → all packages `ok` (13 liveprobe unit tests plus the
  rest).
- `make check` → exit 0 (backend + frontend + policy).
- `./scripts/run-all.sh --teardown` (harness) → exit 0; InfluxDB **16 passed /
  0 failed**, Toolkit smoke 2 passed / 1 not implemented.
- `./scripts/live-probe.sh` (read-only probes) → exit 0:
  - InfluxDB 2 → `edition=influxdb-2.x-oss`, `dialects=[flux influxql sql]`,
    `write=[line-protocol:supported]`, `schemaDiscovery=supported`, requests=1.
  - InfluxDB 3 → `edition=influxdb-3.x-core`, `dialects=[flux influxql sql]`,
    `write=[line-protocol:supported]`, `schemaDiscovery=unsupported`, requests=1.
  - Default (production) policy on the loopback endpoint → refused
    (`tls-validation-failed`), proving the exception is test-only.

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

## Limitations

- The probe budget counts HTTP requests, not wall-clock operations.
- Edition identification relies on the `/health` shape; an edition that changes
  its health payload would be reported as unidentified rather than misidentified.
- InfluxDB 1.x is unverified (no isolated instance).
- Capability mapping is a small, explicit table, not a full compatibility matrix
  (ADR-E is not yet proposed).
- Only Linux amd64 is verified.
