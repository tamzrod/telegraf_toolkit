# Development Roadmap

This is an initial plan, not an implementation status report.

## Phase 0 — Project foundation
- [x] Define intended scope and initial documentation
- [ ] Decide runtime, UI stack, API framework and packaging
- [ ] Establish data models and configuration versioning
- [ ] Build automated tests and local fixtures

## Phase 1 — Telegraf Manager
- [ ] Manage local/remote instances and status/log inspection
- [ ] Discover supported plugins and version-specific options
- [ ] Load, validate, stage, diff and apply TOML configuration
- [ ] Revision history and rollback

## Phase 2 — Visual Configurator and Canonical Registry
- [ ] Input -> processor -> output pipeline designer
- [ ] Searchable measurement/field/tag explorer
- [ ] Drag-and-drop mappings with rename and unit transforms
- [ ] Import/export canonical schemas and mapping profiles
- [ ] Preview, validation and deployment workflow

## Phase 3 — Historical Synchronization
- [ ] Connection profiles and version capability probing
- [ ] Per-measurement time-block inventory and count comparison
- [ ] Persistent jobs, block statuses, retries and resume
- [ ] Version-aware reads and destination writes
- [ ] Verification beyond counts and conflict policies
- [ ] Backfill throttling and observability

## Phase 4 — Production hardening
- [ ] Authentication, authorization, audit and secret management
- [ ] Backup/restore, performance and failure-injection testing
- [ ] Cross-version interoperability matrix
- [ ] Documentation, release packaging and upgrade strategy

## Scope boundaries
The Toolkit coordinates data pipelines and historian repair. It is not a replacement for Telegraf or InfluxDB. No frontend, sync engine, Telegraf integration or deployment is implemented by these documentation files.
