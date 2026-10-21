# Authorized Registry API

Status: **Implemented (server-authorized, metadata and references only; multi-row writes are transactional).**
Tasks: P2L-001 (persistence), P2L-002 (secret store), P2L-003 (authorized API), P2L-006 (probe hardening), P2L-007 (credential binding), P2C-002 (transactional enrollment).
Merged into `main` via PR #6 on 2026-10-21 (merge commit `0ba4827`); the transactional guarantees above are added by P2C-002.

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

| Role | profile.read | profile.write | profile.probe | credential.read | credential.write | credential.attach | audit.read | credential.resolve |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| viewer | ✓ | | | | | | | |
| operator | ✓ | | ✓ | ✓ | | | | |
| configurator | ✓ | ✓ | ✓ | ✓ | | | | |
| administrator | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

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
| POST | `/api/v1/registry/probe` | profile.probe | Read-only capability probe; body is a connection profile ID only (endpoint and credential resolved server-side) |

Responses carry a `credential_ref_id`, never a value. The `credentials` route
returns alias, role, fingerprint and timestamps only.

## Credential-to-profile binding (P2L-007)

Attaching a credential reference to a profile is a **privileged** operation,
enforced server-side, not by the request shape or the UI:

- `credential.attach` is an administrator-only permission. A `configurator` may
  create a profile, but a profile that carries a credential reference is refused
  with 403 unless the principal holds `credential.attach`.
- An administrator either creates the profile with its credential through the
  privileged path, which records an approved **binding**, or approves a binding
  separately via `POST /api/v1/registry/credential-bindings`. The binding records
  the credential reference, connection ID, endpoint and canonical scope.
- A profile may exist **unbound** (no credential reference). An unbound profile
  is valid but resolves no credential.
- The service enforces the binding at **profile creation** (`CreateProfileAs`
  refuses a credential reference without a matching binding) and re-verifies it
  at **probe time** (`VerifyProfileCredentialBinding`), so a credential is never
  resolved for a profile whose identity is not approved.
- Changing a profile's endpoint or scope invalidates any prior binding: the
  binding matches on all of reference, connection ID, endpoint and canonical
  scope, so re-creating a profile with a different endpoint or scope cannot reuse
  an existing approval.
- The binding stores no secret value; the binding listing carries only the
  reference and profile identity.

This closes the escalation path where a principal with `profile.write` could
enumerate reference IDs, attach another principal's stored secret to an
endpoint it controls, and probe it to cause credential forwarding.

| Method | Path | Permission | Behaviour |
| --- | --- | --- | --- |
| GET | `/api/v1/registry/credential-bindings` | credential.attach | List approved bindings (no values) |
| POST | `/api/v1/registry/credential-bindings` | credential.attach | Approve a binding for a profile identity |

## Atomic enrollment and deletion (P2C-002)

A registry write that touches more than one row is applied as **one unit of
work**: the profile, its approved binding and the audit events are committed
together or rolled back together.

- `POST /api/v1/registry/profiles` (with or without a credential reference)
  writes the profile and its audit event together. A failure at either write
  leaves nothing behind, so a `4xx`/`5xx` response never corresponds to a
  half-created profile.
- `POST /api/v1/registry/profiles` with a credential reference (administrator
  only) writes the profile, the binding and both audit events together. A
  failure at any write leaves **no profile, no binding and no audit event** —
  a profile never exists without the approval that authorises it.
- `POST /api/v1/registry/credential-bindings` writes the binding and its audit
  event together; a failure keeps the previous approval in place.
- `DELETE /api/v1/registry/profiles/{id}` removes the profile's bindings, the
  profile and the audit event together, so a deleted profile cannot leave an
  orphaned binding behind. The store enforces the same rule with a foreign key
  (`ON DELETE CASCADE`).
- A **refused** request (401/403) is rejected before the service is called, so
  a denied attachment leaves no profile, no binding and no audit event at all.

The authorization rules are unchanged: `credential.attach` is still
administrator-only, and a binding still has to match the reference, connection
ID, endpoint and canonical scope exactly. See
[`docs/METADATA_STORE.md`](METADATA_STORE.md) for the store-level guarantees,
including the deletion policy, the idempotency and uniqueness rules, and the
SQLite busy/constraint treatment.

## Input validation

- A create body is decoded strictly (unknown fields are rejected) and validated
  through the contract (`ValidateProfileInput`): a stable identity, a complete
  scope, a valid endpoint and no secret-bearing text or credential-bearing URL. A
  credential reference is optional (a profile may be unbound) but must never be a
  value.
- A body carrying a secret-like key (`value`, `password`, `token`, `apikey`,
  `secret`, …) anywhere is refused before parsing, so a value is never handled.
- Validation failures return a sanitized error with a stable code and an HTTP
  status mapped from that code (400/403/404/409/422). A credential reference that
  is not approved for the profile's endpoint/scope returns `credential-not-bound`
  (403).

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
- The token file and the master key file must be readable only by their owner
  (mode 0600). On a host whose filesystem reports no POSIX permission bits
  (Windows), both files are refused and the Registry data routes stay
  unregistered; run the Registry locally on Linux.
