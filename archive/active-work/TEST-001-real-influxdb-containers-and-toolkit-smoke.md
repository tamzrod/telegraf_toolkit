# TEST-001 — Real InfluxDB 2/3 containers and Toolkit integration smoke

**State:** HUMAN-AUTHORIZED ACTIVE WORK. The human requested a bounded CWAL microtask to run real test databases and exercise the Toolkit program.
**Designated branch:** `test/real-influxdb-integration`.
**Current task:** TEST-001 (sole authorized task).

## Objective
Create a reproducible, **non-production**, isolated Docker Compose test environment with real **InfluxDB 2 OSS** and **InfluxDB 3 Core** containers, then run realistic synthetic-data and Toolkit smoke/integration checks. This is infrastructure/testing work, NOT authorization to implement the operational Registry, accept ADR-0002/0003, or probe any production service.

## Implementation steps
1. Preflight Docker engine/Compose permissions and verify the execution host is an authorized isolated developer/test environment. If Docker or isolation is unavailable, deliver committed reproducible configuration and report the deployment as blocked; do not claim containers are running.
2. Under `testing/influxdb/`, add documented Compose configuration, `.env.example`, synthetic dataset/seed and probe scripts, startup/teardown commands, test runner, and a README. Track these source files in Git; keep secrets and runtime volumes under paths already ignored in `.gitignore`.
3. Use official InfluxDB 2 OSS and InfluxDB 3 Core images with **verified exact patch versions and immutable digests**, respecting pinning policy (never floating `latest`, `2` or `3-core`). Validate image architecture availability and entrypoint/CLI syntax against official docs before committing.
4. Bind host-published endpoints to `127.0.0.1` only (or avoid publishing ports), use a separate named Compose project/network and dedicated test volumes. No external production endpoints; no host Docker socket inside containers; do not run privileged containers.
5. Bootstrap InfluxDB 2 via file-backed initialization secrets for admin username, password and token, with synthetic org/bucket. For InfluxDB 3 Core, securely provision its initial administrator token **before exposing the API**, preferably via its supported preconfigured-admin-token file flow. Generate credentials locally with restrictive permissions, never echo them, never commit them, never put them in command arguments, Compose interpolation output, logs or report artifacts.
6. Wait on readiness with bounded retries, write known synthetic measurements/tags/fields/timestamps to disposable scopes and read/query them using each version's documented supported APIs. Verify authorization negatives (invalid token), query dialect support only where the chosen edition actually supports it, basic reconnect and persistent-volume restart. Avoid destructive operations outside this Compose project.
7. Run Toolkit's existing Go/frontend test suite and test the application's existing scaffold `/api/v1/health` and Registry UI accurately. Inspect whether the current Toolkit backend truly implements an authenticated live Registry endpoint. **Do not invent one or mistake UI mocks / `registrycontract` in-memory fixtures for live integration**. Report explicit `PASS`, `FAIL` or `NOT IMPLEMENTED` separately for direct DB tests, Toolkit-to-DB authenticated connections, real capability probes, and UI flows.
8. Run `make check` (or equivalent exact steps) and any container integration tests that can run, save sanitized logs/results under ignored directories, and document exact tested image tags/digests, platform, endpoints (non-secret), outcomes and limitations. Provide repeatable cleanup; do not remove unrelated volumes or containers.
9. Verify no secrets or database contents tracked by Git (review `git status`, `git diff --cached`, ignored paths). On success archive this task by `git mv` to `archive/active-work/`, update handoff and Active Work README, commit/push a confirmed branch checkpoint, and report `WORK_PACKAGE_READY`. If blocked, document actual evidence and stop; never self-promote another task.

## Scope/architecture stop conditions
- ADR-0002, ADR-0003 remain Proposed: do not implement persistence or an operational secret store in this task.
- No production historian access, destructive data writes outside test containers, secrets publication or unaudited privileged operations.
- User's request authorizes *isolated test-host* Docker containers only, not remote/prod deployment. Stop if no demonstrably safe Docker host is available.
- PR initiation, review, approval and merge remain human-only; do not create/change PR state.

## Verification evidence (recorded 2026-10-20)

**Environment / isolation:** execution host is a disposable containerized
developer runtime (`Linux 6.8.0-1055-gke x86_64`, Docker Engine 29.8.1,
Compose v5.5.1). The Docker daemon was not running at session start; it was
started as an isolated local daemon (`sudo dockerd`). The invoking user is in
the `sudo` group but not the `docker` group, so the test scripts were run with
`DOCKER_SUDO=1`. No production historian, remote host, Docker socket mount,
privileged container or operational Registry change was involved.

**Pinned images (verified against Docker Hub, exact patch tag + index digest):**

- `influxdb:2.9.1@sha256:cdd3dbdea45c2f43513abd06ee9644b6e05d2354a2b93ca829c9f07d716eaaac`
  (linux/amd64 + linux/arm64; in-image `influxd version` → `InfluxDB v2.9.1`).
- `influxdb:3.12.1-core@sha256:1c8200f75657c7586678b52570f8c78aedfa2cd42e59c238a733423aab6bd406`
  (linux/amd64 + linux/arm64; `influxdb3 --version` → `InfluxDB 3 Core, 3.12.1`).
- CLI/flag and endpoint syntax were validated against the images and official
  docs before use (`--admin-token-file`, `--object-store=file`,
  `DOCKER_INFLUXDB_INIT_*_FILE`, `/api/v3/query_sql`, `/api/v3/write_lp`,
  `/api/v3/query_influxql`, `/api/v3/configure/database`, `/api/v2/query`).

**Deliverables (tracked under `testing/influxdb/`):** `compose.yaml`,
`.env.example`, `README.md`, `seed/dataset.lp` and `scripts/`
(`lib.sh`, `init-secrets.sh`, `up.sh`, `run-tests.sh`, `toolkit-smoke.sh`,
`run-all.sh`, `teardown.sh`). Secrets, data, logs, tmp, artifacts and reports
are Git-ignored.

**Commands and actual outcomes:**

- `./scripts/run-all.sh --teardown` (fresh purge, then full run) → InfluxDB
  checks **16 passed, 0 failed**; Toolkit smoke **2 passed, 0 failed, 1 not
  implemented**; `influxdb checks exit: 0`, `toolkit smoke exit: 0`; teardown
  removed only this project's containers and network.
- InfluxDB 2 OSS PASS: unauthenticated `/health` (`status=pass`, `v2.9.1`);
  authenticated `/api/v2/orgs` (200); invalid token rejected (401); line-protocol
  write (204); Flux total count `environment.used_percent == 5`; InfluxQL
  `/query SHOW MEASUREMENTS` → `environment`+`sensor`; restart persistence
  (5 points after restart).
- InfluxDB 3 Core PASS: `/health` requires auth (401 without token);
  authenticated `/health` → `OK`; invalid token rejected (401); database create
  over `/api/v3/configure/database` (200); `write_lp` (204); SQL
  `count(environment)==5`; `query_influxql SHOW MEASUREMENTS` → `environment`;
  restart persistence (5 rows after restart); integer field type preserved
  (`up=1`).
- Toolkit smoke: `GET /api/v1/health` → 200 with `scaffold=true`,
  `connected=false`; `POST /api/v1/health` → 405; `GET /api/v1/registry/profiles`
  → **NOT IMPLEMENTED** (404). The browser client calls that path but the
  backend implements only the health endpoint in `backend/internal/httpapi`.
- `make check` → exit 0: backend `gofmt -l` clean, `go vet`, `go build`,
  `go test` (all packages `ok`); frontend `npm ci`, `eslint`, `tsc -b && vite
  build`, `vitest run` → `Test Files 13 passed (13)`, `Tests 142 passed (142)`;
  `policy-check.sh` → PASS. Go 1.27.3 was installed from the official tarball
  with its published SHA256 verified; no repository dependency versions changed.
- Secret hygiene: `grep` over `reports/`, `logs/`, `tmp/` for token/password
  patterns → none found; `git add -An testing/` stages only the 10 intended
  source files (no secret, data, log or report path).

**Honest split of results:**

- Direct DB tests (real containers, real writes/queries/auth/restart): **PASS**.
- Toolkit-to-DB authenticated connection: **NOT IMPLEMENTED** — the Toolkit has
  no live database client; nothing in the backend contacts InfluxDB.
- Real capability probes: **NOT IMPLEMENTED** — `registrycontract` probing is
  fixture/in-memory only; the edition matrix is a test expectation.
- UI flows against a live backend: **NOT IMPLEMENTED** — the Databases page uses
  a labeled mock adapter; `/api/v1/registry/profiles` does not exist.

**Limitations:** the redactor is a targeted pattern match, not a guarantee;
credentials are synthetic throwaway values for disposable containers. This task
delivers test infrastructure and honest evidence only; it does not implement
live Registry connectivity and does not accept ADR-0002 or ADR-0003 (both remain
Proposed).
