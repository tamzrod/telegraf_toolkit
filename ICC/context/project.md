# Project context

Status: Phase 0/1/2 scaffolds implemented; ADR-0002 metadata persistence implemented, secret store not implemented.

Telegraf Toolkit is a browser-based management/control plane for official Telegraf runtimes, visual configuration, a dual-database canonical mapper, reusable connection/credential registry, and independent cross-version InfluxDB historical synchronization.

**Authority:** `IDENTITY.md`. Detailed contracts remain in `docs/`. The Go backend and React SPA scaffolds exist; the Registry backend now persists profile, credential-reference and audit metadata in SQLite (ADR-0002, `docs/METADATA_STORE.md`). The secret store (ADR-0003) is accepted but not yet implemented.

**Boundaries:** No managed Telegraf fork, no floating versions, no direct browser privileged credentials/Docker access, no implicit deployment or historian repair, and no count-match equivalence claim. The metadata store holds credential references only, never secret values.

**Open decisions:** host-agent interface, production packaging and concrete adapters. Preserve these as unresolved. (Metadata store and secret-store custody were decided in ADR-0002/ADR-0003.)

**Source map:** `IDENTITY.md`, `docs/ARCHITECTURE.md`, `docs/METADATA_STORE.md`, `docs/ROADMAP.md`, `docs/adr/0001-official-telegraf-and-version-pinning.md`, `docs/adr/0002-metadata-store-selection.md`, `docs/adr/0003-secret-store-integration.md`.

**Audit:** refreshed by P2L-001 after implementing ADR-0002 persistence; re-check source files before further implementation.
