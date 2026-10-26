# Telegraf Toolkit — Phase 0 scaffold

Status: **scaffold, not a product.** This is the Phase 0 application/tooling
skeleton. The scaffold health surface still reports `scaffold: true` and
`connected: false`, and it performs no historian reads or writes. The Phase 2
Database Registry backend now exists alongside it (persistent SQLite metadata
store, envelope-encrypted secret store, authorized Registry API, read-only live
probing) — see [`docs/ROADMAP.md`](ROADMAP.md) and [`docs/REGISTRY_API.md`](REGISTRY_API.md).
The live Registry performs **read-only** probes only; it never writes to a
historian, never manages a Telegraf runtime and never touches the Docker
daemon.

## Layout

```text
backend/                     Go API (module github.com/tamzrod/telegraf_toolkit/backend)
  cmd/api/                   API entrypoint (health + optional Registry routes)
  internal/httpapi/          HTTP surface + tests
  internal/version/          build version constant
  internal/authz/            role/permission model + token-file authenticator
  internal/metadatastore/    SQLite metadata store + versioned migrations (ADR-0002)
  internal/secretstore/      envelope-encrypted secret store (ADR-0003)
  internal/registrycontract/ profile/credential/capability contracts + validation
  internal/registryservice/  Registry service (profiles, references, audit)
  internal/registryapi/      authorized Registry HTTP API + binding endpoints
  internal/liveprobe/        read-only capability prober (SSRF/DNS-pinned)
  internal/fixtures/         test-only fixtures
ui/                          React + TypeScript SPA (Vite)
  src/app/                   routes/shell
  src/api/                   typed API client boundary
  src/features/overview/     first feature module + tests
  src/features/databases/    Registry module (mock + live listing/probing)
  src/types/                 shared types
.github/workflows/ci.yml     format/lint/build/test + policy checks
Makefile                     local commands mirroring CI
```

## Pinned versions

Exact versions are pinned in `ui/package.json` (with `ui/package-lock.json`)
and `backend/go.mod`. No floating tags, ranges or `latest` are permitted.

| Component | Pinned version |
| --- | --- |
| Go toolchain | 1.27.3 |
| Node.js (CI) | 24.21.0 |
| React / React DOM | 19.3.1 |
| Vite | 8.3.4 |
| TypeScript | 6.0.3 |
| Vitest | 5.0.5 |
| Tailwind CSS | 4.3.3 |
| TanStack Query | 5.104.5 |
| React Router | 7.18.4 |
| ESLint | 10.13.0 |

TypeScript is pinned to 6.0.3 rather than the newest major because
`typescript-eslint@8.72.0` requires `typescript >=4.8.4 <6.1.0`. Installing a
newer TypeScript fails peer resolution; this is recorded as a real constraint,
not hidden with `--force`.

## Local verification

See [`DEVELOPER_SETUP.md`](DEVELOPER_SETUP.md) for clean-environment creation,
isolated mock fixtures and portability limits.

Requires Go 1.27.3 and Node.js 24.21.0 on `PATH`.

```bash
make check          # backend + frontend + policy checks (mirrors CI)
make backend-check  # non-mutating gofmt check, vet, build, test
make frontend-check # npm ci, lint, build, test
make policy-check   # shared regex policy checks (see DEVELOPER_SETUP.md)
make run-api        # run the scaffold API on :8080
```

Run the SPA in development:

```bash
cd ui && npm ci && npm run dev
```

## Scaffold health endpoint

```bash
curl -s http://127.0.0.1:8080/api/v1/health
```

```json
{
  "status": "ok",
  "service": "telegraf-toolkit-api",
  "version": "0.0.0-scaffold",
  "scaffold": true,
  "connected": false,
  "description": "Phase 0 scaffold. Not connected to databases, Telegraf runtimes or Docker."
}
```

The `scaffold` and `connected` fields exist so no consumer can mistake this for
a live connection.

## Frontend integration conventions (Phase 1)

The SPA has exactly one integration seam: `ui/src/api/client.ts`. Phase 1
implements only `/api/v1/health`; no endpoint may claim database, runtime or
historian capability. Conventions:

- **Base path.** All paths are built from `API_BASE` (`/api/v1`) inside
  `requestJson`, so callers pass only a relative path (`/health`).
- **Typed errors.** Transport failures throw `ApiError`, which carries a status
  code and URL but never a raw response body. UI code must not render unsanitized
  backend text.
- **Payload validation.** Each response is checked with a runtime type guard
  (`isHealth` in `ui/src/types/health.ts`). A guard failure raises `ApiError`
  instead of coercing an unexpected shape into feature state.
- **Cancellation.** Query functions forward the `AbortSignal` provided by
  TanStack Query, so unmounting a consumer aborts the in-flight request.
- **Query keys.** Keys come from `ui/src/api/queryKeys.ts`. The health query
  lives in `ui/src/hooks/useHealth.ts` with `retry: false` so the scaffold error
  state is deterministic.
- **Mock adapters.** Test-only mock responses live under `ui/src/api/mock/` and
  `ui/src/fixtures/`. Application code must never import them; tests stub
  `fetch` instead. Fixtures are deterministic and synthetic.
- **Feature folder convention.** Each module owns `ui/src/features/<name>/` with
  its page and tests; shared presentation lives in `ui/src/components/common/`.
- **No credentials.** The browser never sends or stores credentials, tokens or
  credential-bearing URLs.

## Deliberately not decided here

The metadata store (ADR-0002) and secret-store integration (ADR-0003) are
**Accepted** and implemented — see [`docs/METADATA_STORE.md`](METADATA_STORE.md)
and [`docs/SECRET_STORE.md`](SECRET_STORE.md). The host-agent interface
(ADR-0004) and production packaging (ADR-0005) remain **Proposed** and are not
implemented; feature-boundary stubs are used instead. See
[`docs/contracts/README.md`](../docs/contracts/README.md).
