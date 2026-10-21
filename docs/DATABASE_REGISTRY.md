# Database Registry and Credential Management

Status: **Design intent, partially implemented.** The Database Registry backend is
implemented and merged into `main` (PR #6, 2026-10-21): persistent connection
profiles, credential *references*, an envelope-encrypted secret store, a
server-authorized API with role-based permissions, read-only live capability
probing and an administrator-only credential attachment binding. The browser
delivers live listing and read-only probing only. Profile reuse across the
pipeline editor, mapper and synchronizer; dependency inspection; browser
credential enrollment/revocation; runtime secret injection and production
packaging remain **planned** (ADR-0004 and ADR-0005 are **Proposed**).

## Purpose
Provide a single registry of database **connection profiles** shared by the Telegraf configuration editor, canonical Taglist mapper, and historical synchronization engine. Store connection metadata separately from credentials. Never embed API keys or passwords in project exports, Git repositories, UI responses, or ordinary TOML snapshots.

## Conceptual architecture

```text
Toolkit UI
   |-- Database Registry (connection profiles, tested capabilities)
   |-- Credential Manager (create/rotate/revoke access)
   |                  |
   +-- Management API + RBAC
          |-- Metadata store (profile IDs, endpoints, capabilities, secret refs)
          |-- Secret store (encrypted secrets / external secret backend)
          |-- Capability probe / scoped query service
          |-- Configuration compiler -> scoped runtime credential injection
          +-- Sync worker -> scoped source-read and target-write credentials
```

## Database connection profiles

Suggested attributes:
- Stable `connection_id`, display name, purpose/description, optional plant/site grouping.
- Database family and major version: InfluxDB 1.x, 2.x, 3.x; detected product edition where needed.
- Base URL(s), TLS settings including trust roots, explicit timeout and connectivity settings.
- Logical target scope: 1.x database/retention policy, 2.x organization/bucket, 3.x database and its available API conventions.
- Query dialect/capabilities: InfluxQL, Flux, SQL; do **not** infer dialect support solely from major version.
- Write protocol/capabilities, schema discovery support, allowed operations and observed limits.
- Credential reference IDs **only**; health state, last tested time, connection errors sanitized for display.
- No need to store plaintext credentials in the registry.

A database connection may have separate credentials for *read*, *write* and *administration*. No assumption that every InfluxDB deployment uses API tokens: InfluxDB 1.x often uses username/password or deployment-specific authentication; 2.x uses tokens, while 3.x auth depends on product and deployment.

## Credential lifecycle

1. **Add**: enter credentials over authenticated TLS; encrypt at rest using an encryption key provided independently of the metadata database (or use an external secret manager).
2. **Scope**: assign minimum permissions and target database/bucket/measurement access supported by the destination.
3. **Reference**: attach credential IDs to profiles and authorized jobs; UI only receives names, scopes, fingerprints and last-rotated metadata.
4. **Use**: resolve credentials only server-side or in an authorized host agent/worker. Pass secrets to a Telegraf runtime through a protected delivery mechanism (restricted files or securely managed runtime secrets); avoid command-line flags, public environment dumps, logs, and world-readable Compose/TOML files.
5. **Rotate**: add new credential, validate, atomically update affected assignments/runtime configurations, confirm operation and then revoke old credentials. Handle in-flight jobs and rollbacks.
6. **Revoke/Delete**: block unauthorized future use and display impacted runtime pipelines and jobs before removal.
7. **Audit**: record creator, actor, action, credential *reference* and outcome without recording the secret value.

Treat an API key as sensitive **even if read-only**. Never display its full value again after creation unless the upstream system explicitly requires one-time retrieval; in normal Toolkit operation secrets are write-only from the UI.

## Access and safeguards
- Roles: viewer (list/test status), operator (use permitted connections), configurator (create/change assignments), administrator (manage credentials). Refine with per-connection and per-site authorization.
- Enforce authorization on backend operations, not just UI visibility.
- Avoid SSRF: validate endpoints; restrict reachable network targets as appropriate and block unintended access to metadata/internal services.
- Use TLS verification by default; insecure verification requires an explicit auditable exception.
- Sanitize test-connection errors and never include secrets in stack traces or metric labels.
- Protect database backups: encryption keys must not live in the same unprotected backup as encrypted values.
- Credential export is **metadata-only** by default. Full secret portability requires a separate explicitly authorized encrypted backup/restore workflow.
- Redact secrets during TOML preview, diff, historical revision view, and diagnostics.

## Integration with existing modules

**Telegraf configuration editor:** choose a registered destination by name; the compiler selects compatible output plugin configuration and injects only the authorized write secret into the selected isolated runtime. Generated deployable config containing secrets must be protected, never checked into Git.

**Taglist mapper:** enumerate measurements/fields/tags via a scoped read credential; cache discovery results with TTL and permissions. Do not allow a viewer to use an unrestricted administrative credential.

**Database synchronizer:** source and target profiles are distinct; prefer read-only for source and restricted write on target. Use separate verified query/write capabilities and track which credential references the job uses without serializing their values.

## UI screens

- **Database Registry**: connections, type/version, endpoint, site, health, capability badges, last test.
- **Connection Editor**: protocol, dialect, logical database, authentication method and Test Connection.
- **Credentials**: secret references, scope, usage, rotate/revoke and audit history.
- **Dependencies**: list pipelines and sync jobs that rely on a selected profile or credential.

## Acceptance criteria

- Same connection can be reused for multiple authorized pipelines and sync jobs.
- Expired token reports an actionable sanitized error without leaking its value.
- Rotating one secret does not interrupt unrelated runtimes.
- Exported profiles and canonical registry never contain credentials.
- Failed TLS validation is visible and not silently bypassed.
- Each version uses its supported query/write interface, verified at registration time.
- Deleting an in-use credential requires dependency review.

## Implementation status against this design

| Area | State | Source |
| --- | --- | --- |
| Connection profile persistence (SQLite, versioned migrations) | Implemented (P2L-001) | [`docs/METADATA_STORE.md`](METADATA_STORE.md) |
| Envelope-encrypted credential store, server-side resolve and rotation | Implemented (P2L-002) | [`docs/SECRET_STORE.md`](SECRET_STORE.md) |
| Server-authorized API, RBAC, sanitized errors, audit | Implemented (P2L-003, P2L-007) | [`docs/REGISTRY_API.md`](REGISTRY_API.md) |
| Transactional enrollment: profile, binding and audit applied all-or-nothing; no orphaned binding | Implemented (P2C-002) | [`docs/METADATA_STORE.md`](METADATA_STORE.md) |
| Read-only live capability probing with pinned-peer/SSRF controls | Implemented (P2L-004, P2L-006) | [`docs/LIVE_PROBING.md`](LIVE_PROBING.md) |
| Browser live listing and read-only probing | Implemented (P2L-005); writes gated | [`docs/LIVE_UI.md`](LIVE_UI.md) |
| Capability evidence per capability (not per edition) | Not implemented — capabilities are projected from the detected edition | [`docs/LIVE_PROBING.md`](LIVE_PROBING.md) |
| InfluxDB 1.x support | Unverified — no isolated 1.x instance | [`docs/LIVE_PROBING.md`](LIVE_PROBING.md) |
| Profile reuse by pipeline editor / mapper / synchronizer | Not implemented — those modules are Phase 4–6 | [`docs/ROADMAP.md`](ROADMAP.md) |
| Dependency inspection, browser credential lifecycle, runtime secret injection | Not implemented | [`docs/ROADMAP.md`](ROADMAP.md) |
