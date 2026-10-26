# Authorized Registry API

Status: **Implemented (server-authorized, metadata and references only; multi-row writes are transactional).**
Tasks: P2L-001 (persistence), P2L-002 (secret store), P2L-003 (authorized API), P2L-006 (probe hardening), P2L-007 (credential binding), P2C-002 (transactional enrollment), P3-002 (authorized schema discovery).
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

| Role | profile.read | profile.write | profile.probe | schema.read | credential.read | credential.write | credential.attach | audit.read | credential.resolve |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| viewer | ✓ | | | | | | | | |
| operator | ✓ | | ✓ | ✓ | ✓ | | | | |
| configurator | ✓ | ✓ | ✓ | ✓ | ✓ | | | | |
| administrator | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

`schema.read` is deliberately distinct from `profile.probe`: the read is a
different operation with a different data exposure, so it can be granted or
revoked independently even though the operator, configurator and administrator
roles hold both today. The viewer role holds neither.

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
| POST | `/api/v1/registry/credentials` | credential.write **and** credential.attach | Enroll a credential for a profile: the value is write-only, the response carries the reference |
| POST | `/api/v1/registry/credentials/replace` | credential.write **and** credential.attach | Replace the credential a profile is bound to (observe → stage → validate a protected authorization → CAS swap → revoke the staged material); a racing replacement reports `credential-replace-conflict` (409) |
| DELETE | `/api/v1/registry/credentials/{reference}` | credential.write | Revoke a credential; refused with 409 while an approval or a profile still references it, never detaching the dependent |
| GET | `/api/v1/registry/audit` | audit.read | List sanitized audit events |
| GET | `/api/v1/registry/status` | none (status only) | Store reachability, schema version, secret-store flag |
| POST | `/api/v1/registry/probe` | profile.probe | Read-only capability probe; body is a connection profile ID only (endpoint and credential resolved server-side). The response carries the three-valued outcomes **and** the `observations` that back them (check, method, path, status, outcome, reason) |
| GET | `/api/v1/registry/schema/measurements` | schema.read | Bounded, GET-only metadata read of the profile's observed measurement names |
| GET | `/api/v1/registry/schema/fields` | schema.read | Bounded field-key read for one observed measurement (`measurement` query parameter) |
| GET | `/api/v1/registry/schema/tags` | schema.read | Bounded tag-key read for one observed measurement (`measurement` query parameter) |
| GET | `/api/v1/registry/credential-bindings` | credential.attach | List approved bindings (no values) |
| POST | `/api/v1/registry/credential-bindings` | credential.attach | Approve a binding for a profile identity |
| DELETE | `/api/v1/registry/credential-bindings/{reference}/{connection}` | credential.attach | Revoke an approval |

Responses carry a `credential_ref_id`, never a value. The `credentials` route
returns alias, role, fingerprint and timestamps only.

## Authorized schema discovery (P3-002)

The Registry exposes a bounded, read-only metadata read for the selected
profile's **actual** schema. It is a separate surface from the capability probe
and does not change the probe's conclusions.

- **Profile-only inputs.** The route fixes the kind. The caller supplies
  `connection_id`, plus `measurement` for field/tag reads. The endpoint, logical
  scope and credential come from the stored profile; a caller-supplied URL,
  credential reference, query, deadline or limit is refused with 400. An
  unknown or repeated query parameter is refused rather than ignored.
- **Fixed statement allowlist.** The server executes only
  `SHOW MEASUREMENTS`, `SHOW FIELD KEYS FROM <measurement>` and
  `SHOW TAG KEYS FROM <measurement>` over GET. There is no caller-supplied
  statement, no POST query route, no `SELECT … INTO`, no DDL, no deletion and
  no write. The allowlist, not the HTTP method, is the write barrier.
- **Membership before interpolation.** A field/tag read first observes the
  measurement list and requires an exact membership match; the identifier is
  also escaped. A non-member returns 404 `measurement-not-found`; a control
  character or over-long name returns 400. A malformed attempt never reaches a
  statement.
- **Per-edition adapter.** The edition is identified from an observed `/health`
  response, never from the profile label. InfluxDB 2 resolves the bucket and its
  DBRP database with read-only `GET /api/v2/buckets` and `GET /api/v2/dbrps`
  (it never creates a mapping) and queries `GET /query`; InfluxDB 3 queries
  `GET /api/v3/query_influxql`. A scope that does not match the observed
  edition is refused as unknown without issuing a statement.
- **Bounds.** 15 s total deadline, 5 s per request, 8-request budget, 1 000-row
  cap, 1 MiB response-byte cap and context cancellation. The response carries an
  explicit `truncated` flag; a short list is never silently implied.
- **Honest outcomes.** `supported` only for a 200 with the expected shape.
  401/403, 404, 405, timeout, server error, transport failure or an unexpected
  body stays `unknown`; a 404 on a query route is not `unsupported`. A
  database authentication refusal is not an empty schema.
- **Credential behaviour.** A configured credential is resolved server-side only
  after its approved binding is re-verified for the exact profile, endpoint and
  scope. A missing, invalid or revoked configured credential fails closed with
  no anonymous fallback. A profile with no configured credential is read
  anonymously only where the deployment permits it.
- **Identity for invalidation.** Every response repeats `connection_id`, the
  canonical `scope` and the observed `edition`, so a client can reject a stale
  answer when the selection or the profile's scope changes.
- **Redacted audit.** Each read records one event: actor, action
  `schema.read.<kind>`, the profile target identity and the outcome. No rows,
  metadata values or credentials are recorded.
- **InfluxDB 1.x.** A database/retention-policy scope returns `unknown` with an
  explicit `schema-1x-unverified` observation and makes no request. InfluxDB 1.x
  remains unverified.

Response shape:

```json
{
  "connection_id": "plant-a",
  "scope": "acme/historian",
  "edition": "influxdb-2.x-oss",
  "kind": "fields",
  "measurement": "sensor",
  "outcome": "supported",
  "measurements": [],
  "fields": [{"name": "value", "type": "float"}],
  "tags": [],
  "observations": [
    {
      "check": "schema-v2-measurements",
      "method": "GET",
      "path": "/query?db=…&q=SHOW+MEASUREMENTS",
      "status": 200,
      "outcome": "supported",
      "edition": "influxdb-2.x-oss",
      "detail": "the endpoint returned the expected metadata"
    }
  ],
  "truncated": false,
  "requests": 5,
  "read_at": "2026-10-26T12:00:00Z",
  "mock": false
}
```

`measurements`, `fields` and `tags` are always present (empty arrays when the
observed list is empty) so an observed empty schema is distinguishable from a
missing field. An observation for a step that made no request (for example the
InfluxDB 1.x unverified state) carries `status: 0` and `method: "none"`, so it
cannot be read as an attempted GET. No response carries a credential, a
credential-bearing URL or sample data.

## Credential lifecycle (P2C-005)

Enrolling, replacing and revoking a credential are **administrator-only**:
`credential.write` and `credential.attach` are granted to the administrator
role alone, so a configurator, an operator and a viewer are refused with 403 and
nothing is written.

The value is **write-only**. It is accepted under the exact key `value` and
nowhere else: a body that smuggles a value under `password`, `token`, `apikey`
or `secret` is refused before it is parsed. The value is encrypted into an
envelope (ADR-0003), and the response carries the **reference** only - the
alias, the role, the fingerprint and the timestamps. No route returns a
credential value, and the browser never holds one after the request.

> **Transport requirement.** A credential value must only be submitted over a
> TLS-terminated transport. The backend does not terminate TLS itself
> (packaging is ADR-0005, **Proposed**), so the deployment must place a
> TLS-terminating front end in front of it. Over plain http the value would
> travel in clear.

**Enrolment** binds the credential to a profile. The body names the
`connection_id`; the endpoint and the canonical scope of the approval come from
the **stored profile**, never from the request, so an approval can only ever be
created for the profile's own identity (P2L-007). The reference, the approval
and the profile's pointer to the reference are applied as one unit of work
(P2C-002); if that unit of work fails, the envelope is removed again and the
rollback is audited.

**Replacement** follows a reviewed lifecycle (hardened by P2C-007):

1. **Observe.** The ciphertext currently stored for the credential is read as
   a compare-and-swap precondition. The swap below applies only while that
   exact ciphertext is still the one in place.
2. **Stage.** The new value is stored under a collision-resistant,
   per-attempt staged reference. No attempt ever reads or deletes another
   attempt's staged material, so concurrent replacements cannot interfere with
   each other's candidates.
3. **Validate.** The staged value is resolved server-side and validated
   against the profile's approved endpoint with the read-only prober - the same
   pinned peer, TLS and SSRF controls the probe route uses. Validation proves
   **protected authorization**, not endpoint reachability: the protected
   read-only listing for the identified edition (InfluxDB 2 `/api/v2/buckets`,
   InfluxDB 3 `/api/v3/configure/database`) must refuse an **anonymous**
   request and accept the **candidate**. An endpoint that serves the listing to
   anonymous callers authenticates nothing, so it cannot prove the candidate;
   an edition-identifying `/health` answer is public on InfluxDB 2 and proves
   nothing by itself. An anonymous, denied or inconclusive answer **fails
   closed**: the staged material is removed and the credential that was in
   place stays usable (422).
4. **Swap.** The value behind the **original** reference is replaced by a
   conditional update keyed on the observed ciphertext. If a concurrent
   replacement changed the credential first, nothing is written and the
   attempt reports a conflict (`credential-replace-conflict`, 409); the
   credential that is in place is whatever the winning attempt left. The
   reference ID, the profile and the approval are untouched, so the approval
   stays valid and no unrelated profile is interrupted.
5. **Revoke.** This attempt's staged material - and only this attempt's - is
   removed, whatever the outcome.

A candidate can only ever prove **authenticated read access** by a read-only
check. Whether it also holds write or admin privileges cannot be proven without
mutating the historian, which no replacement ever does; the validation
therefore never claims more than authenticated read access, and an operator
must confirm write-scoped tokens out of band before relying on them for writes.

**Revocation** is refused with 409 (`credential-in-use`) while **anything**
still depends on the credential: an approved binding that approves it, or a
profile that names it. A revocation never silently detaches a profile; the
dependency is removed first (revoke the approval and point the profile at
another credential, or delete the profile). The dependency checks, the reference
removal and the audit event are one transaction, so a concurrent approval
cannot interleave with a revocation: an approval either commits before the
check (the revocation is refused) or finds the reference gone (it fails with
404). The metadata transition commits **before** the secret material is
erased, so an interruption between the two leaves an unreferenced, unreachable
envelope - never a profile pointing at a deleted secret, and never a working
secret erased while a dependent still exists. Revoking a reference whose
metadata is already removed is an idempotent no-op that still removes any
envelope a prior interrupted attempt left behind.

Every transition is audited (`credential.enroll`, `credential.replace`,
`credential.replace.rejected`, `credential.replace.conflict`,
`credential.revoke`, and the rollbacks) with the reference, never with the
value.

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

### Credential-lifecycle hardening evidence (P2C-007, recorded 2026-10-21)

**Environment:** branch `phase-2/registry-completion`; Go 1.27.3 (linux/amd64
for the race suite); Node.js 24.15.0 / npm 11.12.1; Windows development host
with a WSL Ubuntu 22.04 runtime; the isolated containers ran under the WSL
Docker daemon with the digest-pinned images the harness uses.

**Commands and actual outcomes:**
- `gofmt -l .` → clean; `go vet ./...` and `go vet -tags integration ./...` →
  pass; `go build ./...` and `go build -tags integration ./...` → pass.
- `go test ./... -count=1` (linux/amd64) → every package `ok`.
- `go test -race ./... -count=1` (linux/amd64) → every package `ok`.
- `npm ci && npm run lint && npm run build && npm test` → 0 vulnerabilities,
  lint clean, build clean, 20 files / 188 tests passed.
- `bash scripts/policy-check.sh` → **PASS**.

**Acceptance mapping (P2C-007):**
- Replacement validation proves **protected authorization** →
  `liveprobe.TestVerifyAcceptsAnAuthorizedCandidate`,
  `TestVerifyRefusesAWrongCandidate`,
  `TestVerifyFailsClosedWhenTheProtectedListingServesAnonymous`,
  `TestVerifyFailsClosedWhenTheProtectedListingIsInconclusive`,
  and the API-level `registryapi.TestCredentialReplacementFailsClosedForAnUnauthorizedCandidate`
  (a wrong token against a public `/health` never replaces the usable
  credential).
- Unique per-attempt staging and a compare-and-swap swap →
  `secretstore.TestReplaceIfCurrentConflictsOnAStaleObservation`,
  `TestReplaceIfCurrentConcurrentAttemptsHaveOneWinner`,
  `metadatastore.TestEnvelopeReplaceIfAppliesOnlyOnTheExactCiphertext`,
  `registryservice.TestReplacementStageReferencesAreUniqueAndCleanupIsIsolated`
  and `registryapi.TestConcurrentReplacementsHaveOneWinner` (1 × 200, 1 × 409).
- Failures, retries and restart →
  `registryservice.TestReplacementFailsClosedAndPreservesTheOldCredential`,
  `TestReplacementRetrySucceedsAfterAFailure`,
  `TestReplacementAfterAnInterruptedAttemptSucceeds`,
  `TestReplacementSurvivesRestart`.
- Refuse-and-preserve revocation, idempotent completion, recovery →
  `registryapi.TestCredentialRevocationIsRefusedWhileAnythingReferencesIt`,
  `registryservice.TestRevocationRefusesAndPreservesWhileAnythingReferences`,
  `TestRevocationOfAnUnreferencedCredentialRemovesEverything`,
  `TestRevocationIsIdempotent`,
  `TestRevocationRecoveryAfterAnEnvelopeDeletionFailure`,
  `TestRevocationFailureInsideTheUnitOfWorkPreservesEverything`,
  `TestConcurrentBindAndRevokeCannotBypass`.
- P2L-006/P2L-007 preserved → `liveprobe.TestVerifyAppliesTheProductionEndpointPolicy`,
  the unchanged peer-pinning and SSRF tests, the administrator-only role
  denials, and the no-value-in-response/audit assertions.
- Against the **real, isolated** containers: the protected listing refused an
  anonymous request (401) on both editions, accepted the valid candidate
  (200 → authorized) and refused the wrong candidate (401 → not authorized on
  InfluxDB 2; the deployment authenticates every request, so InfluxDB 3 fails
  closed at edition identification). The full lifecycle — enrol → refused
  invalid replacement (422) → authorized replacement (200) → refused revocation
  while referenced (409) → revocation once unreferenced (204) — was executed
  end to end against both editions. **14 of 14 checks passed, 0 failed.**

**Limitations recorded (not papered over):** the verification proves
authenticated **read** access only; write-scoped privileges cannot be proven
read-only and are not claimed. The gated tests ran from inside each
container's network namespace because Docker 29's loopback port publishing is
broken in this WSL runtime (the harness scripts and compose file are
unchanged); the host-side baseline matrix (`scripts/run-tests.sh`) therefore
was not executed here, and its recorded evidence stands from P2C-006. InfluxDB
1.x remains unverified.

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
