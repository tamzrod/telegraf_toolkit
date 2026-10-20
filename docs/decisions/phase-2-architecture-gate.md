# Phase 2 architecture decision gate

Status: **Decision checklist — no architecture decision is made or implied by this document.**
Task: P2-001 (Architecture decision gate and Registry contract inventory).

This is a human-readable checklist for the decisions that gate Phase 2 Database Registry work. It restates the open choices from the **Proposed** ADRs and records exactly which Phase 2 work is blocked until a human accepts a decision. Reading this document, or the contract refinement in [`docs/contracts/registry.md`](../contracts/registry.md), does not accept any ADR.

- Authority for scope and invariants: [`IDENTITY.md`](../../IDENTITY.md).
- Domain inventory: [`docs/contracts/README.md`](../contracts/README.md).
- Proposed (not accepted) decisions: [`ADR-0002`](../adr/0002-metadata-store-selection.md), [`ADR-0003`](../adr/0003-secret-store-integration.md).
- Registry design intent: [`docs/DATABASE_REGISTRY.md`](../DATABASE_REGISTRY.md).

## Gate 1 — Metadata store (ADR-0002, Proposed)

**What is undecided:** which store persists connection-profile metadata, logical scopes, capability-probe results, credential *references*, canonical schemas, mapping profiles, runtime revisions, sync jobs/blocks, verification states and audit events.

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

## Gate 2 — Secret store and key custody (ADR-0003, Proposed)

**What is undecided:** how credential values and runtime secrets are stored, referenced, resolved and rotated, and how encryption keys are custodied separately from ciphertext.

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

## What is permitted while both gates remain unresolved

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

## Exact Phase 2 work blocked by an unaccepted decision

| Work | Blocked by | Nature of the block |
| --- | --- | --- |
| Persistent profile/scope/job/audit storage and migrations | ADR-0002 | Cannot pick a store, schema or migration tooling |
| Capability-probe result persistence and freshness/TTL storage | ADR-0002 | Result storage shape depends on the store |
| Real credential storage, resolution, rotation, revocation | ADR-0003 | Cannot pick a custody/encryption mechanism |
| Runtime secret injection and config-secret delivery | ADR-0003 | Delivery mechanism depends on the secret store |
| Live capability probing against real InfluxDB 1.x/2.x/3.x | ADR-0003 (credentials) | Needs real scoped credentials to authenticate a probe |

Work **not** blocked (interface/mock only): contract declarations, in-memory mock adapters, isolated capability fixtures, endpoint/TLS/SSRF validation logic, sanitized-error handling, audit *event shape* validation and demo UI.

## Stop conditions for agents

- If a task's objective requires one of the blocked items above, stop that part and report the dependency rather than accepting an ADR or guessing a mechanism.
- Do not mark ADR-0002 or ADR-0003 as Accepted. Acceptance is human-owned.
- Do not treat this checklist, the ADR recommendations or any fixture as an accepted decision.
