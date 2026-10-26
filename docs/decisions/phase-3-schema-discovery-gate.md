# Phase 3 schema-discovery architecture gate

Status: **HUMAN APPROVAL RECORDED (2026-10-26); P3-001 verification/acceptance pending.**
Task: P3-001 (roadmap reconciliation and schema-discovery architecture gate).
Branch: `phase-3/canonical-mapper`.
Recorded: 2026-10-26.
Related: [`docs/contracts/canonical-mapper.md`](../contracts/canonical-mapper.md)
§3 and §7, [`docs/LIVE_PROBING.md`](../LIVE_PROBING.md),
[`docs/ROADMAP.md`](../ROADMAP.md).

> This record documents a decision. It does **not** accept one. No ADR is
> accepted, no new read path is implemented, and no capability claim is raised
> by this document. P3-002 may not start until the approval state in §9 changes.

## 1. Why this gate exists

The Phase 3 entry contract requires the mapper to browse actual source and
destination measurements, field keys and tag keys as distinct types. The Phase 2
Registry exposes only a **namespace-level** listing today: buckets on 2.x and
databases on 3.x. Contract §7 therefore left the measurement-level read path
open between a **new read-only API on the Registry** and a **read-only query
capability**, and stated that a query path changes the P2C-003 conclusion and
needs its own decision.

P3-001 was tasked to decide that question at an explicit gate. Measuring the
pinned isolated editions showed that the question as originally framed rests on
an incorrect premise, so the measurable facts had to be recorded before any
option could be chosen.

## 2. Inherited and not reopened

These are already decided and are not options in this gate:

- Both sides are **registered profiles**, selected explicitly; the endpoint,
  scope and credential are resolved server-side and never supplied by the
  browser.
- The read requires an **approved binding** for the profile's exact reference,
  connection, endpoint and canonical scope (P2L-007), and an authorized role.
- Pinned-peer, TLS and SSRF controls are inherited unchanged (P2L-006,
  P2C-007/P2C-008).
- The mapper writes **no** historian data, creates no schema, deploys no
  pipeline and starts no synchronization.
- No credential value, token or credential-bearing URL appears in a request, a
  response, a log, an export or browser storage.
- `unknown` is never rendered as supported or unsupported.
- ADR-0004 (host-agent privilege boundary) and ADR-0005 (packaging) remain
  **Proposed** and are untouched. No historian mutation or new privilege is
  authorized here.

## 3. Measured evidence (2026-10-26)

Environment: the pinned isolated harness in `testing/influxdb/` — InfluxDB 2 OSS
v2.9.1 and InfluxDB 3 Core v3.12.1, loopback-only, synthetic data, retained
synthetic state. Method: **GET requests only** against the running containers,
credentials read from the harness secret files (never argv), no writes, no
schema changes, no configuration changes; the stack was torn down afterwards.
Only status codes and response shapes were inspected; no credential value was
printed.

### 3.1 InfluxDB 2 OSS v2.9.1

| GET request | Status | Observed |
| --- | --- | --- |
| `/api/v2/buckets` | 200 | bucket listing |
| `/api/v2/dbrps?org=ttk-test-org` | 200 | DBRP mappings, including `virtual: true` entries |
| `/query?q=SHOW DATABASES` (1.x-compat) | 200 | `_monitoring`, `_tasks`, `ttk-test-bucket` |
| `/query?db=ttk-test-bucket&q=SHOW MEASUREMENTS` | 200 | `environment`, `sensor` |
| `/query?db=ttk-test-bucket&q=SHOW FIELD KEYS FROM "environment"` | 200 | `note` string, `temp_c` float, `up` integer, `used_percent` float |
| `/query?db=ttk-test-bucket&q=SHOW TAG KEYS FROM "environment"` | 200 | `host`, `region` |
| `/api/v2/query?q=SHOW MEASUREMENTS` | 405 | `allow: OPTIONS, POST` |
| `/api/v2/measurements` | 404 | `path not found` |

The 1.x-compatibility `/query` route requires the **DBRP database name**, not
the bucket name: `db=environment` produced an empty/error result, while
`db=ttk-test-bucket` (the name listed by `SHOW DATABASES`) returned the real
metadata.

### 3.2 InfluxDB 3 Core v3.12.1

| GET request | Status | Observed |
| --- | --- | --- |
| `/api/v3/configure/database?format=json` | 200 | `_internal`, `ttk_test_db` |
| `/api/v3/configure/table?db=ttk_test_db&format=json` | 404 | `Not found` |
| `/api/v3/query_influxql?db=ttk_test_db&q=SHOW MEASUREMENTS` | 200 | `environment`, `sensor` |
| `/api/v3/query_influxql?...&q=SHOW FIELD KEYS FROM "environment"` | 200 | `note` string, `temp_c` float, … |
| `/api/v3/query_influxql?...&q=SHOW TAG KEYS FROM "environment"` | 200 | `host`, `region` |
| `/api/v3/query_sql?...&q=SELECT table_name FROM information_schema.tables` | 200 | table list |
| `/api/v3/query/influxql?...` (slash form) | 404 | `Not found` |
| `/api/v3/query/sql?...` (slash form) | 404 | recorded by P2C-003 |

### 3.3 Findings that change the gate

**F1 — There is no query-free measurement-level listing on either pinned
edition.** `/api/v2/measurements` and `/api/v3/configure/table` do not exist;
the only way to obtain measurements, field keys and tag keys is to execute a
statement that returns them. A "metadata-only GET path" therefore still means
*executing metadata statements over GET*; it cannot mean "list without
querying".

**F2 — Both pinned editions do serve GET metadata statements.** InfluxDB 2 OSS
answers the 1.x-compatibility `GET /query`; InfluxDB 3 Core answers
`GET /api/v3/query_influxql` and `GET /api/v3/query_sql`. Measurement, field-key
and tag-key metadata (with types) was returned in every case above. A GET-only
metadata read is feasible **without introducing POST**.

**F3 — The published P2C-003 conclusion is contradicted for these routes.** The
probe evidence records "the verified editions serve a query endpoint only for
POST" and leaves every dialect `unknown`. That is true for `/api/v2/query`
(405, `allow: OPTIONS, POST`), but it is not true for the routes above. The
probe catalogue checks `/api/v3/query/sql` and `/api/v3/query/influxql`; the
serving endpoints use an **underscore** (`/api/v3/query_sql`,
`/api/v3/query_influxql`), so the catalogue saw 404 and classified the dialect
as `unknown`. Correcting the catalogue would change published capability
outcomes and is **outside P3-001's authority**; it is recorded here as a
prerequisite, not performed.

F3 is a documentation/evidence defect, not a new capability claim. The Registry
capability model still reports these dialects as `unknown` until the catalogue is
corrected under explicit authorization.

## 4. Options

| | Option A — bounded read-only metadata API | Option B — narrower metadata-only GET path |
| --- | --- | --- |
| Method | GET only (no POST introduced); fixed statement catalogue | GET only; fixed statement catalogue |
| Honest scope on the pinned editions | measurements + field keys + tag keys on 2 OSS and 3 Core | Same reachable scope, but only if it also executes metadata statements |
| A genuinely query-free variant | not available | namespace listings only (buckets/databases) — cannot meet the Phase 3 objective |
| New surface | a new authorized backend metadata-read route and per-edition statement adapters | same internal mechanism, narrower response contract |
| Changes P2C-003 conclusion | yes — requires correcting the catalogue/evidence (F3) | yes, for exactly the same reason |
| Meets "browse measurements/fields/tags" | yes | only if it executes statements; the query-free variant does not |

Under the measured evidence, Option B is not a query-free alternative to Option
A. The only meaningful choice is between:

- **A1 — bounded metadata-read API using fixed GET metadata statements**
  (recommended), versus
- **A2 — stay query-free** and leave measurement/field/tag discovery `unknown`,
  which makes P3-002/P3-003 deliver only namespace names.

A POST-based read path is **not required** by the evidence and is not
recommended. It would widen the method boundary for no capability gain on the
pinned editions.

## 5. Threat model for the recommended shape (A1)

Controls inherited unchanged: stored-profile endpoint only, DNS-resolved pinned
peer, TLS verification, redirect refusal, deny-by-default SSRF policy,
server-side credential resolution, approved-binding verification, RBAC,
sanitized errors, no secrets in responses or audit.

New controls this read path must establish:

- **Statement allowlist, not a passthrough.** The caller never supplies a
  statement. The server selects from a fixed catalogue: `SHOW MEASUREMENTS`,
  `SHOW FIELD KEYS FROM <measurement>`, `SHOW TAG KEYS FROM <measurement>`.
  Identifier interpolation must be validated against the discovered measurement
  names, never concatenated from raw input.
- **GET is not inherently non-mutating in InfluxQL.** `SELECT ... INTO` writes
  to a destination measurement. The allowlist — not the HTTP method — is the
  write barrier. No `SELECT ... INTO`, no DDL, no `DROP`, no `DELETE`, no
  arbitrary SQL.
- **Bounded read.** Per-request timeout, total request budget, row/measurement
  caps with an explicit `truncated` state, context cancellation, and no
  retry storm.
- **Metadata only.** No sample points, no field values, no series data, no
  counts that could be mistaken for historian verification.
- **Per-edition adapter.** Edition is taken from an observation, never a label.
  2.x gets the DBRP database name via the read-only `GET /api/v2/dbrps` and
  never creates or edits a mapping. 3.x uses the InfluxQL metadata statements.
- **Role decision required.** The narrow option reuses the existing read role
  (`profile.probe`); a separate `schema.read` role is a role-model change and
  would itself need approval. This record does not choose one.
- **Audit.** Record the observation (profile reference, edition, statement
  name, outcome, status) without response rows or secrets.
- **Error redaction.** A denial, timeout, malformed body and unknown edition
  stay distinct and sanitized; none becomes "unsupported".

## 6. Per-edition scope

| Edition | Status | Read mechanism (measured) | Notes |
| --- | --- | --- | --- |
| InfluxDB 2 OSS v2.9.1 | **observed** | `GET /api/v2/buckets`, `GET /api/v2/dbrps`, then `GET /query` with the DBRP database name | `GET /api/v2/query` is POST-only (405) and is not used |
| InfluxDB 3 Core v3.12.1 | **observed** | `GET /api/v3/configure/database`, then `GET /api/v3/query_influxql` metadata statements | `/api/v3/configure/table` is 404; underscore query paths only |
| InfluxDB 1.x | **unverified** | none | no isolated 1.x instance exists; the 1.x-compatibility endpoint on 2.x is not evidence of 1.x support |
| InfluxDB Cloud variants | **unverified** | none | not exercised |

## 7. Limits and unknown states

- An unknown or unidentified edition stops the read with an explicit unknown
  state; no statement is attempted.
- A missing approved binding is refused server-side (403); the mapper shows the
  refusal.
- A profile with no credential may read only what the deployment serves
  anonymously; today that is nothing on the pinned editions, so the result is
  `unknown`, not empty.
- A measurement whose field/tag keys cannot be read returns `unknown` with the
  observation, never an empty list presented as a schema.
- Results above the caps are returned with `truncated: true`; they are never
  silently cut.
- A scope or profile change invalidates the cached metadata and any draft.
- InfluxDB 1.x remains unverified. The 1.x-compatibility endpoint is a 2.x
  compatibility surface, not 1.x support.

## 8. Tests required before a future implementation is accepted

Negative/security tests: unauthorized role, missing binding, denied credential,
SSRF endpoint, redirect, TLS failure, timeout, oversized response, invalid
measurement identifier, statement-allowlist rejection (including a
`SELECT ... INTO` attempt), cancellation, and an unknown edition. Evidence
tests: recorded observation citing method/edition/statement; unknown stays
unknown; no secrets in errors or audit. Isolated 2 OSS and 3 Core end-to-end
metadata reads must pass before any UI claim. Race tests for concurrent reads
and for a scope change during a read.

## 9. Decision and approval state

**Recommendation:** Option **A1** — a bounded, authorized, read-only metadata
API that executes a fixed allowlist of GET metadata statements, with no POST
introduced, reusing the profile-only endpoint, approved binding and
pinned-peer/TLS/SSRF controls.

**Original approval state: PENDING — superseded by the human approval below.** The following require an explicit
human decision before P3-002 may start:

1. **Open a metadata query-execution read path at all** (even GET-only,
   SHOW-only). This is a new class of outbound read and is the substance of the
   gate.
2. **Correct the P2C-003 evidence and the probe catalogue's v3 query paths**
   (F3). This changes published capability outcomes and is separate authority.
3. **Choose the role**: reuse `profile.probe` or introduce a `schema.read`
   permission.
4. **Confirm the statement catalogue and bounds** in §5, including the
   `truncated` contract and the refusal to accept a caller-supplied statement.

Until then, P3-001 is not accepted and is not archived; P3-002–P3-006 remain
queued and must not start.

## 10. Explicitly not decided here

- Canonical-schema versioning, approval and import/export shape.
- Dependency inspection (which pipelines and jobs use a profile).
- Fine-grained per-connection/per-site authorization.
- Runtime secret delivery and the host-agent boundary (ADR-0004, Proposed).
- Production packaging (ADR-0005, Proposed).
- Any InfluxDB 1.x or Cloud support claim.
- Any write, deployment or synchronization capability.


## 11. Human approval and binding execution decision (2026-10-26)

**APPROVED by the human.** This section supersedes the pending approval language in §9, without altering historical measurements or claiming implementation.

1. **Option A1 approved:** introduce a backend-authorized, GET-only measurement metadata read API. Only fixed `SHOW MEASUREMENTS`, `SHOW FIELD KEYS FROM <validated measurement>`, and `SHOW TAG KEYS FROM <validated measurement>` statements may execute. No caller-supplied query, arbitrary SQL/InfluxQL passthrough, POST query route, `SELECT ... INTO`, DDL, deletion, writes, sample point retrieval, deployment or synchronization.
2. **P2C-003 correction approved:** correct mismatched InfluxDB 3 query endpoint catalogue paths and derived capability conclusions with new tests. Preserve original test evidence/history and append a clearly marked correction. Never silently rewrite historical observations or claim untested dialects/editions.
3. **Dedicated permission approved:** implement `schema.read`, distinct from `profile.probe`, with server-side deny-by-default authorization tests.
4. **Bounds and safety approved:** fixed statement allowlist, measurement identifier validation/escaping, per-request timeouts, total budgets, caps on rows/measurements/response size, explicit `truncated` state, cancellation, safe errors and redacted audit. Preserve server-resolved registered profile endpoint/scope, pinned-peer/TLS/SSRF and credential protection.

**Database authentication follows deployment configuration.** An authorized Toolkit user with `schema.read` may attempt an unauthenticated database read only when the selected profile intentionally has *no configured credential* and the database permits it. A configured credential must have an approved binding for exact profile/endpoint/scope. Invalid, denied, expired or revoked configured credentials must fail closed without anonymous fallback. Toolkit authorization is never optional. Database authentication refusal is not an empty schema.

**Evidence boundary:** the tested query-free endpoints returned 404 in pinned 2 OSS 2.9.1 / 3 Core 3.12.1; do not generalize this to all endpoints/editions/deployments.

**Execution:** The architecture gate is no longer awaiting approval, but P3-001 is **not accepted or archived**. Resume `operation cwal` at CURRENT=P3-001: reconcile documentation, correct approved catalogue/evidence in the appropriate scoped task, perform prescribed checks, and advance pointers only after verified acceptance, archive, commit/push and remote-SHA confirmation. Then proceed with the preauthorized P3-002–P3-006 queue sequentially. Stop only for genuinely new gates. No agent PR actions, no production historian writes, and no acceptance of Proposed ADR-0004/0005.
