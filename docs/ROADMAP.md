# Telegraf Toolkit — Development Roadmap

**Status:** Planning and architecture complete enough to begin foundation implementation; no operational application implemented yet.  
**Authority:** [IDENTITY.md](../IDENTITY.md) defines product scope, architecture invariants and initial delivery order. This roadmap tracks delivery, not execution authorization.

A checked item reflects repository evidence, not intent. Proposed features and mock workflows are never presented as working connections, deployed runtimes or verified historian reconciliation.

## Delivery sequence

| Phase | Outcome | Status |
| --- | --- | --- |
| 0 | Repository and engineering foundation | In progress |
| 1 | Application shell and labeled mock workflows | Not started |
| 2 | Database Registry and credential references | Not started |
| 3 | Dual-database Canonical Registry / Taglist Mapper | Not started |
| 4 | Visual Pipeline Designer and TOML validation | Not started |
| 5 | Isolated Telegraf Runtime Manager | Not started |
| 6 | Independent historian synchronizer and block explorer | Not started |
| 7 | Production security, interoperability and packaging validation | Not started |

Phases identify primary delivery order, not permission to postpone baseline authentication, input validation, secret protection or tests until Phase 7. Work on an enabling shared contract may occur earlier when required by the currently authorized task.

## Phase 0 — Repository and engineering foundation

### Established
- [x] Define project identity, product scope, core architecture and initial delivery order.
- [x] Accept official upstream Telegraf baseline **1.40.1** and immutable dependency policy ([ADR-0001](adr/0001-official-telegraf-and-version-pinning.md)).
- [x] Select React + TypeScript, Vite, Tailwind CSS/shadcn/ui, React Flow, dnd-kit, Monaco and TanStack Query as frontend direction.
- [x] Select Go API/background workers as backend direction.
- [x] Establish OpenHands agent router, Operation CWAL, BLACK SHEEP WALL, lightweight ICC, Active Work ledger and handoff.
- [x] Document initialization-only direct-`main` exception and normal branch/pull-request development policy.

### Remaining
- [x] Define versioned API/domain contracts for connections, credential references, capability probes, canonical schemas, mapping profiles, runtime/revisions, sync jobs/blocks and audit events. ([`docs/contracts/README.md`](contracts/README.md))
- [x] Prepare decision-ready **Proposed** ADRs for the outstanding architectural choices (metadata store, secret-store integration, host-agent privilege boundary, initial packaging): [`docs/adr/0002`](adr/0002-metadata-store-selection.md)–[`0005`](adr/0005-initial-production-packaging.md). These are authored proposals only: **none is an accepted architecture decision.** Acceptance is outstanding and human-owned, so the underlying choices remain unresolved.
- [x] Select and pin exact application/toolchain dependency versions; commit lockfiles and checksums as applicable. (Go 1.27.3; `ui/package-lock.json`; see [`docs/DEVELOPMENT.md`](DEVELOPMENT.md))
- [x] Establish CI checks (format, lint, build, tests, version-policy and secret safety). ([`.github/workflows/ci.yml`](../.github/workflows/ci.yml))
- [x] Establish local mock data, test fixtures and isolated development environments without real historian writes. ([`docs/DEVELOPER_SETUP.md`](DEVELOPER_SETUP.md))
- [x] Document developer setup, verification commands and task acceptance/evidence template. ([`docs/DEVELOPER_SETUP.md`](DEVELOPER_SETUP.md))

**Exit criteria:** foundational contracts are reviewable; reproducible builds/tests and CI exist for the created scaffold; missing architectural decisions needed for the next phase are resolved explicitly. No live deployment is implied.

> **Status note (P0-005):** The architectural ADRs are authored as **Proposed** only. Metadata store, secret-store integration, host-agent privilege boundary and production packaging are **not yet accepted decisions**, so Phase 0 exit criteria are not fully satisfied for architecture until a human accepts them. Do not treat these proposals as resolved architecture.

## Phase 1 — Application shell and mock workflows

- [ ] Scaffold one React/TypeScript SPA with Vite, shared routing, layout, theme and responsive navigation.
- [ ] Implement feature entry points for Overview, Runtimes, Pipelines, Databases, Credentials, Canonical Mapper, Synchronizer and Audit/Settings.
- [ ] Add clearly labeled mock/empty/loading/error/permission states, including simulated authorization without claims of backend enforcement.
- [ ] Establish typed API client boundary, query-state patterns and feature-local component/testing conventions.
- [ ] Add smoke, accessibility and navigation tests.

**Exit criteria:** the SPA builds and runs locally; each module is navigable; all sample data is visibly labeled; no mock action performs deployment, credential mutation or historian writes.

## Phase 2 — Database Registry and credential references

- [ ] Implement reusable source/destination InfluxDB connection profiles with explicit database/bucket/retention scope.
- [ ] Probe actual query/write/schema capabilities for InfluxDB 1.x, 2.x and 3.x by edition/deployment rather than assuming version-wide parity.
- [ ] Establish protected server-side secret references and scoped read/write permissions; no plaintext secrets in browser responses, logs, exports or Git.
- [ ] Implement backend-authorized test connection, sanitized errors, TLS policy and SSRF-aware endpoint validation.
- [ ] Expose profile health, capabilities, usage/dependencies and credential rotation/revocation workflow with audit.

**Exit criteria:** registered profiles are reusable across approved modules; permissions and secret isolation are tested; supported capabilities and unsupported cases are explicitly reported.

## Phase 3 — Dual-database Canonical Registry / Taglist Mapper

- [ ] Require explicit **Source Database** and **Destination Database** selection, including scope, before schema browsing.
- [ ] Browse actual source and destination measurements, field keys and tag keys as distinct types, with cache invalidation on connection/scope changes.
- [ ] Create editable drag-and-drop mapping drafts for measurement/field/tag names, tag values, filters and explicit unit transforms.
- [ ] Validate types, destination collisions and canonical contracts with sample-point previews.
- [ ] Version and import/export canonical schemas separately from source-specific mapping profiles; exclude secrets.
- [ ] Save profiles without deploying pipelines or starting historical synchronization.

**Exit criteria:** source/destination switching, drafts, validation, save/rollback and portable artifacts pass tests. Shared mapping semantics are specified for both live data and backfill.

## Phase 4 — Visual Pipeline Designer and TOML validation

- [ ] Build input → processor → output graph editing, keyboard alternatives and plugin configuration panels.
- [ ] Discover and validate version-specific official Telegraf plugin settings, starting with 1.40.1.
- [ ] Generate/review native TOML, diffs and revisions without leaking secrets.
- [ ] Support draft, validation and approved apply/rollback semantics without silently changing a live runtime.
- [ ] Verify mapping compilation behavior against Phase 3 contracts.

**Exit criteria:** representative plugin configurations produce tested TOML; invalid configurations are rejected; saving remains distinct from deployment.

## Phase 5 — Isolated Telegraf Runtime Manager

- [ ] Implement restricted backend/host-agent runtime control with explicit permissions and audit.
- [ ] Manage separately isolated official Telegraf runtimes through single-host Docker Compose; one runtime/container by default, grouping plugins only within a shared failure domain.
- [ ] Monitor status, logs, restarts, health and resource use; expose configuration revisions and selective restart/rollback.
- [ ] Enforce explicit version pins (production verified image digests), resource budgets and failure isolation.
- [ ] Validate authorized deployment previews and ensure unrelated collectors remain uninterrupted.

**Exit criteria:** independent runtime deploy/restart/rollback and failure isolation are verified in a non-production environment.

## Phase 6 — Historian synchronizer and time-block explorer

- [ ] Implement independent sync controller, workers, persistent job/block index and source/destination selection.
- [ ] Compare per-measurement counts in UTC half-open time blocks; label count matches distinctly from verified equivalence.
- [ ] Support capability-aware InfluxDB 1.x/2.x/3.x queries/writes, selectable fields and shared canonical mappings.
- [ ] Add explicitly reviewed selective backfill, idempotent retries, checkpoints, resume and bounded throughput.
- [ ] Define and test collision/conflict policies, timestamp/tag/field-type preservation and deeper point-level verification.
- [ ] Expose block status, errors, repair evidence and unresolved conflicts without disrupting live collection.

**Exit criteria:** missing, changed, extra, partially copied and actively written time-block scenarios pass isolated integration tests; no count-only result is mislabeled verified.

## Phase 7 — Production validation, security and release

- [ ] Complete authorization/RBAC, audit, credentials/rotation, TLS and secret-handling threat tests across all modules.
- [ ] Test InfluxDB version/edition/query/write compatibility against an explicit matrix.
- [ ] Validate workload capacity, failure injection, backup/restore and recovery for runtime and sync paths.
- [ ] Verify accessibility, operational observability, least privilege and deployment isolation.
- [ ] Produce pinned/digest-verified packaging, install/upgrade/rollback documentation and release checks.

**Exit criteria:** supported environments and limits are documented with reproducible evidence; security and operational gates pass before production release.

## Development and CWAL execution policy

- This roadmap is **planning only**. It does not authorize OpenHands implementation.
- Humans define and promote bounded tasks to `workflow/active_work/`; Operation CWAL executes only the current authorized task, checks relevant ICC/source and records actual verification.
- During **repository initialization**, explicitly approved governance/setup/documentation changes may use direct `main` commits. **Application scaffolding and all regular software development** use dedicated branches, checks, human-reviewed PRs and confirmed merges, regardless of whether initialization remains open.
- Completion is durable only after the relevant checkpoint is confirmed on `origin/main`; unmerged branch archive moves are provisional.
- Accepted architecture changes require explicit review/ADR; never silently overrule `IDENTITY.md`.

## Scope boundaries and outstanding decisions

Telegraf Toolkit manages official Telegraf runtimes, mapping and historian reconciliation; it does not replace Telegraf or InfluxDB. Live collection and historical repair have independent lifecycles; schema/mapping save actions never imply deployment or repair. Actual support is determined by probed capabilities, not optimistic version labels.

Open decisions: metadata store, host-agent interface, production packaging and concrete adapters. Exact dependency versions must be chosen and pinned when scaffolding, not invented in documentation.
