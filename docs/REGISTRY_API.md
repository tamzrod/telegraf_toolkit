# Authorized Registry API

Status: **Implemented (server-authorized, metadata and references only).**
Tasks: P2L-001 (persistence), P2L-002 (secret store), P2L-003 (authorized API).

This document describes the authorized Registry HTTP surface. Authorization is
enforced **server-side**; the browser is never a security boundary. No route
stores, resolves, returns or logs a credential value.

## Authentication

Every Registry data route requires a bearer token:

```
Authorization: Bearer <token>
```

- A missing or malformed header is rejected with **401**.
- A token that does not resolve to a principal is rejected with **401**.
- Tokens are provisioned outside the application in a JSON file with
  **owner-only (0600)** permissions. The file is never committed; a group- or
  other-readable file is refused at startup.

Token file shape:

```json
{
  "tokens": [
    { "token": "<opaque token>", "actor": "alice", "roles": ["administrator"] },
    { "token": "<opaque token>", "actor": "bob", "roles": ["viewer"] }
  ]
}
```

`authz.LoadTokenFile` fails closed: a missing, too-open, malformed, empty or
role-invalid file yields no authenticator. Without an authenticator the backend
registers **no** Registry data routes at all, so there is no anonymously
reachable surface.

## Roles and permissions

Roles follow [`DATABASE_REGISTRY.md`](DATABASE_REGISTRY.md). Permissions are
additive and least-privilege: a principal holds only what its roles grant, and an
unknown role grants nothing.

| Role | profile.read | profile.write | credential.read | credential.write | audit.read | credential.resolve |
| --- | --- | --- | --- | --- | --- | --- |
| viewer | ✓ | | | | | |
| operator | ✓ | | ✓ | | | |
| configurator | ✓ | ✓ | ✓ | | | |
| administrator | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

Denial is the default. An anonymous principal is unauthenticated (401); a
principal lacking the required permission is forbidden (403).

## Routes

| Method | Path | Permission | Behaviour |
| --- | --- | --- | --- |
| GET | `/api/v1/registry/profiles` | profile.read | List profiles (references only) |
| POST | `/api/v1/registry/profiles` | profile.write | Create a profile; validated and persisted |
| GET | `/api/v1/registry/profiles/{id}` | profile.read | Read one profile |
| DELETE | `/api/v1/registry/profiles/{id}` | profile.write | Delete a profile |
| GET | `/api/v1/registry/credentials` | credential.read | List credential references (no values) |
| GET | `/api/v1/registry/audit` | audit.read | List sanitized audit events |
| GET | `/api/v1/registry/status` | none (status only) | Store reachability, schema version, secret-store flag |

Responses carry a `credential_ref_id`, never a value. The `credentials` route
returns alias, role, fingerprint and timestamps only.

## Input validation

- A create body is decoded strictly (unknown fields are rejected) and validated
  through the contract (`ValidateProfileInput`): a stable identity, a complete
  scope, a credential **reference** and no secret-bearing text or
  credential-bearing URL.
- A body carrying a secret-like key (`value`, `password`, `token`, `apikey`,
  `secret`, …) anywhere is refused before parsing, so a value is never handled.
- Validation failures return a sanitized error with a stable code and an HTTP
  status mapped from that code (400/404/409/422).

## Audit

Every mutation records an audit event attributed to the acting principal's
identity reference, with the target reference ID and an outcome. Audit events
never contain a credential value. A denial (401/403) is not recorded as a
successful mutation; it is refused before the service is called.

## Coverage and unsupported editions

- **Coverage:** authenticated, authorized profile CRUD and read-only credential
  reference / audit listing for the single-host deployment.
- **Unsupported:** token issuance/rotation, OIDC/OAuth, per-connection or
  per-site authorization refinement, and any credential-value route. These
  require separate authorization. There is no anonymous admin path and no
  browser-only access control.

## Verification evidence (recorded 2026-10-21)

**Environment:** branch `phase-2/live-registry-implementation`; Go 1.27.3. No new
dependency (standard library plus the existing pinned SQLite driver).

**Commands and actual outcomes:**
- `gofmt -l .` → clean; `go vet ./...` → pass; `go build ./...` → pass.
- `go test ./... -count=1` → all packages `ok`.
- `make check` → exit 0.
- Process-level: with a token file, anonymous `GET /profiles` → **401**; a viewer
  `POST /profiles` → **403**; an administrator `POST /profiles` → **201**; a
  viewer `GET /profiles` → **200**. With no token file, the data routes are not
  registered.

**Acceptance mapping (P2L-003):**
- Backend-enforced auth with explicit scopes → `authz` role matrix and
  `registryapi` 401/403 tests.
- Validated CRUD, actual persistence → API create/list/get/delete tests over a
  real SQLite store.
- Secret references only in responses → `TestCreateProfileResponseCarriesReferenceNotValue`,
  `TestCredentialsRouteNeverReturnsValue`.
- Audited mutations → `TestAdminCanReadAudit` (actor recorded).
- Denial tests → `TestAnonymousRequestsAreRejected`, `TestInvalidTokenIsRejected`,
  `TestViewerCanReadButNotWrite`, `TestViewerCannotReadCredentialsOrAudit`,
  `TestConfiguratorCanWriteProfilesButNotCredentials`.
- No anonymously writable endpoints → `TestRegistryDataRoutesRequireAuthentication`.

## Limitations

- Tokens are static and file-provisioned; there is no token issuance, expiry or
  rotation, and no OIDC/OAuth integration.
- Authorization is role-based only; per-connection/per-site refinement is not
  implemented.
- Transport TLS termination is out of scope here; the backend assumes a
  TLS-terminating front end in deployment (packaging is ADR-0005, Proposed).
- Only Linux amd64 is verified. No historian read/write, Docker control or live
  probe is performed.
