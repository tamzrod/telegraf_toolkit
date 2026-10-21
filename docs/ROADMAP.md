# Telegraf Toolkit — Development Roadmap

**Status:** Phase 0 engineering foundation and Phase 1 shell merged via PRs #2/#3; Phase 2 mock/interface Registry merged via PR #4. Real isolated InfluxDB 2 OSS / InfluxDB 3 Core testing and repeatability verification merged via PR #5 (TEST-001/TEST-002). Toolkit-to-InfluxDB connectivity remains NOT IMPLEMENTED; the operational Phase 2 Registry is the next delivery milestone.
**Authority:** [IDENTITY.md](../IDENTITY.md) defines product scope, architecture invariants and initial delivery order. This roadmap tracks delivery, not execution authorization.

A checked item reflects repository evidence, not intent. Proposed features and mock workflows are never presented as working connections, deployed runtimes or verified historian reconciliation.

## Delivery sequence

| Phase | Outcome | Status |
| --- | --- | --- |
| 0 | Repository and engineering foundation | Engineering scaffold merged; ADR-0002/0003 accepted for single-host Registry on 2026-10-21; ADR-0004/0005 still Proposed |
| 1 | Application shell and labeled mock workflows | Integrated into `main` via PR #3 |
| 2 | Database Registry and credential references | Mock/interface scope merged in PR #4; isolated InfluxDB 2/3 tests merged in PR #5; live Toolkit connectivity NOT IMPLEMENTED; operational work authorized only via CWAL |
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
- [x] Prepare decision-ready **Proposed** ADRs for the outstanding architectural choices (metadata store, secret-store integration, host-agent privilege boundary, initial packaging): [`docs/adr/0002`](adr/0002-metadata-store-selection.md)–[`0005`](adr/0005-initial-production-packaging.md). ADR-0002 and ADR-0003 were human-accepted on 2026-10-21 for single-host SQLite and backend-only envelope encryption with separate key custody/rotation. ADR-0004 and ADR-0005 remain Proposed.
- [x] Select and pin exact application/toolchain dependency versions; commit lockfiles and checksums as applicable. (Go 1.27.3; `ui/package-lock.json`; see [`docs/DEVELOPMENT.md`](DEVELOPMENT.md))
- [x] Establish CI checks (format, lint, build, tests, version-policy and secret safety). ([`.github/workflows/ci.yml`](../.github/workflows/ci.yml))
- [x] Establish local mock data, test fixtures and isolated development environments without real historian writes. ([`docs/DEVELOPER_SETUP.md`](DEVELOPER_SETUP.md))
- [x] Document developer setup, verification commands and task acceptance/evidence template. ([`docs/DEVELOPER_SETUP.md`](DEVELOPER_SETUP.md))

**Exit criteria:** foundational contracts are reviewable; reproducible builds/tests and CI exist for the created scaffold; missing architectural decisions needed for the next phase are resolved explicitly. No live deployment is implied.

> **Architecture update (2026-10-21):** ADR-0002 (SQLite single-host metadata) and ADR-0003 (backend-only envelope encryption with separate key custody and rotation) are Accepted. Host-agent privilege boundary ADR-0004 and production packaging ADR-0005 remain Proposed; do not imply they are resolved.

## Phase 1 — Application shell and mock workflows

- [x] Scaffold one React/TypeScript SPA with Vite, shared routing, layout, theme and responsive navigation.
- [x] Implement feature entry points for Overview, Runtimes, Pipelines, Databases, Credentials, Canonical Mapper, Synchronizer and Audit/Settings.
- [x] Add clearly labeled mock/empty/loading/error/permission states, including simulated authorization without claims of backend enforcement.
- [x] Establish typed API client boundary, query-state patterns and feature-local component/testing conventions.
- [x] Add smoke, accessibility and navigation tests.

**Exit criteria:** the SPA builds and runs locally; each module is navigable; all sample data is visibly labeled; no mock action performs deployment, credential mutation or historian writes.

> **Integrated status (PR #3):** P1-001–P1-005 were verified and merged into `main` at `938845f1b2e121e0e36929359574b9b7ee679522`. Responsive visual and screen-reader behavior still require real-browser checks. Mock permission states do not enforce authorization. ADR-0002–0005 remain Proposed.

## Phase 2 — Database Registry and credential references

- [x] Define interface-only Registry contracts (profile identity/scope, probed capabilities, credential references, sanitized errors, audit semantics) with a fixed/interface/open separation. ([`docs/contracts/registry.md`](contracts/registry.md))
- [x] Provide a human decision checklist for the outstanding metadata-store and secret-store decisions. ([`docs/decisions/phase-2-architecture-gate.md`](decisions/phase-2-architecture-gate.md))
- [x] Implement typed validation, in-memory mock adapters and an edition-keyed capability fixture matrix with tests. (`backend/internal/registrycontract/`)
- [x] Implement the credential-reference boundary, redaction, a deny-by-default endpoint threat model and rotation ordering with tests. (`backend/internal/registrycontract/security.go`)
- [x] Implement a typed browser Registry contract and a clearly labeled Databases mock workflow with explicit source/destination selection. (`ui/src/api/registry.ts`, `ui/src/features/databases/`)
- [x] Implement reusable source/destination InfluxDB connection profiles with explicit database/bucket/retention scope. **Persistent storage delivered (P2L-001):** profiles, scopes, credential *references* and audit events persist in the ADR-0002 SQLite metadata store with versioned migrations, restart durability, concurrency and backup/restore tests ([`docs/METADATA_STORE.md`](METADATA_STORE.md)). Server-authorized profile serving is P2L-003.
- [ ] Probe actual query/write/schema capabilities for InfluxDB 1.x, 2.x and 3.x by edition/deployment rather than assuming version-wide parity. **Partially delivered (P2L-004):** a read-only, SSRF-protected live prober identifies editions and capability sets from real responses, verified against the pinned InfluxDB 2 OSS v2.9.1 and 3 Core v3.12.1 test containers ([`docs/LIVE_PROBING.md`](LIVE_PROBING.md)). **InfluxDB 1.x is unverified** (no isolated 1.x instance); a full compatibility matrix is ADR-E, not yet proposed.
- [x] Establish protected server-side secret references and scoped read/write permissions; no plaintext secrets in browser responses, logs, exports or Git. **Delivered (P2L-002):** an envelope-encrypted secret store (AES-256-GCM, externally provisioned key file, server-only resolver, rotation with recovery) stores values as ciphertext envelopes in the metadata DB while keys stay outside it ([`docs/SECRET_STORE.md`](SECRET_STORE.md)). Server-authorized HTTP routes are P2L-003.
- [x] Implement backend-authorized test connection, sanitized errors, TLS policy and SSRF-aware endpoint validation. **Delivered (P2L-003/P2L-004):** sanitized errors, TLS policy, a server-authorized API with RBAC and denial tests ([`docs/REGISTRY_API.md`](REGISTRY_API.md)), a server-side credential resolver (P2L-002), and a read-only capability prober with DNS-resolved peer checks, redirect refusal and deny-by-default SSRF policy ([`docs/LIVE_PROBING.md`](LIVE_PROBING.md)).
- [x] Expose profile health, capabilities, usage/dependencies and credential rotation/revocation workflow with audit. **Delivered (P2L-001–P2L-005):** persistent profile/reference/audit store, envelope-encrypted credential store with rotation, authorized API, read-only live capability probing and live UI listing ([`docs/LIVE_UI.md`](LIVE_UI.md)). Usage/dependency inspection and browser write flows remain future work.

> **Integrated status (PRs #4/#5):** Phase 2 mock Registry was merged by PR #4. TEST-001/TEST-002 delivered a real InfluxDB 2 OSS v2.9.1 and InfluxDB 3 Core v3.12.1 isolated test harness by PR #5, with 16/16 direct DB tests passing twice on retained state. ADR-0002/0003 are Accepted; the operational live Registry was then implemented on `phase-2/live-registry-implementation` through authorized CWAL tasks P2L-001–P2L-005 (persistent metadata store, envelope-encrypted secret store, authorized API, read-only live probing, live UI listing). These are checkpointed on the unmerged work branch pending human PR review. No InfluxDB 1.x deployment was validated. Browser write flows and usage/dependency inspection remain future work.

**Exit criteria:** registered profiles are reusable across approved modules; permissions and secret isolation are tested; supported capabilities and unsupported cases are explicitly reported.

### Real database test infrastructure (merged PR #5)

- [x] Pin and deploy isolated InfluxDB 2 OSS v2.9.1 and InfluxDB 3 Core v3.12.1 containers with synthetic-only data and test credentials.
- [x] Verify direct authentication, supported read/write query APIs, persistence across restart and rejection of invalid tokens (16/16 DB checks in TEST-001).
- [x] Verify repeatable runs with retained state and safe credential-reuse failure modes (TEST-002).
- [ ] Connect the Toolkit Registry backend and UI to these live test instances; currently no authenticated backend Registry endpoint or real capability probe exists.

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

Accepted for the initial single-host Registry: SQLite metadata and envelope-encrypted secret storage (ADR-0002/0003). Open decisions: host-agent privilege interface, production packaging, and later adapters/multi-host support. Exact dependency versions must be chosen and pinned when scaffolding, not invented in documentation.
