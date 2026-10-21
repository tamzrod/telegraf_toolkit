# Project context

Status: Phase 0/1/2 scaffolds implemented; the operational Phase 2 Registry backend (P2L-001–P2L-007) merged into `main` via PR #6 on 2026-10-21.

Telegraf Toolkit is a browser-based management/control plane for official Telegraf runtimes, visual configuration, a dual-database canonical mapper, reusable connection/credential registry, and independent cross-version InfluxDB historical synchronization.

**Authority:** `IDENTITY.md`. Detailed contracts remain in `docs/`. The Go backend and React SPA scaffolds exist; the Registry backend persists profile, credential-reference, audit and ciphertext-envelope metadata in SQLite (ADR-0002, `docs/METADATA_STORE.md`), stores credential values as authenticated envelopes with keys held in a separate file (ADR-0003, `docs/SECRET_STORE.md`), serves an authorized HTTP surface with role-based permissions and an administrator-only credential attachment binding (`docs/REGISTRY_API.md`), performs read-only live capability probing with pinned-peer/SSRF controls (`docs/LIVE_PROBING.md`) and is listed by the Databases UI from the live API (`docs/LIVE_UI.md`). Authentication is static file-provisioned bearer tokens; capability outcomes are projected from the detected edition rather than observed per capability; InfluxDB 1.x is unverified.

**Boundaries:** No managed Telegraf fork, no floating versions, no direct browser privileged credentials/Docker access, no implicit deployment or historian repair, and no count-match equivalence claim. The metadata store holds credential references and ciphertext; plaintext values and keys never enter it. The browser ships live listing and read-only probing only; browser write flows and credential lifecycle remain unimplemented.

**Open decisions:** host-agent interface, production packaging and concrete adapters. Preserve these as unresolved. (Metadata store and secret-store custody were decided in ADR-0002/ADR-0003.)

**Source map:** `IDENTITY.md`, `docs/ARCHITECTURE.md`, `docs/METADATA_STORE.md`, `docs/SECRET_STORE.md`, `docs/REGISTRY_API.md`, `docs/LIVE_PROBING.md`, `docs/LIVE_UI.md`, `docs/ROADMAP.md`, `docs/adr/0001-official-telegraf-and-version-pinning.md`, `docs/adr/0002-metadata-store-selection.md`, `docs/adr/0003-secret-store-integration.md`.

**Audit:** refreshed by P2L-001 (ADR-0002 persistence) and P2L-002 (ADR-0003 secret store); refreshed again by P2C-001 after PR #6 (P2L-003–P2L-007 merged); re-check source files before further implementation.
