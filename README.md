# Telegraf Toolkit

**Configure. Map. Synchronize.**

Visual Telegraf management, configuration, canonical data mapping, and cross-version InfluxDB synchronization toolkit.

> **Status:** Early-stage delivery. The Phase 0 toolchain, the Phase 1 application
> shell, the Phase 2 Database Registry — including the administrator-only
> credential lifecycle — and the Phase 3 Canonical Mapper (measurement-level
> read path, schema browser, versioned canonical schemas and mapping profiles
> with approval/history/rollback and safe export) are implemented and merged
> into `main` (PRs #2–#8). The Phase 4 pipeline designer (P4-001–P4-007) is
> implemented and checkpointed on `phase-4/pipeline-designer`: typed graph and
> manual documents for a verified Telegraf 1.40.1 plugin subset, a
> browser-local TOML preview, immutable draft/approval revisions, explicit
> graph↔manual transitions with snapshots, and fail-closed compilation of
> approved Phase 3 mapping references. Phase 4 is **not merged**; human PR
> review remains required. No route deploys or restarts Telegraf, and no route
> writes a historian. Not implemented or unverified: runtime
> deployment/management, historical synchronization, unit scaling and tag
> filters in mapping compilation, a UI for editing mapping pins, InfluxDB 1.x
> coverage, production packaging and the host-agent interface. See
> [Roadmap](docs/ROADMAP.md) and the
> [Phase 3 gate record](docs/decisions/phase-3-schema-discovery-gate.md).

## Vision

Telegraf Toolkit provides a unified control plane for configuring telemetry pipelines and repairing historical gaps. Telegraf remains the collection/processing/output runtime; the Toolkit manages configuration, visual mapping and cross-version historian synchronization.

## Planned capabilities

1. **Telegraf Manager:** Manage instances, inspect runtime status/logs and handle versioned configuration deployment.
2. **Configuration Editor:** Visual input -> processor -> output pipeline builder, with raw TOML editing, validation and rollback.
3. **Database Synchronizer:** Drag databases, measurements, or selected fields into sync jobs. Compare counts by time block **per measurement**, mark affected blocks, selectively backfill and verify.
4. **Canonical Registry / Taglist Mapper:** Browse measurements, fields and tags; drag into destination measurements; rename fields for canonical compliance; transform units; import/export reusable canonical schemas and mapping profiles.

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
