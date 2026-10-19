# Database Registry and Credential Management

Status: **Architecture proposal — not implemented**.

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
