# Domain and API Contract Inventory

Status: **Contract inventory proposal — the Registry slice is now implemented; the remaining domains are still planned.**
Task: P0-001 (Domain/API contract inventory).
Scope: planning/contract catalog. This document does **not** itself define final wire formats, Go types, REST routes, database schemas or authentication mechanisms. The Database Registry slice has since been implemented and merged (see [Phase 2 refinement](#phase-2-refinement)); the canonical schema, mapping profile, runtime revision and sync job/block domains remain planned.

## Purpose and authority

This file is a reviewable catalog of the domain entities and API boundaries required by Phase 0. It records **which** contracts must exist, **who owns** them, their **source/destination semantics**, their **security boundary**, and what remains **undecided**.

- Authority for product scope and invariants: [`IDENTITY.md`](../../IDENTITY.md).
- Detailed domain designs: [`docs/ARCHITECTURE.md`](../ARCHITECTURE.md), [`docs/DATABASE_REGISTRY.md`](../DATABASE_REGISTRY.md), [`docs/CANONICAL_MAPPING.md`](../CANONICAL_MAPPING.md), [`docs/SYNCHRONIZATION.md`](../SYNCHRONIZATION.md), [`docs/FRONTEND_ARCHITECTURE.md`](../FRONTEND_ARCHITECTURE.md), [`docs/ROADMAP.md`](../ROADMAP.md).
- Accepted runtime/version decision: [`docs/adr/0001-official-telegraf-and-version-pinning.md`](../adr/0001-official-telegraf-and-version-pinning.md) (ADR-0001).

Where this inventory and a cited source disagree on an established invariant, the source governs and the conflict is reported (see [Stop conditions](#stop-conditions)).

### Status vocabulary

Each contract below is classified so planned, decided and implemented behavior stay distinct (see `BLACK_SHEEP_WALL.md` rule 6):

| Label | Meaning |
| --- | --- |
| **Decided** | An accepted decision or invariant already fixes the concept. The contract must not contradict it. |
| **Planned** | The concept is required by a cited design document, but its concrete shape is not yet accepted. |
| **Unresolved** | A dependent choice is explicitly open and blocks finalizing the contract. |

No contract here was **Implemented** by this task. This catalog created no executable adapter, service, REST route or database schema. The **Database Registry slice** (connection profile, logical scope, credential reference, probed capabilities, audit event) has since been implemented and merged — see [Phase 2 refinement](#phase-2-refinement). The canonical schema, mapping profile, runtime revision and sync job/block domains remain **Planned**.

## Contract inventory (versioned)

The inventory itself is versioned as `contract_inventory_version: 1`. Individual artifacts carry their own `schema_version` (see each section). Versioning rules for the artifacts are recorded in [`docs/CANONICAL_MAPPING.md`](../CANONICAL_MAPPING.md) (portable artifacts) and remain to be finalized per artifact.

### 1. Connection profile

- **Definition:** A reusable registry entry describing how to reach one InfluxDB deployment: stable `connection_id`, display name, purpose/site grouping, family/major version, base URL(s), TLS settings, timeouts, logical scope, dialect/write capabilities, credential references, health and sanitized errors.
- **Source reference:** [`docs/DATABASE_REGISTRY.md`](../DATABASE_REGISTRY.md) "Database connection profiles"; [`docs/ARCHITECTURE.md`](../ARCHITECTURE.md) "Database Registry and Credential Manager".
- **Status:** **Planned** (attributes are "suggested" in the source, not yet accepted as a wire format).
- **Ownership:** Database Registry and Credential Manager (backend). UI is a consumer.
- **Source/destination semantics:** A profile is **direction-neutral**; the same profile may act as source, destination, or both. Direction is assigned by the job/mapping that references it, never by the profile itself.
- **Security boundary:** Server-side only. The profile stores **credential reference IDs, never plaintext credentials**. Endpoint validation must be SSRF-aware and TLS-verified by default. Browser receives metadata, health, fingerprints and sanitized errors only.
- **Unresolved schema details:**
  - Final field names, types and required/optional set.
  - Capability representation (see [Probed capabilities](#4-probed-capabilities)).
  - Site/plant grouping cardinality.
  - Whether a profile can hold multiple scopes.
  - Storage location depends on the metadata-store decision ([ADR candidate ADR-A](#adr-candidates)).

### 2. Logical database scope

- **Definition:** The concrete data namespace a profile targets within a deployment: InfluxDB 1.x database/retention policy, 2.x organization/bucket, 3.x database plus its available API conventions.
- **Source reference:** [`docs/DATABASE_REGISTRY.md`](../DATABASE_REGISTRY.md) (logical target scope); [`docs/UI_DESIGN.md`](../UI_DESIGN.md) "with the appropriate database/bucket scope on each side".
- **Status:** **Planned**.
- **Ownership:** Database Registry (backend); selected explicitly in the mapper and sync designer UI.
- **Source/destination semantics:** Scope is **selected independently per side**. The mapper and synchronizer must require an explicit **Source scope** and **Destination scope** before browsing or writing. No hard-coded or default scope.
- **Security boundary:** The selected scope constrains which credential permissions apply. A scope change must invalidate cached schema and pending drafts. Scope is metadata, not a secret.
- **Unresolved schema details:**
  - Canonical representation that spans 1.x retention policy / 2.x bucket / 3.x database.
  - Whether scope is embedded in the profile or a separate referenced entity.
  - Multi-scope profiles.

### 3. Credential reference

- **Definition:** A stable identifier that points to a secret held in a protected server-side secret store, plus non-secret metadata (alias, scope, read/write/admin role, age, expiry, last-rotated, fingerprint, usage dependencies).
- **Source reference:** [`docs/DATABASE_REGISTRY.md`](../DATABASE_REGISTRY.md) "Credential lifecycle"; [`docs/ARCHITECTURE.md`](../ARCHITECTURE.md) "Credential Manager".
- **Status:** **Decided** (invariant: profiles store secret *references*, never plaintext; browser never resolves raw secrets) with **Planned** concrete shape.
- **Ownership:** Credential Manager (backend, privileged).
- **Source/destination semantics:** A credential reference may be attached to **read (source)**, **write (destination)** and **administration** roles. Source reads prefer read-only credentials; destination writes use restricted write credentials. Jobs record which credential references they use **without serializing values**.
- **Security boundary:** Secret values are write-only from the UI, resolved only server-side or in an authorized host agent/worker, and injected into runtimes through a protected delivery mechanism. Never in Git, logs, exports, TOML revisions, browser storage, URLs or API responses. Rotation adds-and-validates the new credential before revoking the old.
- **Unresolved schema details:**
  - Secret-store integration and encryption-key management ([ADR candidate ADR-B](#adr-candidates)).
  - Fingerprint/verification algorithm.
  - Rotation/expiry semantics and in-flight job handling.
  - Dependency-inspection query shape.

### 4. Probed capabilities

- **Definition:** The set of query dialects (InfluxQL/Flux/SQL), write protocols, schema-discovery support, allowed operations and observed limits actually **detected** for a connection at registration/test time — not inferred from a version label.
- **Source reference:** [`docs/DATABASE_REGISTRY.md`](../DATABASE_REGISTRY.md) "Query dialect/capabilities: do **not** infer dialect support solely from major version"; [`docs/SYNCHRONIZATION.md`](../SYNCHRONIZATION.md) "Capability-check each connection before offering a job"; [`docs/ROADMAP.md`](../ROADMAP.md) Phase 2.
- **Status:** **Decided** (capability must be probed, not assumed) with **Planned** representation.
- **Ownership:** Capability probe / scoped query service (backend).
- **Source/destination semantics:** Capabilities are recorded **per connection** and evaluated separately for the **source side (read/query)** and **destination side (write)**. A job is offered only when both sides' required capabilities are present.
- **Security boundary:** Probing uses scoped credentials; probe errors are sanitized and never include secrets. Probing must not perform writes.
- **Unresolved schema details:**
  - Capability matrix schema and the version/edition support matrix ([ADR candidate ADR-E](#adr-candidates)).
  - Probe result freshness/TTL and re-probe triggers.
  - How "unsupported" vs "unknown" is represented.

### 5. Canonical schema

- **Definition:** A portable, versioned definition of an approved shared schema: measurements with canonical field names, expected types, units and tag keys.
- **Source reference:** [`docs/CANONICAL_MAPPING.md`](../CANONICAL_MAPPING.md) "Portable artifacts" and the illustrative example.
- **Status:** **Planned**. The example in the source is explicitly illustrative, not an established contract.
- **Ownership:** Canonical Registry (backend-owned artifact, user-editable via UI).
- **Source/destination semantics:** A canonical schema describes the **destination-side canonical target**, not a live connection. It is **separate from** mapping profiles (see next section) and is importable/exportable independently.
- **Security boundary:** Canonical exports must contain **no credentials**. Units and names are explicit; never inferred from field names alone.
- **Unresolved schema details:**
  - Final JSON/YAML field names and required structure.
  - Type system, unit vocabulary and validation rules.
  - Registry-level identity/versioning and approval workflow.
  - CSV field-list import/export subset and its limited metadata.

### 6. Mapping profile

- **Definition:** A source/site/device-specific adapter that maps source measurement/field/tag identities to canonical (or destination) identities, with explicit renames, unit transforms, tag rules, filters and collision handling.
- **Source reference:** [`docs/CANONICAL_MAPPING.md`](../CANONICAL_MAPPING.md) "Portable artifacts", "Contracts and safety"; [`docs/UI_DESIGN.md`](../UI_DESIGN.md) Taglist / Canonical Mapper.
- **Status:** **Planned**.
- **Ownership:** Mapping Registry (backend); authored in the mapper UI.
- **Source/destination semantics:** A profile **explicitly records both source and destination** connection references/scopes plus the mapping rules between them. Source and destination are always distinct and explicitly selected; changing either invalidates cached schema and draft mappings. The same mapping semantics must apply consistently to **live Telegraf pipelines and historical backfill**.
- **Security boundary:** Mapping exports contain **no secrets**. Type/collision validation occurs before save. Saving a mapping **must not** deploy a pipeline or start synchronization.
- **Unresolved schema details:**
  - Final field/rule grammar (rename, scale, tag value, filter, derived tag).
  - Transform expression/unit-conversion representation and safety limits.
  - Version diff/rollback representation.
  - Collision-handling configuration shape.

### 7. Runtime revision

- **Definition:** A versioned, immutable record of a Telegraf runtime's configuration state: the pinned runtime version, the TOML configuration revision, validation result, deploy/rollback state and audit linkage.
- **Source reference:** [`docs/ARCHITECTURE.md`](../ARCHITECTURE.md) "Telegraf Manager"; [`docs/ROADMAP.md`](../ROADMAP.md) Phase 4/5; [`docs/adr/0001-official-telegraf-and-version-pinning.md`](../adr/0001-official-telegraf-and-version-pinning.md).
- **Status:** **Planned**.
- **Ownership:** Instance/Config Service (backend) with the restricted host-agent/Docker controller (privileged).
- **Source/destination semantics:** Runtime revisions concern the **collection (data plane)** side, not historian sync. A revision is per-runtime. Saving/validating a revision is distinct from **explicitly deploying** it.
- **Security boundary:** Generated deployable configuration containing secrets must be protected and never checked into Git. Docker operations belong in a separately authorized service/host agent; the browser must never get Docker daemon control. Runtime versions are pinned to explicit patch versions; production images additionally use verified digests; never floating tags.
- **Unresolved schema details:**
  - Revision storage format and retention.
  - Secret-injection mechanism for runtime configs.
  - Host-agent privilege boundary and control interface ([ADR candidate ADR-C](#adr-candidates)).
  - Deployment packaging ([ADR candidate ADR-D](#adr-candidates)).

### 8. Sync job and UTC half-open block

- **Definition:** A reviewed job that reconciles historical data between an explicit source and destination: selection, mapping reference, time range, block size, direction, verification policy, schedule and status. A **block** is identified by `job + source measurement + mapped selection + [start,end)`.
- **Source reference:** [`docs/SYNCHRONIZATION.md`](../SYNCHRONIZATION.md) "Block identity", "Repair and resume"; [`docs/ROADMAP.md`](../ROADMAP.md) Phase 6.
- **Status:** **Decided** (blocks are per-measurement, UTC, half-open; drag creates a proposal not a write) with **Planned** concrete shape.
- **Ownership:** Sync Controller / background workers (backend), independent of live Telegraf pipelines.
- **Source/destination semantics:** Source and destination profiles are **distinct**; source is read-only, destination write is restricted. All boundaries are **UTC half-open `[start,end)`**; the in-progress/current block stays unfinalized until a lateness/grace window expires. Never delete destination-only data without an explicit separate policy.
- **Security boundary:** No writes to source historians. Retries must be idempotent; collision/duplicate/out-of-order handling must be defined before enabling overwrite. Backpressure and bounded batches prevent repair from starving live collection. Repair is non-destructive by default and dry-run capable.
- **Unresolved schema details:**
  - Job/block persistence schema ([ADR candidate ADR-A](#adr-candidates) metadata store).
  - Block state machine (see next section) persistence.
  - Conflict/collision policy representation.
  - Checkpoint/watermark format and resume semantics.
  - Block-size and scheduling representation.

### 9. Verification state

- **Definition:** The per-block status describing reconciliation progress and integrity, distinct from a raw count comparison.
- **Source reference:** [`docs/SYNCHRONIZATION.md`](../SYNCHRONIZATION.md) "Proposed block state machine"; [`docs/UI_DESIGN.md`](../UI_DESIGN.md) "Make count-matched status visually distinct from fully verified historical blocks".
- **Status:** **Decided** (invariant: `COUNT_MATCH` ≠ `VERIFIED`; equal counts never imply verified equivalence) with **Planned** persistence.
- **Ownership:** Sync Controller / verifier (backend); surfaced in the UI.
- **Source/destination semantics:** States describe a block's reconciliation between a specific source and destination. Proposed states:
  `PENDING -> SCANNING -> COUNT_MATCH | DIRTY | MISSING | ERROR`
  `DIRTY/MISSING -> QUEUED -> COPYING -> VERIFYING -> VERIFIED | NEEDS_REVIEW | ERROR`
- **Security boundary:** Verification is read-only against the source and read/limited-write against the destination; it must never mutate source historians. `COUNT_MATCH` must **not** be displayed as fully `VERIFIED` unless the requested verification policy is satisfied.
- **Unresolved schema details:**
  - Final state names and transitions.
  - Verification policy options and deeper point-level/digest comparison definitions.
  - Multi-field point counting semantics (rows vs field values vs unique series/timestamp identities).
  - How unresolved conflicts are recorded.

### 10. Audit event

- **Definition:** An immutable record of a user-initiated or system action: actor, action, target (profile/credential/runtime/job/mapping), outcome, timestamp and correlation to the affected references.
- **Source reference:** [`docs/DATABASE_REGISTRY.md`](../DATABASE_REGISTRY.md) "Audit: record creator, actor, action, credential *reference* and outcome without recording the secret value"; [`docs/ARCHITECTURE.md`](../ARCHITECTURE.md) "audit user-initiated actions".
- **Status:** **Decided** (audit must exist and must never record secret values) with **Planned** concrete shape.
- **Ownership:** Management API / RBAC and audit subsystem (backend).
- **Source/destination semantics:** Audit events are cross-cutting and reference other contracts by ID; they are not themselves source/destination-directional, but credential and sync events must record the involved source/destination references without their secret values.
- **Security boundary:** Events record credential **references only**, never secret values, tokens or passwords. Enforced server-side; backend authorization is authoritative, not UI visibility.
- **Unresolved schema details:**
  - Event schema, retention and immutability mechanism.
  - Actor identity source (depends on auth mechanism — not defined before accepted decisions).
  - Correlation/ID conventions across modules.

## Phase 2 refinement

The Database Registry slice of this inventory is refined at interface level in [`docs/contracts/registry.md`](registry.md) (P2-001). That document separates fixed invariants from provisional interface shapes and explicitly-open items, and is paired with the human decision checklist [`docs/decisions/phase-2-architecture-gate.md`](../decisions/phase-2-architecture-gate.md).

**Current implementation state (merged via PR #6, 2026-10-21):** ADR-0002 (SQLite metadata store) and ADR-0003 (envelope-encrypted secret store) were **accepted by the human on 2026-10-21** and are implemented in `backend/internal/metadatastore` and `backend/internal/secretstore`. The Registry contract is served by a server-authorized HTTP API (`backend/internal/registryapi`, [`docs/REGISTRY_API.md`](../REGISTRY_API.md)) with role-based permissions, a read-only live capability prober (`backend/internal/liveprobe`, [`docs/LIVE_PROBING.md`](../LIVE_PROBING.md)) and an administrator-only credential attachment binding ([`docs/REGISTRY_API.md`](../REGISTRY_API.md)). Authentication is **static file-provisioned bearer tokens** (no issuance, expiry, rotation or OIDC/OAuth); capability outcomes are projected from the detected edition rather than observed per capability; **InfluxDB 1.x remains unverified**. ADR-0004 and ADR-0005 remain **Proposed**.

## Phase 3 entry contract

The bounded contract the Canonical Mapper starts from - the source and
destination identity, the scope of the first increment, what the Registry can
prove today, the unknown states the mapper must model and the boundaries it
inherits - is recorded in
[`docs/contracts/canonical-mapper.md`](canonical-mapper.md). It implements
nothing and claims no capability the Registry has not observed.

## Cross-cutting contract rules

- **Planned vs decided separation.** Sections marked **Decided** carry invariants that must not be contradicted; **Planned**/**Unresolved** items must not be frozen into wire formats, Go types, REST routes, database schemas or auth mechanisms by this task.
- **Source and destination are always explicit.** Connection profiles, logical scopes, mapping profiles and sync jobs each select source and destination explicitly. No hard-coded or default side.
- **Portable artifacts stay separate.** Canonical schemas (destination canonical target) and mapping profiles (source-specific adapter) remain separate, independently versioned, importable/exportable artifacts.
- **`COUNT_MATCH` ≠ `VERIFIED`.** Count equality only indicates a count match; it never claims point-level equivalence.
- **No secrets in contracts or examples.** No contract, example or export in this inventory contains credentials, tokens or credential-bearing URLs.

## ADR candidates

The following architectural decisions gate one or more contracts above. They were recorded as candidates for the ADR task (P0-002) and are cross-linked, not decided, here. Sources: [`docs/ROADMAP.md`](../ROADMAP.md) "Open decisions" and Phase 0 remaining items; [`ICC/context/project.md`](../../ICC/context/project.md) "Open decisions".

| ID | Candidate decision | Blocks | Status |
| --- | --- | --- | --- |
| ADR-A | Metadata store selection | Connection profile, scope, sync job/block, verification state persistence | **Accepted** (ADR-0002, 2026-10-21) |
| ADR-B | Secret-store integration and encryption-key management | Credential reference, runtime secret injection | **Accepted** (ADR-0003, 2026-10-21) |
| ADR-C | Host-agent privilege boundary and control interface | Runtime revision, runtime control | **Proposed** (ADR-0004) |
| ADR-D | Initial production packaging | Runtime revision, deployment | **Proposed** (ADR-0005) |
| ADR-E | Version/capability matrix and concrete InfluxDB adapters | Probed capabilities, sync jobs | Not yet proposed |

This inventory does not accept any decision. Where an ADR has since been accepted by the human, the dependent Registry schema details are implemented (see [Phase 2 refinement](#phase-2-refinement)); where an ADR is still **Proposed**, the dependent schema details remain intentionally unresolved.

### ADR proposals (P0-002) and current status

| Candidate | Proposal | Status |
| --- | --- | --- |
| ADR-A | [ADR-0002 Metadata Store Selection](../adr/0002-metadata-store-selection.md) | **Accepted** 2026-10-21, implemented (P2L-001) |
| ADR-B | [ADR-0003 Secret-Store Integration](../adr/0003-secret-store-integration.md) | **Accepted** 2026-10-21, implemented (P2L-002) |
| ADR-C | [ADR-0004 Host-Agent Privilege Boundary](../adr/0004-host-agent-privilege-boundary.md) | **Proposed** — not accepted, not implemented |
| ADR-D | [ADR-0005 Initial Production Packaging](../adr/0005-initial-production-packaging.md) | **Proposed** — not accepted, not implemented |
| ADR-E | Not yet proposed (version/capability matrix) | Not proposed |

## Domain-coverage checklist

| # | Domain | Accounted | Source reference | Decided vs planned | Source/destination | Security boundary | Unresolved |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | Connection profiles | Yes | DATABASE_REGISTRY.md | Decided invariant / Planned shape | Direction-neutral | Server-side; no plaintext | Yes |
| 2 | Logical database scopes | Yes | DATABASE_REGISTRY.md, UI_DESIGN.md | Planned | Explicit per side | Permission-scoped | Yes |
| 3 | Credential references | Yes | DATABASE_REGISTRY.md | Decided / Planned shape | Read/write/admin roles | Write-only; never exported | Yes |
| 4 | Probed capabilities | Yes | DATABASE_REGISTRY.md, SYNCHRONIZATION.md | Decided / Planned shape | Per side (read/write) | Scoped, sanitized, no writes | Yes |
| 5 | Canonical schemas | Yes | CANONICAL_MAPPING.md | Planned | Destination canonical target | No credentials | Yes |
| 6 | Mapping profiles | Yes | CANONICAL_MAPPING.md | Planned | Explicit source + destination | No secrets; no deploy on save | Yes |
| 7 | Runtime revisions | Yes | ARCHITECTURE.md, ROADMAP.md, ADR-0001 | Planned | Collection/data plane | Protected configs; pinned versions | Yes |
| 8 | Sync jobs / UTC half-open blocks | Yes | SYNCHRONIZATION.md | Decided / Planned shape | Distinct source/destination | Non-destructive; idempotent | Yes |
| 9 | Verification states | Yes | SYNCHRONIZATION.md | Decided / Planned shape | Per source/destination block | Read-mostly; COUNT_MATCH≠VERIFIED | Yes |
| 10 | Audit events | Yes | DATABASE_REGISTRY.md, ARCHITECTURE.md | Decided / Planned shape | Cross-cutting | References only, no secrets | Yes |

**Coverage note (after PR #6):** rows 1, 2, 3, 4 and 10 now have an implemented
Registry backend — persistence, secret storage, the authorized API, live probing
and audit. Their "Planned shape" status above refers to this inventory catalog,
not to the shipped implementation. Rows 5, 6, 7, 8 and 9 remain planned.

## Stop conditions

- If cited documents conflict on an **established invariant**, stop and report the conflict rather than choosing an implementation.
- If finalizing any contract requires an unresolved ADR, stop and report it as an open decision.
- Ambiguous scope expansion, new privileged access or historian writes are blockers requiring human direction.

## Verification evidence

- Domain-coverage checklist above enumerates all ten required domains with source references and decided/planned separation.
- Changed paths: `docs/contracts/README.md` (added). No runtime, adapter, service or schema created.
- No credentials, tokens or secret values appear in this document.
- Runtime tests: not applicable (documentation-only output).
