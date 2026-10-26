# Phase 2 architecture decision gate

Status: **Decision checklist — no architecture decision is made or implied by this document.**
Task: P2-001 (Architecture decision gate and Registry contract inventory).
Gate status: Gate 1 and Gate 2 were **closed by human acceptance on 2026-10-21**
(ADR-0002 and ADR-0003 are **Accepted**). ADR-0004 (host-agent privilege
boundary) and ADR-0005 (production packaging) remain **Proposed**.

This is a human-readable checklist for the decisions that gated Phase 2 Database Registry work. It restates the open choices from the ADRs and records exactly which Phase 2 work was blocked until a human accepted a decision. Reading this document, or the contract refinement in [`docs/contracts/registry.md`](../contracts/registry.md), does not accept any ADR.

- Authority for scope and invariants: [`IDENTITY.md`](../../IDENTITY.md).
- Domain inventory: [`docs/contracts/README.md`](../contracts/README.md).
- Accepted decisions: [`ADR-0002`](../adr/0002-metadata-store-selection.md) and [`ADR-0003`](../adr/0003-secret-store-integration.md) (both accepted 2026-10-21, both implemented).
- Still **Proposed** (not accepted): [`ADR-0004`](../adr/0004-host-agent-privilege-boundary.md), [`ADR-0005`](../adr/0005-initial-production-packaging.md).
- Registry design intent: [`docs/DATABASE_REGISTRY.md`](../DATABASE_REGISTRY.md).

## Gate 1 — Metadata store (ADR-0002, **Accepted** 2026-10-21)

**Outcome:** the human accepted the single-host embedded SQLite path (option A).
ADR-0002 is **Accepted** and the persistence boundary is implemented in
`backend/internal/metadatastore` ([`docs/METADATA_STORE.md`](../METADATA_STORE.md)).
The options below are kept as the decision record.

**What was undecided:** which store persists connection-profile metadata, logical scopes, capability-probe results, credential *references*, canonical schemas, mapping profiles, runtime revisions, sync jobs/blocks, verification states and audit events.

| Option | Summary | Blocks |
| --- | --- | --- |
| A | Embedded relational store (e.g. a pinned SQLite-class engine, file-based) | Single-writer concurrency limits; migration tooling must be validated |
| B | External relational server (e.g. a pinned PostgreSQL) | Extra service to operate, pin, back up and secure |
| C | Document/embedded key-value store | Weak relational integrity for cross-entity references and audit queries |
| D | Time-series store | Wrong tool for transactional metadata; couples control plane to a data plane |

**Recommendation in ADR-0002:** staged A→B. **Proposed only.**

**Human decision required:**

- [ ] Accept A, B, or the staged A→B path as the target.
- [ ] State whether multi-host management is in scope for the next phase (biases toward B).

## Gate 2 — Secret store and key custody (ADR-0003, **Accepted** 2026-10-21)

**Outcome:** the human accepted application-level envelope encryption with
externally supplied keys (option A behind a `SecretStore` abstraction).
ADR-0003 is **Accepted** and the secret store is implemented in
`backend/internal/secretstore` ([`docs/SECRET_STORE.md`](../SECRET_STORE.md)).
The options below are kept as the decision record.

**What was undecided:** how credential values and runtime secrets are stored, referenced, resolved and rotated, and how encryption keys are custodied separately from ciphertext.

| Option | Summary | Blocks |
| --- | --- | --- |
| A | Application-level envelope encryption in the metadata store, keys supplied externally | Toolkit owns careful key custody and rotation |
| B | External secret manager (Vault-class) | Additional service to operate and secure |
| C | Host filesystem secret files | Weaker audit/rotation ergonomics; permission-dependent |
| D | Cloud KMS / managed secret service | External dependency and network/auth surface |

**Recommendation in ADR-0003:** Option A behind a `SecretStore` abstraction. **Proposed only.**

**Human decision required:**

- [ ] Accept A or B as the initial default.
- [ ] Accept the key-custody and rotation procedure.
- [ ] State whether external secret-manager integration is required now or deferred.

## What was permitted while both gates remained unresolved (historical)

Permitted, because it commits to no unresolved architecture:

- Interface declarations and interface-only contract refinement ([`docs/contracts/registry.md`](../contracts/registry.md)).
- In-memory / mock adapters that satisfy those interfaces and are clearly labeled mock.
- Isolated per-connection capability fixtures and negative validation tests.
- Security-boundary and sanitized-error tests that use **no** real credential values.
- Clearly labeled demo UI backed by mock data.

Not permitted, because it presupposes an unaccepted decision:

- Selecting or depending on a concrete metadata store, migration set or schema.
- Storing, encrypting, resolving, displaying or rotating any real credential value.
- Choosing an encryption-key custody mechanism.
- Any live InfluxDB read/write, historian write/delete, Telegraf or Docker control.

## Exact Phase 2 work blocked by an unaccepted decision (at the time)

| Work | Blocked by | Nature of the block |
| --- | --- | --- |
| Persistent profile/scope/job/audit storage and migrations | ADR-0002 | Cannot pick a store, schema or migration tooling |
| Capability-probe result persistence and freshness/TTL storage | ADR-0002 | Result storage shape depends on the store |
| Real credential storage, resolution, rotation, revocation | ADR-0003 | Cannot pick a custody/encryption mechanism |
| Runtime secret injection and config-secret delivery | ADR-0003 | Delivery mechanism depends on the secret store |
| Live capability probing against real InfluxDB 1.x/2.x/3.x | ADR-0003 (credentials) | Needs real scoped credentials to authenticate a probe |

Work **not** blocked (interface/mock only): contract declarations, in-memory mock adapters, isolated capability fixtures, endpoint/TLS/SSRF validation logic, sanitized-error handling, audit *event shape* validation and demo UI.

**Current status:** every row above was unblocked by the 2026-10-21 acceptance of
ADR-0002 and ADR-0003. The persistence boundary (P2L-001), the envelope-encrypted
secret store (P2L-002), the authorized Registry API (P2L-003), read-only live
probing (P2L-004) and the P2L-006/P2L-007 security hardenings were implemented
and merged via PR #6. Runtime secret injection and config delivery remain
unimplemented and now depend on the still-Proposed ADR-0004 host-agent privilege
boundary, not on ADR-0002/0003.

## Stop conditions for agents

- If a task's objective requires one of the blocked items above, stop that part and report the dependency rather than accepting an ADR or guessing a mechanism.
- ADR-0002 and ADR-0003 are **Accepted** by human decision and are implemented; do not treat them as open.
- ADR-0004 and ADR-0005 remain **Proposed**. Do not mark them Accepted, and do not treat this checklist, the ADR recommendations or any fixture as an accepted decision.
