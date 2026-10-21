# Telegraf Toolkit Context Index

ICC is a lightweight context cache. Start with `ICC/context/project.md`; follow its source references only as necessary. Do not construct deep nodes before implementation creates real boundaries.

## Initial semantic routes
- Project and architecture — `IDENTITY.md`, `docs/ARCHITECTURE.md`
- Database registry/security — `docs/DATABASE_REGISTRY.md`
- Persistent control-plane metadata store (ADR-0002) — `docs/METADATA_STORE.md`, `backend/internal/metadatastore/`
- Phase 2 registry interface contracts and decision gate — `docs/contracts/registry.md`, `docs/decisions/phase-2-architecture-gate.md`, `docs/decisions/phase-2-integration-handoff.md`
- Dual-database canonical mapping — `docs/CANONICAL_MAPPING.md`
- Telegraf runtimes and configuration — `docs/FRONTEND_ARCHITECTURE.md`, `docs/adr/0001-official-telegraf-and-version-pinning.md`
- Historical reconciliation — `docs/SYNCHRONIZATION.md`
- UI and delivery — `docs/UI_DESIGN.md`, `docs/ROADMAP.md`

Each future node must cite its supporting source paths and audited commit. Changed HEAD alone does not invalidate unrelated nodes.
