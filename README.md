# Telegraf Toolkit

**Configure. Map. Synchronize.**

Visual Telegraf management, configuration, canonical data mapping, and cross-version InfluxDB synchronization toolkit.

> **Status:** Phases 0–4 are merged into `main`, including Phase 4's P4-001–P4-007 and P4-007-FU1 audit remediation. The Pipeline Designer supports typed graph and manual TOML configurations for a verified Telegraf 1.40.1 plugin subset, revisions and approval, syntax-validated manual TOML, and fail-closed compilation of supported Phase 3 mapping references. Manual syntax validation is **not** full Telegraf plugin-semantic validation, and credential detection is bounded. Pipeline mapping-pin editing, loading existing server revisions in the UI, unsupported transforms (including unit scaling and tag filters), live runtime deployment/management, historian synchronization, InfluxDB 1.x coverage, host-agent interface and production packaging remain incomplete or unverified. See [Roadmap](docs/ROADMAP.md).

## Vision

Telegraf Toolkit provides a unified control plane for configuring telemetry pipelines and repairing historical gaps. Telegraf remains the collection/processing/output runtime; the Toolkit manages configuration, visual mapping and cross-version historian synchronization.

## Planned capabilities

1. **Telegraf Manager:** Manage instances, inspect runtime status/logs and handle versioned configuration deployment.
2. **Configuration Editor:** Visual input -> processor -> output pipeline builder, with raw TOML editing, validation and rollback.
3. **Database Synchronizer:** Drag databases, measurements, or selected fields into sync jobs. Compare counts by time block **per measurement**, mark affected blocks, selectively backfill and verify.
4. **Canonical Registry / Taglist Mapper:** Browse measurements, fields and tags; drag into destination measurements; rename fields for canonical compliance; transform units; import/export reusable canonical schemas and mapping profiles.

## Proposed future direction — Device Intelligence & Community Library

A future, **not yet authorized or implemented** extension would add device-first discovery (SNMP OIDs, OPC UA nodes, Modbus model register maps and SunSpec models), selectable telemetry points, and an offline-capable community-maintained library of provenance-tracked, sanitized device profiles. See the proposed Phase 8 in the [Roadmap](docs/ROADMAP.md).

## Design commitments

- Live collection and historical synchronization have independent lifecycles.
- Copy/repair is **non-destructive by default**, with explicit job review and bounded resource usage.
- Counts detect candidates for repair but **do not prove point-level equivalence**.
- Source query and destination write compatibility is evaluated per InfluxDB version and deployment capability.
- Reusable mapping definitions must behave consistently for live pipelines and historical backfills.
- The UI distinguishes InfluxDB measurements, fields and tags even if the browser is called a Taglist.

## Documents

- [Architecture](docs/ARCHITECTURE.md) — components and boundaries
- [Synchronization design](docs/SYNCHRONIZATION.md) — time blocks, state machine, verification and recovery
- [Canonical mapping](docs/CANONICAL_MAPPING.md) — schemas, field/tag renaming and portable profiles
- [Database registry and credentials](docs/DATABASE_REGISTRY.md) — connection profiles, capability discovery and protected API keys
- [Frontend architecture](docs/FRONTEND_ARCHITECTURE.md) — React/TypeScript stack, module structure and build phases
- [UI design](docs/UI_DESIGN.md) — navigation, database registry, credentials and visual workflows
- [Roadmap](docs/ROADMAP.md) — proposed delivery phases
- [Domain/API contract inventory](docs/contracts/README.md) — Phase 0 contract catalog
- [Developer setup and fixtures](docs/DEVELOPER_SETUP.md) — reproducible local setup and isolated mock fixtures
- [Phase 0 scaffold notes](docs/DEVELOPMENT.md) — scaffold layout, pinned versions and commands

## Runtime version policy

The initial development/test baseline is the **official Telegraf 1.40.1 release**. Telegraf Toolkit is not a Telegraf fork. All deployment images and runtime dependencies must use **fixed versions**, never floating tags such as `latest`. Production Docker image references should additionally use verified SHA256 digests. Upgrades are explicit and tested. See [ADR-0001](docs/adr/0001-official-telegraf-and-version-pinning.md).

## Technology status

Frontend direction: **React + TypeScript SPA** with a **Go backend**. Exact dependency versions are pinned when scaffolding. The control-plane metadata store is implemented as persistent SQLite per [ADR-0002](docs/adr/0002-metadata-store-selection.md) ([`docs/METADATA_STORE.md`](docs/METADATA_STORE.md)); the envelope-encrypted secret store per [ADR-0003](docs/adr/0003-secret-store-integration.md) is implemented ([`docs/SECRET_STORE.md`](docs/SECRET_STORE.md)), together with a server-authorized Registry API, read-only live capability probing and an administrator-only credential attachment binding ([`docs/REGISTRY_API.md`](docs/REGISTRY_API.md)). Deployment packaging, the host-agent/remote management interface, and Telegraf integration details remain to be finalized.

## License

Licensed under the [Apache License 2.0](LICENSE).
