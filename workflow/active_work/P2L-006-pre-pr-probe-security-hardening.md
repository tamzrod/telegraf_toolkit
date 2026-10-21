# P2L-006 — Pre-PR live-probe security hardening

**State:** HUMAN-AUTHORIZED ACTIVE WORK (standing instruction: significant audit fixes through Operation CWAL)
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
