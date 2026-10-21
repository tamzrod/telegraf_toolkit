# P2L-006 — Pre-PR live-probe security hardening

**State:** HUMAN-AUTHORIZED ACTIVE WORK — **COMPLETE, CHECKPOINTED 2026-10-21** (archived to `archive/active-work/`). Both HIGH-priority findings fixed with regression coverage.
**Branch:** `phase-2/live-registry-implementation`
**Current task:** P2L-006; sole authorized task.

## Objective
Resolve two HIGH-priority independent pre-PR security findings before exposing the live Registry:
- **F-01 DNS rebinding / peer mismatch:** `backend/internal/liveprobe/probe.go` currently validates an initial DNS lookup but uses a standard HTTP client that can independently resolve DNS again at dial time. Ensure connection goes only to a validated destination IP, validate every connected peer, preserve correct TLS SNI/certificate hostname verification, prevent proxy-related bypass, deny redirects, and retain bounded deadlines, budgets and response sizes.
- **F-02 Credential forwarding:** `backend/internal/registryapi/registryapi.go` accepts caller-selected `endpoint` and `credential_ref_id` independently, permitting a principal holding `profile.probe` to direct a stored secret to an arbitrary external host. Resolve authorized connection profiles server-side, enforce exact allowed endpoint/scope/credential-ref association, and never forward a resolved secret to a caller-supplied unrelated origin. Permission to probe alone must not grant arbitrary secret use. Include principals with weaker roles in security tests.

## Acceptance and verification
1. Review both dataflow paths and their tests. Implement secure origin binding and DNS/peer enforcement in actual server code. Preserve the test-only explicit loopback policy exclusively for isolated `testing/influxdb/` and maintain production-default denial of private, metadata and loopback addresses.
2. Add adversarial automated tests: DNS changes between validation and dial, multi-answer DNS, proxy/redirect paths, forbidden actual dial peer, TLS hostname validity, untrusted origin with valid stored credential ref, cross-profile credential substitution, invalid/missing profile, role-based authorization (operator/viewer/admin), denied attempts without leaked tokens in response/logs.
3. Exercise legitimate authorized probes against isolated InfluxDB 2 OSS and 3 Core using TEST-only provisioned credentials. Do not weaken protections to make fixtures pass. Do not claim InfluxDB 1.x verified.
4. Run `make check`, `go test -race ./...`, applicable `testing/influxdb/` live-probe and Toolkit integration harness checks, `scripts/policy-check.sh`, and secret hygiene inspection. State actual evidence and limitations; if Docker unavailable, report blockage for required live tests.
5. Archive this task using `git mv` to `archive/active-work/`, update `handoff.md` and the Active Work README to empty queue, commit/push and confirm remote checkpoint. Report `WORK_PACKAGE_READY` only when both vulnerabilities have regression coverage and checks pass. Otherwise report `TASK_BLOCKED` with actionable evidence and stop.

## Restrictions
No production historians/credentials, no unsanctioned network probes, no acceptance of ADR-0004/0005, no unrelated feature expansion or weak fallback to mocks. Use only isolated test containers. Never log actual credential values. No agent PR create/review/merge. Human owns PR gates.

## Verification evidence (recorded 2026-10-21)

**State:** COMPLETE, CHECKPOINTED. Both HIGH-priority findings fixed with
regression coverage; all checks pass.

**Environment:** branch `phase-2/live-registry-implementation`; Go 1.27.3; Node.js
24.21.0 / npm 11.21.0; Docker Engine 29.9.1 / Compose v5.6.0 on the isolated test
host. Pinned images unchanged.

### F-01 — DNS rebinding / peer mismatch (fixed)

`backend/internal/liveprobe/probe.go` now resolves the endpoint **once**, pins the
validated addresses, and builds a **per-probe** `http.Client` that:
- dials only a pinned IP and never re-resolves the hostname;
- re-validates the concrete peer address in a `net.Dialer.Control` hook;
- pins TLS `ServerName` to the endpoint host and never sets `InsecureSkipVerify`;
- sets `Proxy: nil` so an environment proxy cannot bypass pinning;
- refuses redirects and keeps bounded deadline/budget/body.

New regression tests (`liveprobe/security_test.go`):
`TestDialIsPinnedToValidatedPeer` (resolver changes answer between validation and
dial), `TestDialRejectsUnpinnedPeer`, `TestProbeRefusesMultiAnswerWithDeniedPeer`,
`TestProbeRefusesUnspecifiedAndLinkLocal`, `TestProbeRefusesRedirectToDeniedHost`,
`TestPinnedClientUsesEndpointSNI`, `TestPinnedClientTLSHostnameVerification`,
`TestPinnedClientHasNoProxy`, `TestPinnedClientRejectsEmptyPeers`.

### F-02 — credential forwarding (fixed)

`backend/internal/registryapi/registryapi.go` now accepts **only a connection
profile ID** for a probe; the endpoint and credential reference are resolved
server-side from the stored profile. The profile carries its own `endpoint`
(new migration v4 + repository + payload + UI contract), and a caller-supplied
`endpoint` or `credential_ref_id` is rejected as an unknown field.

New regression tests (`registryapi/security_test.go`):
`TestProbeUsesProfileOwnEndpointAndCredential`,
`TestProbeCannotForwardCredentialToArbitraryHost`,
`TestProbeCannotSubstituteAnotherProfilesCredential`,
`TestProbeAuthorizationByRole` (anonymous/viewer/operator/administrator),
`TestDeniedProbeLeaksNoCredential`, `TestProbeResultCarriesNoCredential`;
plus `TestProbeRouteRejectsUnknownProfile`,
`TestProbeRouteIgnoresCallerSuppliedEndpoint`,
`TestProbeRouteRejectsPrivateTargetUnderDefaultPolicy`.

Production-default SSRF denial is unchanged; the loopback/insecure-TLS exception
remains opt-in (`-allow-test-probe-policy`) for the isolated harness only.

### Commands and actual outcomes
- `gofmt -l .` → clean; `go vet ./...` and `go vet -tags integration ./...` → pass;
  `go build ./...` → pass.
- `go test ./... -count=1` → all packages `ok` (170 tests).
- `go test -race ./... -count=1` → all packages `ok`.
- `make check` → exit 0 (backend gofmt/vet/build/test; frontend lint/build/`vitest
  run` 15 files / 152 tests; `policy-check.sh` → PASS).
- `testing/influxdb/scripts/live-probe.sh` (isolated stack) → exit 0: InfluxDB 2
  → `edition=influxdb-2.x-oss`; InfluxDB 3 → `edition=influxdb-3.x-core`;
  default-policy loopback rejection passes.
- `testing/influxdb/scripts/toolkit-live-registry.sh` (isolated stack) → exit 0
  (8 checks): anonymous 401; admin create 201; listing; viewer 403; live probe
  `edition=influxdb-2.x-oss` `mock=false`; caller-supplied endpoint rejected 400;
  profile survives backend restart.
- Secret hygiene: no `.key`/`.db`/token/secret/report/log artifact is tracked;
  credential values in tests are test-only literals; `git add -An` stages none.

### Limitations
- InfluxDB 1.x remains unverified (no isolated instance).
- Tokens remain static file-provisioned (no issuance/expiry/rotation, no OIDC).
- Browser credential-write flows remain gated; the harness seeds a TEST-only
  credential through a build-tagged helper.
- Only Linux amd64 verified. No production access, historian write or Docker
  privilege expansion occurred.
