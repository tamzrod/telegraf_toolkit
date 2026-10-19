# UI/UX Specification — Telegraf Toolkit

Status: **Design concept, not implemented**.

## Navigation
- Overview — Runtime health, ingestion, sync progress
- Runtimes — Isolated Telegraf containers, logs, restart, pinned image version
- Pipeline Designer — Visual inputs -> processors -> outputs, TOML preview, validation, revisions
- Databases — Shared InfluxDB connection profiles, dialect capabilities, health and dependencies
- Credentials — Secret references, permission scopes, usage, rotation and revocation
- Taglist & Canonical — Measurement/field/tag explorer, field renaming, unit transforms, portable mappings
- Synchronizer — Source/target drag-and-drop, time-block heatmap, backfill and verification
- Audit / Settings — Change logs, permissions, version policy

## Database Registry screen
Searchable table with connection alias, host, InfluxDB version/edition, database/bucket scope, query/write capabilities, connection health and last tested timestamp. Actions: Add, Test, Edit, Inspect dependencies, Disable, Delete (after dependency review). Group profiles by site where useful.

Add/edit connection opens a drawer with identity, endpoint, InfluxDB version, scope, TLS trust and authentication method. Use credential references rather than secrets; perform tests on the backend. Test results must be sanitized.

## Credentials screen
Show alias, authentication type, associated profiles, read/write/admin scope, age, expiry, and runtime/job dependencies. Actions: Add, Rotate, Revoke, Inspect dependencies. Credential input is masked and write-only. No plaintext reveal in UI, logs, or exported documents. Preserve auditable changes.

## Visual workflows
- Drag a measurement from a source connection onto a destination to create a sync **draft**, not an immediate write.
- Drag fields/tags from the Taglist onto a destination measurement; edit canonical name and transformations before save.
- Assign pipelines to isolated Telegraf runtimes. Validate and diff TOML before apply.
- Make count-matched status visually distinct from fully verified historical blocks.
- Show loading, failure, permission and stale-data states. Prototype data must always be labeled as sample.

## Access model
Viewer: authorized metadata and health; Operator: permitted tests and sync jobs; Configurator: mappings and pipeline edits; Administrator: credentials, rotation and revocation. All enforced server-side.

## Acceptance checks
1. Registry supports InfluxDB 1.x, 2.x and 3.x profiles and capabilities without displaying secrets.
2. Connection tests never expose password, token or credential-bearing URL.
3. Editing a connection can show dependent runtimes and sync jobs.
4. Secret rotation verifies the new credential before revoking the old one.
5. Source-table drag creates a reviewed sync draft.
6. Canonical registry exports contain no credentials.
7. UI adapts to desktop and narrow viewport layouts.

See [DATABASE_REGISTRY.md](DATABASE_REGISTRY.md), [ARCHITECTURE.md](ARCHITECTURE.md), and [CANONICAL_MAPPING.md](CANONICAL_MAPPING.md).
