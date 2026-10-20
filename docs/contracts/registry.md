# Registry contract refinement (Phase 2)

Status: **Interface-only contract refinement — not implemented.**
Task: P2-001 (Architecture decision gate and Registry contract inventory).
Parent inventory: [`docs/contracts/README.md`](README.md) (`contract_inventory_version: 1`).

This document narrows the Database Registry slice of the parent inventory into interface-level contracts that can be implemented as declarations and in-memory mocks **without** accepting any ADR. It does not define a wire format, storage schema, migration, REST route or authentication mechanism, and it does not accept [`ADR-0002`](../adr/0002-metadata-store-selection.md) or [`ADR-0003`](../adr/0003-secret-store-integration.md). See [`docs/decisions/phase-2-architecture-gate.md`](../decisions/phase-2-architecture-gate.md) for what remains blocked.

Coverage in this refinement: **profile identity and scope**, **probed capabilities**, **credential references**, **sanitized errors** and **audit semantics**. Each section states what is fixed now (an invariant) versus what stays open until a decision is accepted.

## Status vocabulary

| Label | Meaning |
| --- | --- |
| **Fixed** | An invariant already established by `IDENTITY.md`, an accepted ADR or a cited design doc. Implementations must not contradict it. |
| **Interface** | A shape that may be declared and mocked now, but is explicitly provisional and not a frozen wire format. |
| **Open** | Blocked on an unaccepted decision; must not be frozen or persisted yet. |

## 1. Profile identity and scope

**Fixed invariants**

- A profile has a stable `connection_id` that is unique within the registry and never reused after deletion.
- A profile is **direction-neutral**: it may act as source, destination or both. Direction is assigned by the job or mapping that references it, never stored on the profile.
- A profile stores a **credential reference**, never a plaintext credential.
- Every profile carries a `mock` discriminator so synthetic entries can never be mistaken for a live registration.

**Interface shape (provisional)**

```text
ConnectionProfile
  connection_id      stable string identity
  display_name       human label; mock entries are prefixed "MOCK"
  family             product family (e.g. influxdb)
  major_version      integer major version label (a hint, not a capability claim)
  scope              LogicalScope
  capabilities       ProbedCapabilities
  credential_ref_id  opaque reference into a secret store
  health             SanitizedHealth
  mock               boolean discriminator
```

```text
LogicalScope (exactly one kind is set)
  kind = database-retention-policy   1.x: database + retention_policy
       | organization-bucket         2.x: organization + bucket
       | database                    3.x: database
```

**Open**

- Final field names, types and required/optional set.
- Whether a profile may hold multiple scopes.
- Site/plant grouping cardinality.
- Where the profile is persisted ([ADR-0002](../adr/0002-metadata-store-selection.md)).

**Rules**

- `major_version` is a **label only**. Capability decisions must read `capabilities`, never the version.
- The canonical scope string form is `<database>/<retention_policy>`, `<organization>/<bucket>` or `<database>`. It is a stable identity for comparison, not a display format.

## 2. Probed capabilities

**Fixed invariants**

- Capabilities are **probed per connection**, never inferred from `major_version`.
- Capabilities are evaluated **per side**: read/query capabilities for the source, write capabilities for the destination. A job is offered only when both sides satisfy their requirement.
- Probing performs **no writes** and uses scoped credentials.

**Interface shape (provisional)**

```text
ProbedCapabilities
  query_dialects    set of {influxql, flux, sql}
  write_protocols   set of {line-protocol, ...}
  schema_discovery  boolean
```

**Open**

- Capability matrix schema and the version/edition support matrix (ADR-E, not yet proposed).
- Probe result freshness/TTL and re-probe triggers (storage depends on ADR-0002).
- How "unsupported" is distinguished from "unknown".
- Live probing, which needs real scoped credentials ([ADR-0003](../adr/0003-secret-store-integration.md)).

**Rules**

- A capability gap is reported as an explicit, typed error (see [section 4](#4-sanitized-errors)), never as a silent fallback or a version assumption.
- Mock capability sets are labeled mock and must not be presented as live probe results.

## 3. Credential references

**Fixed invariants**

- A profile and any job store a credential **reference**, never a secret value.
- Secret values are write-only from the UI and resolved only server-side or in an authorized agent/worker.
- No secret value may appear in API responses, logs, exports, TOML revisions, URLs, browser storage or Git.
- A reference carries a role: `read`, `write` or `admin`. Source reads prefer `read`; destination writes use a restricted `write` reference.

**Interface shape (provisional)**

```text
CredentialRef
  credential_ref_id  opaque stable identifier
  alias              human label
  role               read | write | admin
  fingerprint        non-reversible verification token (algorithm Open)
  last_rotated       timestamp metadata
  expires_at         optional timestamp metadata
  mock               boolean discriminator
```

**Open**

- Secret-store integration and encryption-key management ([ADR-0003](../adr/0003-secret-store-integration.md)).
- Fingerprint/verification algorithm.
- Rotation/expiry semantics and in-flight job handling.
- Dependency-inspection query shape.

**Rules**

- A `CredentialRef` never contains a value field. Any adapter that would return a value is out of contract and must not be implemented before ADR-0003 is accepted.
- Mock references are clearly labeled and point to no real store.

## 4. Sanitized errors

**Fixed invariants**

- Errors surfaced to the browser or logs are **sanitized**: no secret values, tokens, passwords or credential-bearing URLs.
- A failed TLS validation is visible and is never silently bypassed.
- Endpoint validation is SSRF-aware.

**Interface shape (provisional)**

```text
SanitizedError
  code       stable machine code (e.g. unsupported-capability, tls-validation-failed)
  message    human-readable, secret-free
  detail     optional non-secret context
```

**Error codes (provisional)**

| Code | Meaning |
| --- | --- |
| `unsupported-capability` | The connection lacks a required dialect or write protocol |
| `source-destination-not-distinct` | Source and destination resolve to the same connection and scope |
| `selection-incomplete` | A required side was not selected |
| `tls-validation-failed` | TLS verification failed and was not bypassed |
| `endpoint-rejected` | The endpoint failed SSRF-aware validation |

**Open**

- Final code set and stability guarantees.
- Where errors are persisted, if at all (depends on ADR-0002).

**Rules**

- Sanitization is enforced server-side. UI visibility is not an authorization boundary.
- An error must never embed the rejected credential, even partially.

## 5. Audit semantics

**Fixed invariants**

- Audit records **who** did **what** to **which reference**, with an outcome and timestamp, and never records a secret value.
- Audit events reference other entities by ID (profile, credential reference, job, mapping, runtime).
- Backend authorization is authoritative; the UI does not enforce it.

**Interface shape (provisional)**

```text
AuditEvent
  actor          identity reference (identity source Open)
  action         verb (e.g. profile.create, credential.rotate, job.run)
  target_kind    profile | credential_ref | job | mapping | runtime
  target_id      reference ID
  outcome        succeeded | failed | denied
  at             timestamp
  correlation_id optional cross-module correlation token
```

**Open**

- Event schema, retention and immutability mechanism (depends on ADR-0002).
- Actor identity source (depends on the authentication mechanism, not yet decided).
- Correlation/ID conventions across modules.

**Rules**

- A credential-related audit event records the **reference**, never the value, in any field including `detail`.
- Mock audit events are labeled mock and are not evidence of a real action.

## Cross-cutting rules

- **Invariants are fixed; shapes are provisional.** Nothing in the "Interface shape" blocks is a frozen wire format, Go type, REST route or database schema.
- **Source and destination are always explicit and distinct.** No default or hard-coded side.
- **No secrets anywhere.** No contract, example or export in this document contains a credential value, token or credential-bearing URL.
- **Mock is labeled.** Every synthetic profile, capability set, reference and audit event carries `mock: true` and is visibly labeled.
- **`COUNT_MATCH` ≠ `VERIFIED`** remains governed by [`docs/SYNCHRONIZATION.md`](../SYNCHRONIZATION.md) and is unaffected by this refinement.

## Verification evidence

- Coverage: profile identity/scope, probed capabilities, credential references, sanitized errors and audit semantics are each fixed/interface/open separated.
- ADR-0002 and ADR-0003 remain **Proposed**; no storage, key custody or live-secret behavior is specified.
- No credentials, tokens or secret values appear in this document.
- Runtime tests: interface-level tests accompany the backend declarations; see the archived task evidence.
