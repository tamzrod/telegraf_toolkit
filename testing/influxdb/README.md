# Isolated InfluxDB integration test environment

Reproducible, **non-production** Docker Compose environment for exercising the
Telegraf Toolkit against real **InfluxDB 2 OSS** and **InfluxDB 3 Core**
containers. It exists to produce honest integration evidence: real container
startup, real writes/queries, real authorization behavior and real restart
persistence — not mocks.

This directory contains only test infrastructure. It does not implement the
Toolkit Database Registry, does not accept any ADR, and must never be pointed at
a production historian or a remote host.

## Safety model

- **Test-host only.** Bind-mounted data directories under `data/` and generated
  credentials under `secrets/` are local and disposable.
- **Pinned images.** Official upstream `influxdb` images, pinned to an exact
  patch tag **and** an immutable multi-arch index digest. No floating tags.
- **Loopback only.** Host ports bind to `127.0.0.1`. The stack uses its own
  Compose project and network.
- **No privileged containers.** No host Docker socket, no added capabilities.
- **Secrets never printed or committed.** Credentials are generated into
  `secrets/` (Git-ignored, mode `0400`) and injected via Compose `secrets` as
  files, never as inline environment values. Curl reads tokens from files
  (`-H @file`) so they never appear in a process argument list. Logs and
  reports pass through a redactor.

## Pinned images

| Service | Image | Digest (multi-arch index) |
| --- | --- | --- |
| InfluxDB 2 OSS | `influxdb:2.9.1` | `sha256:cdd3dbdea45c2f43513abd06ee9644b6e05d2354a2b93ca829c9f07d716eaaac` |
| InfluxDB 3 Core | `influxdb:3.12.1-core` | `sha256:1c8200f75657c7586678b52570f8c78aedfa2cd42e59c238a733423aab6bd406` |

Both digests resolve to a linux/amd64 and linux/arm64 manifest. Re-verify a pin
before use:

```bash
docker buildx imagetools inspect influxdb:2.9.1@sha256:cdd3dbde...
docker buildx imagetools inspect influxdb:3.12.1-core@sha256:1c8200f7...
```

## Prerequisites

- Docker Engine with the Compose plugin, and permission to use it. If your user
  is not in the `docker` group, export `DOCKER_SUDO=1` (the scripts then prefix
  `sudo docker`).
- `openssl`, `curl` and `bash` on the host.
- Go 1.27.3 (per `docs/DEVELOPER_SETUP.md`) for the Toolkit smoke test only.
  Point `GO_BIN` at it if it is not on `PATH`.

## Usage

```bash
cd testing/influxdb

# 0. optional: copy the non-secret config template
cp .env.example .env

# 1. generate throwaway credentials (mode 0400, never printed)
./scripts/init-secrets.sh

# 2. start the stack and wait for readiness
./scripts/up.sh

# 3. run the InfluxDB integration checks (PASS/FAIL table)
./scripts/run-tests.sh

# 4. run the Toolkit scaffold smoke test
GO_BIN=/path/to/go ./scripts/toolkit-smoke.sh

# 5. run the read-only live capability probes (task P2L-004)
GO_BIN=/path/to/go ./scripts/live-probe.sh

# 6. run the end-to-end live Registry API check (task P2L-005)
GO_BIN=/path/to/go ./scripts/toolkit-live-registry.sh

# 7. run the live credential lifecycle check (task P2C-007)
GO_BIN=/path/to/go ./scripts/credential-lifecycle.sh

# 8. run the authorized GET-only schema-discovery check (task P3-002)
GO_BIN=/path/to/go ./scripts/schema-discovery.sh

# 9. run the authenticated real-backend schema client smoke (task P3-003)
GO_BIN=/path/to/go ./scripts/canonical-mapper-smoke.sh

# ...or do everything in one go and write sanitized evidence
GO_BIN=/path/to/go ./scripts/run-all.sh --teardown

# stop and remove containers + network (keeps data and secrets)
./scripts/teardown.sh

# stop, remove containers and delete test data
./scripts/teardown.sh --volumes

# stop, remove containers and delete test data and generated credentials
./scripts/teardown.sh --volumes --secrets
```

If Docker is unavailable or the host is not an authorized isolated test
environment, the scripts exit `2` with a `BLOCKED:` message. They never claim a
container is running when it is not.

## What the checks cover

`scripts/run-tests.sh` prints one line per check with `PASS`/`FAIL`:

InfluxDB 2 OSS

- unauthenticated `/health` reports `status=pass` and version `v2.9.1`
- authenticated `/api/v2/orgs` with the provisioned operator token
- an invalid token is rejected (`401`)
- line-protocol write (`204`)
- Flux query returns the expected total point count
- InfluxQL-compatible `/query` returns the seeded measurements
- data survives a container restart (persistent volume)

InfluxDB 3 Core

- `/health` requires auth (`401` without a token)
- authenticated `/health` returns `OK`
- an invalid token is rejected (`401`)
- database creation over the HTTP configuration API
- `write_lp` line-protocol write (`204`)
- SQL query returns the expected row count
- InfluxQL-compatible `query_influxql` returns the seeded measurement
- data survives a container restart
- integer field type is preserved

`scripts/toolkit-smoke.sh` reports the Toolkit scaffold separately:

- `GET /api/v1/health` returns the documented scaffold payload
  (`scaffold=true`, `connected=false`)
- `POST /api/v1/health` is rejected (`405`)
- `GET /api/v1/registry/profiles` is reported **NOT REGISTERED** — the live
  Registry exists in source since PR #6, but its data routes are only registered
  when an authentication token file is configured. This run has none, so the
  request is answered with `404`. This is a deliberate, honest result, not a
  failure; the authenticated Registry surface is covered by
  `scripts/toolkit-live-registry.sh`.

`scripts/live-probe.sh` runs the read-only live capability probes
(`backend/internal/liveprobe`, task P2L-004) against the running containers. It
performs GET requests only, never writes, and requires the isolated stack to be
up. See [`docs/LIVE_PROBING.md`](../../docs/LIVE_PROBING.md).

`scripts/credential-lifecycle.sh` (task P2C-007) runs the gated integration
tests that exercise the full credential replacement and revocation lifecycle
through the authorized Registry HTTP surface against both running containers.
A replacement candidate is only accepted after a protected, read-only listing
refuses an anonymous request and accepts the candidate; an invalid candidate is
refused with the working credential preserved. Every request it makes against a
deployment is a read-only GET; no historian is ever written.

`scripts/schema-discovery.sh` (task P3-002) runs the gated integration tests
that read measurement, field-key and tag-key metadata through the authorized
Registry HTTP surface against both running containers. Every deployment request
is a GET from the fixed `SHOW MEASUREMENTS` / `SHOW FIELD KEYS` /
`SHOW TAG KEYS` allowlist; the read path never writes. The check asserts the
reported edition and scope, an injection attempt and an absent measurement are
refused with 404, and a viewer is denied with 403. It requires the stack to be
up and `run-tests.sh` to have seeded the synthetic data.

`scripts/canonical-mapper-smoke.sh` (task P3-003) starts the Toolkit API with a
throwaway master key and admin token under `tmp/`, seeds a read credential
server-side, creates a profile bound to the isolated InfluxDB 2 OSS bucket, and
checks the fixed GET measurements/fields/tags routes. It then runs the typed
browser client (`ui/src/api/schema.live.integration.test.ts`) against the same
authenticated API: the test is skipped in normal `npm test` runs and enabled only
by the harness environment. The token is passed in the child environment, never
printed or placed in argv; all deployment requests are GETs and the script
leaves the containers' synthetic state unchanged.

`scripts/toolkit-live-registry.sh` (task P2L-005) starts the Toolkit backend
with a throwaway master key and authentication token file under `tmp/`, then
exercises the authorized Registry API end to end: anonymous denial (401), an
admin create (201), a listing, a viewer denial (403), a read-only capability
probe against the isolated InfluxDB 2 endpoint, and a profile-survives-restart
check. The test-only loopback probe policy is requested explicitly
(`-allow-test-probe-policy`); it is never a production default.

## Evidence and ignored paths

Tracked: `compose.yaml`, `.env.example`, `scripts/`, `seed/` and this README.

Git-ignored (see `.gitignore`): `secrets/`, `data/`, `volumes/`, `logs/`, `tmp/`,
`artifacts/`, `reports/`, `.env`, and `*.token`/`*.key`/`*.pem`. Never commit a
generated credential or database contents.

`run-all.sh` writes a sanitized report to `reports/` and the Toolkit API log to
`logs/`.

## Scope and limitations

- This is infrastructure and testing only. It does not implement the Toolkit
  Registry backend itself (that lives in `backend/`), it accepts no ADR
  (ADR-0002/0003 are **Accepted** and implemented; ADR-0004/0005 remain
  **Proposed**), and it does not touch any production system.
- Passwords/tokens here are synthetic throwaway values for disposable
  containers. Do not reuse them anywhere.
- The redactor is a targeted pattern match, not a guarantee; treat the
  `secrets/`, `data/`, `logs/` and `reports/` trees as sensitive anyway.
- Running the containers as the host UID (instead of the image default) is a
  deliberate choice so `0400` secret files stay readable without loosening
  permissions. InfluxDB 2 accepts this; InfluxDB 3 Core needs `HOME` writable,
  which the Compose file sets to `/tmp`.
