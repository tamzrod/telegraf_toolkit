# Telegraf Toolkit — Project Identity

## Identity
**Name:** Telegraf Toolkit  
**Repository:** https://github.com/tamzrod/telegraf_toolkit  
**Tagline:** Configure. Map. Synchronize.  
**Status:** Phases 0–4 are merged into `main`, with scope limitations documented in [`docs/ROADMAP.md`](docs/ROADMAP.md). The Phase 3 Canonical Mapper and Phase 4 Pipeline Designer are implemented for focused, tested workflows; runtime management, historian synchronization and production packaging remain future work. InfluxDB 1.x coverage and several advanced mapping/UI capabilities remain unverified or unsupported.
**License:** Apache License 2.0.

Telegraf Toolkit is an open-source, browser-based management and configuration platform for Telegraf, with canonical data mapping and cross-version InfluxDB historical synchronization.

## Product pillars

1. **Telegraf Manager:** Manage multiple isolated Telegraf runtimes, their configurations, logs, health, restart/revisions and deployment.
2. **Visual Configuration Editor:** Construct Telegraf input → processor → output pipelines with graphical editing plus native TOML preview, validation and rollback.
3. **Database Synchronizer:** Select source and destination InfluxDB connections, detect discrepancies by time blocks per measurement, mark missing or mismatched blocks, selectively backfill and verify.
4. **Canonical Registry and Taglist Mapper:** Browse actual database measurements, fields and tag keys after explicitly choosing **source and destination databases**. Visually map and rename destination fields, configure unit transformations, validate canonical compliance, and import/export reusable canonical schemas and mapping profiles.
5. **Database Registry and Credential Management:** Reusable InfluxDB connection profiles, capability probing, protected read/write credentials, rotation, permissions and audit.

## Architectural decisions — preserve these

- **Upstream Telegraf:** Use official Telegraf binaries/container images; no maintained fork or custom compilation by default.
- **Initial Telegraf baseline:** **1.40.1**; any change must be explicit and tested.
- **Immutable dependency policy:** **Never use `latest` or floating tags/versions.** Pin explicit tested dependency versions and lockfiles; production images additionally use verified SHA256 digests.
- **One control plane, multiple runtimes:** Telegraf Toolkit manages many independent Telegraf processes/containers. One runtime equals one container by default; a runtime may host several plugins within the same failure domain.
- **Single-host runtime management:** Docker Compose is sufficient; no Docker Swarm or Kubernetes merely to run dozens of containers. The design does not impose a hard container-count limit; plan capacity from real CPU, memory, storage, network and device-polling load. Consider cluster orchestration only for genuine multi-host/HA requirements.
- **Isolation:** A Telegraf plugin/process failure must not require restarting unrelated collector runtimes. Independent historical backfill must not disrupt live telemetry.
- **Independent synchronizer:** Telegraf may provide compatible data transport, but sync job scheduling, queries, block index, conflict policies, retries and verification remain Toolkit responsibilities.
- **Non-destructive by default:** Drag-and-drop creates drafts; saving a mapping does not deploy a live pipeline or start historical sync. Source data is unchanged unless explicitly authorized.
- **Verification:** Equal point counts indicate only a count match, not complete data integrity. Preserve timestamps, tags and field types, and support deeper point-level verification.
- **Cross-version adapters:** Support InfluxDB 1.x/2.x/3.x according to actual detected query/write capabilities, not assumed universal compatibility.
- **Credentials:** Database profiles store secret references, never plaintext credentials. Backend enforces role permissions. The browser must never directly access privileged database credentials or Docker daemon control. Do not export keys in canonical schemas, TOML revision previews or Git.
- **Portable canonical contracts:** Canonical schemas and source-specific mapping profiles are separate, versioned import/export artifacts. Live ingestion and historical backfill must apply mappings consistently.

## Selected application stack

- **Frontend:** React + TypeScript single-page application.
- **Build:** Vite.
- **UI:** Tailwind CSS + shadcn/ui.
- **Visual pipelines:** React Flow (`@xyflow/react`).
- **Field/measurement drag-and-drop:** dnd-kit.
- **TOML editor:** Monaco Editor.
- **API state:** TanStack Query.
- **Backend direction:** Go API and background workers.
- **Deployment:** Toolkit management application separately deployed from isolated Compose-managed official Telegraf runtime containers.
- **Still to decide:** Host-agent privilege interface (ADR-0004), production packaging (ADR-0005), and later plugin/device discovery adapters. Metadata store (SQLite), secret-store architecture and dependency pinning are established.

## Primary user workflows

### Live data collection
Register database and credential → configure Telegraf input, processing and output plugins → validate/generated TOML → assign to isolated runtime → review diff → explicitly deploy → monitor.

### Canonical mapping
Choose **Source Database** and **Destination Database** from registry, including database/bucket scope → browse real source measurements/fields/tag keys and destination schema → drag selected fields/tags → rename canonical destination fields and configure transforms → validate → save mapping profile. Apply to live pipeline or historical sync only through a separate approved action.

### Historian reconciliation
Choose source and target → choose measurement/field selection, mapping, time span, block size and verification policy → create reviewed sync job → compare counts per measurement and UTC half-open time block → mark discrepancies → selective repair → independently verify and report unresolved conflicts.

## Initial delivery order

1. Application shell, navigation and mock workflows.
2. Database Registry and credential references.
3. Dual-database Taglist / Canonical Mapper.
4. Pipeline Designer and TOML validation.
5. Multi-container Telegraf Runtime Manager.
6. Synchronizer and time-block explorer.
7. Security hardening, version compatibility tests, packaging and operational validation.

## Agent/developer working rules

- Follow existing specifications before coding. Do not silently overturn an accepted decision; propose an ADR when needed.
- Work in small independently testable modules; keep collection/control-plane/sync failures isolated.
- Never claim a mock UI is a live backend connection or count-match is verified equivalence.
- Do not create or modify production deployments, deploy configs, secrets, or source historians without explicit instructions.
- Favor tested behavior and transparent operational status over decorative UI.
- Keep documentation synchronized with architectural changes.

## Canonical project references

- [README.md](README.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Runtime/version ADR](docs/adr/0001-official-telegraf-and-version-pinning.md)
- [Frontend architecture](docs/FRONTEND_ARCHITECTURE.md)
- [UI design](docs/UI_DESIGN.md)
- [Database registry and credential security](docs/DATABASE_REGISTRY.md)
- [Canonical mapping](docs/CANONICAL_MAPPING.md)
- [Historical synchronization](docs/SYNCHRONIZATION.md)
- [Roadmap](docs/ROADMAP.md)
