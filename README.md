# Telegraf Toolkit

**Configure. Map. Synchronize.**

Visual Telegraf management, configuration, canonical data mapping, and cross-version InfluxDB synchronization toolkit.

> **Status:** Early planning and documentation. Features below are proposed; no operational toolkit is implemented yet.

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
- [Roadmap](docs/ROADMAP.md) — proposed delivery phases

## Technology status

Implementation language, UI framework, metadata store, deployment model and Telegraf plugin mechanisms are **not yet selected**. The initial focus is on defining requirements and safe interfaces.

## License

Licensed under the [Apache License 2.0](LICENSE).
