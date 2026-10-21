# UI/UX Specification — Telegraf Toolkit

Status: **Design concept, partially implemented.** The application shell and the
live Databases view (profile listing and read-only capability probing) ship;
browser write flows, credential entry/revocation and the runtime, pipeline,
mapper and synchronizer screens remain design only. Frontend technology and
build plan: [Frontend Architecture](FRONTEND_ARCHITECTURE.md).

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

## Taglist / Canonical Mapper screen

**First select two registered database connection profiles:** Source Database and Destination Database. The page must not assume a fixed source or target. Resolve each profile's version, query capabilities and credentials through the registry, then browse the selected database scopes.

1. Select **Source Database** and its database/bucket scope.
2. Select **Destination Database** and its database/bucket scope.
3. Browse the source measurements, then select a measurement to load its fields and tag keys (distinct types).
4. Browse or create a destination measurement and inspect its existing field/tag schema.
5. Drag selected source fields or tags onto the destination measurement. The UI proposes mappings; allow destination field/key rename to comply with an imported canonical schema, unit conversion, and tag transformation.
6. Preview the source-to-destination mapping, validate collisions and field types, then **Save Mapping Profile**. Deployment to live Telegraf or historical synchronization is a separate explicit action.

### Detailed mapper behavior

- Both Source Database and Destination Database selectors are required before browsing or mapping. Each references a registered connection ID, selected logical scope (bucket/database), and its permitted credential reference.
- Show two live-schema explorers, not static fields: source measurement -> field keys and tag keys on the left; destination measurement -> existing field keys and tag keys on the right. Any inferred types/units must be labeled as inferred until verified.
- Selecting a source field prepares a mapping; users can rename the destination field to an approved canonical name, set explicit unit conversion, and add it to the draft. Multiple fields can map into one target measurement when identity and schema rules permit.
- Show the mapping list with original field name, destination field name, transformation, and remove/edit controls. Offer validation against an imported canonical registry.
- Allow saving the mapping profile independently of deploying it. **Save Mapping** must not start live collection or historical synchronization.
- A deliberate follow-up action may apply the mapping to a selected Telegraf runtime or create a reviewed synchronization job.
- Show empty/loading/failed-connection states and prevent saving invalid or conflicting mappings.

Changing source or destination profiles invalidates stale schema views and prompts to discard or revalidate pending mappings. Do not expose credentials in browser responses. Support same-version and cross-version mapping subject to tested capabilities.

Layout: **Source DB selector + source measurement/field/tag explorer** | **mapping arrows/editor** | **Destination DB selector + destination measurement/field explorer**. Canonical registry import/export is accessible from the mapping editor, not a replacement for choosing databases.

## Credentials screen
Show alias, authentication type, associated profiles, read/write/admin scope, age, expiry, and runtime/job dependencies. Actions: Add, Rotate, Revoke, Inspect dependencies. Credential input is masked and write-only. No plaintext reveal in UI, logs, or exported documents. Preserve auditable changes.

## Visual workflows
- Drag a measurement from a source connection onto a destination to create a sync **draft**, not an immediate write.
- After selecting source and destination databases, drag source fields/tags onto a destination measurement; edit canonical name and transformations before save.
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
