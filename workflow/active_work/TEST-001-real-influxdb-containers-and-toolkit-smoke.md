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

## Acceptance evidence
- Exact image pins/digests, secure isolated Compose configuration, reproducible setup and cleanup, and synthetic data tests.
- Actual container startup/health/query/auth/restart results **or** explicit blocked state if Docker not available.
- Toolkit `make check` result, honest split between tested program functions and yet-unimplemented live Registry connectivity, plus no tracked secrets.
- CWAL checkpoint confirmed only after successful verification; never claim `WORK_PACKAGE_READY` for work not run.
