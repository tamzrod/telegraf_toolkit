# ADR-0002: Metadata Store Selection

- **Status:** Accepted
- **Date:** 2026-10-21
- **Scope:** Persistence of Toolkit control-plane metadata (connection profiles, logical scopes, capability probe results, credential *references*, canonical schemas, mapping profiles, runtime revisions, sync jobs/blocks, verification states, audit events).
- **Deciders:** Human architecture owner (acceptance required).

## Context

The Toolkit control plane must persist the domain entities cataloged in [`docs/contracts/README.md`](../contracts/README.md). [`IDENTITY.md`](../../IDENTITY.md) and [`docs/ARCHITECTURE.md`](../ARCHITECTURE.md) deliberately leave the metadata store undecided. [`docs/ARCHITECTURE.md`](../ARCHITECTURE.md) requires that configuration revisions and block-job state survive restart and support resume.

The store must be operable on a single host (see the Docker Compose single-host decision in [`docs/ARCHITECTURE.md`](../ARCHITECTURE.md)) without introducing cluster orchestration merely for metadata.

## Constraints

- Must persist versioned configuration revisions and sync block/job state with restart/resume support.
- Must never store plaintext secrets; it holds credential **references** only (see [ADR-0003](0003-secret-store-integration.md)).
- Must be pinned to an exact version with a committed lockfile/migration set; no floating tags.
- Must be operable within the single-host Docker Compose deployment; no requirement for a distributed database.
- Backup/restore must keep encryption keys separate from encrypted values ([`docs/DATABASE_REGISTRY.md`](../DATABASE_REGISTRY.md)).
- The Toolkit is a management plane; the metadata store must not become a data-plane dependency for Telegraf collection.

## Considered options

| Option | Summary | Trade-offs |
| --- | --- | --- |
| A. Embedded relational (e.g. SQLite file) | Zero external service; file-based; simple backups | Single-writer; concurrency limits under heavy job/block churn; migration tooling maturity varies |
| B. External relational server (e.g. PostgreSQL) | Strong concurrency, transactions, mature migrations | Adds a service to operate, pin, back up and secure; larger footprint |
| C. Document/embedded KV (e.g. BoltDB-style) | Simple key/value persistence | Weak query/relational integrity for cross-entity references and audit queries |
| D. Time-series store (e.g. InfluxDB) | Already a Toolkit domain | Wrong tool for transactional metadata; couples control plane to a data-plane product |

## Trade-offs

- Option A minimizes operational surface and fits the single-host target, but must be validated for concurrent sync-block writes and audit volume.
- Option B offers the strongest integrity guarantees and is the natural choice if multi-writer concurrency or multi-host management is anticipated, at the cost of an extra pinned, secured service.
- Options C/D are recorded for completeness but are not recommended for relational metadata with referential integrity needs.

## Security implications

- Whatever the store, it must not hold secret values; only references.
- Encryption keys must not live in the same backup as encrypted data.
- The store must be reachable only by the backend, never the browser.
- Audit events must be append-oriented and immutable in practice.

## Validation criteria

- A spike demonstrates restart/resume of an interrupted sync job using persisted block state.
- A concurrency test exercises concurrent block-state writes without corruption (specific to the chosen option).
- Backup/restore procedure is documented and rehearsed with keys separated from ciphertext.
- Chosen dependency is pinned to an exact version with committed migrations/lockfiles.

## Recommendation

**Accepted 2026-10-21 by human architecture owner.** A staged approach is selected: start with an embedded relational store (Option A) for the single-host foundation, and adopt an external relational server (Option B) only when validated concurrency, availability or multi-host requirements demand it. This keeps the initial deployment minimal while preserving a migration path. Implementation of Option A is authorized through separately promoted CWAL tasks; a later PostgreSQL migration needs a new explicit decision.

## Accepted decision and implementation limits

- **Option A: SQLite** embedded relational metadata persistence for the initial **single-host** Toolkit control plane, with versioned migrations, safe transactions, backup/restore tests and concurrency limits documented.
- Design the storage boundary so a future PostgreSQL adapter/migration is possible; PostgreSQL is **not** selected for the initial deployment.
- **Multi-host control-plane management is out of scope for this delivery** and requires a later explicit architecture decision.
- The metadata store holds credential **references only**, not secret values; see ADR-0003.
- This ADR acceptance is a design decision, not permission for production access. Tasks still require CWAL authorization.

## References

- [`docs/contracts/README.md`](../contracts/README.md) — domain inventory and ADR-A candidate
- [`docs/ARCHITECTURE.md`](../ARCHITECTURE.md) — persistence and single-host Compose decisions
- [`docs/DATABASE_REGISTRY.md`](../DATABASE_REGISTRY.md) — backup/encryption-key separation
- [`docs/ROADMAP.md`](../ROADMAP.md) — Phase 0 open decisions
