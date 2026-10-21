# P2L-004 — Real InfluxDB probes with network protections

**State:** HUMAN-AUTHORIZED ACTIVE WORK — **COMPLETE, CHECKPOINTED 2026-10-21** (archived to `archive/active-work/`). Read-only live capability probing implemented and verified against the isolated InfluxDB 2/3 harness.
**Branch:** `phase-2/live-registry-implementation`.
**Architecture gate:** ADR-0002 (single-host SQLite) and ADR-0003 (backend-only envelope encryption, separate key custody and rotation) were accepted by the human on 2026-10-21. Respect their exact scopes; ADR-0004/0005 remain Proposed.

## Objective
Implement consented read-only capability probes for tested InfluxDB 1.x/2.x/3.x deployments, using scoped credentials, endpoint allowlisting, DNS resolution and network-peer SSRF restrictions, TLS defaults, bounded timeouts and sanitized responses. Integration tests against explicitly isolated test instances; never infer support by version alone.

## Acceptance and safety
- Implement actual working behavior, not in-memory or mock substitutes; clearly document coverage and unsupported editions.
- Never connect to production historians or manipulate real credentials without a separately authorized safe test environment and explicit permission. No historian writes, runtime deployments or Docker privilege expansion.
- Require server-side authorization, TLS and DNS/network SSRF protections before live probing. Verify via automated negative and isolated integration tests.
- Preserve exact dependency pins, `make check`, documented evidence and CWAL checkpoints. Human controls PR initiation, review and merge.

## CWAL execution instructions

- Execute strictly in queue order P2L-001 → P2L-005; complete and verify the current task, `git mv` it to `archive/active-work/`, update `handoff.md` and `workflow/active_work/README.md`, commit, push and confirm remote checkpoint before advancing without intermediate human approval.
- Do not weaken backend auth or SSRF protections to allow tests. **Only** isolated `testing/influxdb/` instances are authorized; local loopback/private-address exceptions must be confined to explicit test configuration, never enabled by default on production Registry endpoints.
- Preserve existing typed contracts and mock labelling until functionality is truly connected; no mock-only substitution for an acceptance claim.
- Run `make check`, relevant integration checks and policy checks for each task; capture tested features, blocked states, and limitations without committing tokens/database volumes/logs. Stop as `TASK_BLOCKED` for unavailable safety-critical dependencies, not counterfeit success.
- No OpenHands PR create/review/merge actions; final package `WORK_PACKAGE_READY` only when all five tasks have been archived with confirmed checkpoints.

**Specific acceptance:** exercise real read-only authenticated probes against the PR #5 pinned InfluxDB 2 OSS and 3 Core test containers. Explicitly distinguish unsupported/unverified InfluxDB 1.x; no claim of coverage without a real 1.x instance. Enforce destination DNS/IP peer checks, TLS defaults and tight timeout/budget. Do not silently allow remote/private hosts.

## Verification evidence (recorded 2026-10-21)

**Environment:** branch `phase-2/live-registry-implementation`; Go 1.27.3; Docker
Engine 29.9.1 / Compose v5.6.0 on the isolated test host. No new Go dependency.
Pinned images unchanged: `influxdb:2.9.1@sha256:cdd3dbde…716eaaac` and
`influxdb:3.12.1-core@sha256:1c8200f7…ab6bd406`.

**Deliverables:**
- `backend/internal/liveprobe/probe.go` — read-only prober with deny-by-default
  endpoint policy, DNS resolution of every peer address, redirect refusal,
  bounded deadline/budget/body, edition identification from real responses and
  capability projection.
- `backend/internal/liveprobe/probe_test.go` — 13 unit tests (policy, SSRF,
  mixed-DNS refusal, credential-URL rejection, budget, deadline, redirects,
  2.x/3.x fixtures, no-secret result).
- `backend/internal/liveprobe/integration_test.go` — build-tagged live tests
  against the isolated containers, including a default-policy rejection test.
- `backend/internal/registryapi` — `POST /api/v1/registry/probe` requiring
  `profile.probe`, server-side credential resolution, audited probe.
- `backend/internal/authz` — `profile.probe` permission added to operator,
  configurator and administrator.
- `backend/cmd/api/main.go` — `-allow-test-probe-policy` opt-in for the test
  loopback/insecure-TLS policy (never a production default).
- `testing/influxdb/scripts/live-probe.sh` — harness runner for the live probes.
- `docs/LIVE_PROBING.md` — policy, DNS/TLS, coverage and limitations.

**Commands and actual outcomes:**
- `gofmt -l .` → clean; `go vet ./...` and `go vet -tags integration ./...` →
  pass; `go build ./...` → pass.
- `go test ./... -count=1` → all packages `ok`.
- `make check` → exit 0 (backend + frontend + policy).
- `./scripts/run-all.sh --teardown` → exit 0; InfluxDB **16 passed / 0 failed**;
  Toolkit smoke 2 passed / 1 not implemented.
- `./scripts/live-probe.sh` → exit 0:
  - InfluxDB 2 → `edition=influxdb-2.x-oss`, dialects `[flux influxql sql]`,
    write `[line-protocol:supported]`, schema discovery `supported`, requests=1.
  - InfluxDB 3 → `edition=influxdb-3.x-core`, dialects `[flux influxql sql]`,
    write `[line-protocol:supported]`, schema discovery `unsupported`, requests=1.
  - Production default policy on the loopback endpoint → refused
    (`tls-validation-failed`).

**Acceptance mapping:**
- Real read-only authenticated probes against pinned 2 OSS / 3 Core containers →
  the live integration tests.
- InfluxDB 1.x explicitly unverified → no 1.x instance exists; coverage is not
  claimed (documented in `docs/LIVE_PROBING.md`).
- DNS/IP peer checks, TLS defaults, tight budget → unit tests plus the live
  default-policy rejection.
- No silent remote/private allowance → default policy refuses loopback/private
  and requires a documented override; the test policy is opt-in only.

**Limitations:** InfluxDB 1.x unverified; edition identification depends on the
`/health` shape; capability mapping is a small explicit table (ADR-E not yet
proposed); only Linux amd64 verified. Probes are read-only and no historian
write, Docker privilege expansion or production access occurred.
