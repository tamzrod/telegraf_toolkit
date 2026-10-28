# Phase 3 Canonical Mapper — entry contract

Status: **Entry contract — the measurement-level read path is implemented (P3-002, 2026-10-26); the read-only schema browser (P3-003) and the local, in-memory mapping draft with validation (P3-004, 2026-10-28) are implemented for the pinned InfluxDB 2 OSS and 3 Core editions. Draft persistence, versioning and export are not implemented (P3-005) and InfluxDB 1.x remains unverified.**
Task: P2C-006 (Phase 2 integration acceptance and Phase 3 handoff); reconciled by P3-001; read path delivered by P3-002; schema browser delivered by P3-003.
Depends on: the Phase 2 Registry (P2L-001–P2L-007, P2C-001–P2C-008).

This document is the bounded contract the Phase 3 Canonical Mapper starts from.
It states **what the mapper may rely on today**, what it must not assume, and
where the truth is still unknown. It implements nothing, reads no data and
writes nothing.

The design intent stays in [`docs/CANONICAL_MAPPING.md`](../CANONICAL_MAPPING.md);
the contract inventory stays in [`docs/contracts/README.md`](README.md).

## 1. Source and destination identity

Both sides are **registered Registry profiles**, chosen explicitly and never
derived from a default or a hard-coded host:

| Property | Source of truth | Rule |
| --- | --- | --- |
| Connection identity | The profile's `connection_id` | A mapping draft records the **reference**, never a host, a port or a URL |
| Endpoint | The profile's stored `endpoint` | Resolved server-side. A draft may not carry or override it |
| Credential | The profile's approved binding | The browser never names a credential; the backend resolves the reference server-side |
| Logical scope | The profile's stored scope (database + retention policy, organization + bucket, or database) | The scope is part of the approved identity, so a scope change invalidates the draft |

Two rules carry over unchanged from the Registry contract:

- The source and the destination are **distinct** profiles, and the selection
  must be explicit on both sides.
- Selecting a different profile or a different scope **invalidates** any cached
  schema and prompts a revalidation of the draft.

## 2. Scope of the first mapper increment

The first, read-only increment (P3-003) implements:

- explicit **source** and **distinct destination** selection from the active
  Registry profile list, with the stored canonical scope visible on each side;
- reading the **metadata** of both sides (measurements, field keys, tag keys)
  through the authorized GET-only path;
- showing the source and destination metadata side by side as separate typed
  groups, with loading, empty, permission, unknown, unsupported, not-found,
  truncated and stale states;
- cancelling and dropping cached answers when the profile, scope, token or mode
  changes.

It does **not** yet implement a mapping draft. P3-004 is expected to add the
local draft and validation; P3-005 the versioned canonical schema and mapping
profile persistence.

The mapper must not:

- write to a historian, create a measurement or alter a schema;
- deploy a pipeline or start a synchronization;
- export a mapping profile that contains a credential or a credential-bearing
  URL.

## 3. What the Registry can prove today

The mapper inherits the evidence the Phase 2 acceptance produced, and must
not claim more:

| Capability | State | Evidence |
| --- | --- | --- |
| A registered profile with a stored endpoint, scope and credential **reference** | available | P2L-001/P2C-002, exercised against the real containers |
| A credential that is **approved** for a profile, bound to its exact reference, connection ID, endpoint and canonical scope | available | P2L-007, P2C-002 |
| The **edition** of a deployment, identified from a read-only response | available | P2C-003 (real InfluxDB 2 OSS and 3 Core) |
| A **namespace-level listing** (buckets on 2.x, databases on 3.x) | available | P2C-003 (real containers) |
| A **measurement-level** schema listing (measurements, field keys, tag keys) | **available** | P3-002: a backend-authorized GET-only read of fixed `SHOW` statements under `schema.read`, verified end to end against the pinned InfluxDB 2 OSS v2.9.1 and 3 Core v3.12.1 containers — see §8 |
| A browser **read-only schema browser** for two explicit profiles | **available (read-only)** | P3-003: typed groups, stale/cache cancellation, live/mock labeling and no mock fallback; authenticated real-backend smoke in §9 |
| A **query dialect** (InfluxQL, Flux, SQL) proven to work | **partially observed; not proven for 2.x or 1.x** | P2C-003 correction (P3-002): on the pinned 3 Core, `influxql` and `sql` are observed as `supported` through the corrected GET routes; on 2 OSS both stay `unknown` because its API query route refuses GET. No dialect is claimed for 1.x |
| A **write** protocol | **unknown** | A read-only probe never writes; no authorized synthetic write check exists |
| InfluxDB 1.x | **unverified** | No isolated 1.x instance exists |

The mapper therefore starts with an explicit **unknown** state for anything the
Registry has not observed. "Unknown" must never be rendered as "supported" or
"unsupported": it is the honest answer, and the UI vocabulary already carries
it (P2C-003). A dialect observation is edition- and deployment-specific: the
mapper must carry the observation that supports it, not infer one from a version
label, and it must not treat a 2 OSS profile as though it answered the 3 Core
GET routes.

## 4. The unknown states the mapper must model

| Unknown | What it means for the mapper |
| --- | --- |
| The deployment's edition is unknown | The mapper cannot assume a dialect or a schema endpoint; it must ask, or stop |
| The query dialect is unknown | No preview of sample points can be promised; a draft stays a draft |
| The schema listing is unknown | The measurement, field and tag lists are empty **with an explicit reason**, not silently empty |
| The credential is not approved for the profile | The mapper may not read anything from that profile; the selection is refused server-side (403) |
| The profile has no credential | The mapper may read only what the deployment serves anonymously, and must say so |

Every one of these states is a **first-class answer**, not an error to be
hidden: a draft that cannot be validated says why. P3-003 surfaces them in the
schema browser as distinct loading, empty, permission, unknown, unsupported,
not-found, truncated and stale states; only a positively supported empty answer
is shown as an observed empty schema.

## 5. Boundaries the mapper inherits

- **Server-side authorization.** The browser is never a security boundary. A
  read that the principal may not perform is refused server-side, and the
  mapper shows the refusal.
- **Secret isolation.** No credential value, token or credential-bearing URL
  appears in a draft, a URL, a log, an export or browser storage. The mapper
  names a **reference**, like every other Registry surface.
- **Pinned peers, TLS and SSRF.** The destination of every request is the
  stored profile's endpoint, resolved server-side, with the pinned-peer, TLS
  and SSRF controls the probe route uses. The mapper never accepts a host from
  the browser.
- **No implicit writes.** Browsing the source and the destination changes
  nothing. A mapping draft is not a deployment.
- **Explicit sides.** The source and the destination are distinct, and every
  cached artefact is invalidated when the selection or the scope changes.

## 6. Entry checklist for the first Phase 3 task

A Phase 3 task may start when it states:

1. Which **read-only** API surface it adds (or reuses) for the source and the
   destination metadata, and the exact permission it requires. **Decided by the
   P3-001 gate** (§8): GET-only fixed `SHOW` metadata statements under a
   dedicated `schema.read` permission.
2. How the **unknown** states of section 4 are surfaced in the UI, and that
   none of them is rendered as support.
3. That the mapper writes **no** historian data and creates no schema.
4. That the mapping draft holds **references only**, and that the export
   contract excludes secrets.
5. The **evidence** it will produce: which edition, which commands, and what
   stays unknown.

## 7. Explicitly not decided here

- The read path for a measurement-level schema listing is **decided and
  approved** (Option A1, 2026-10-26): a bounded backend metadata read executing
  only fixed GET `SHOW MEASUREMENTS` / `SHOW FIELD KEYS` / `SHOW TAG KEYS`,
  behind `schema.read`. Implemented by P3-002; see §8.
- How a canonical schema is versioned, shared and approved
  ([`docs/CANONICAL_MAPPING.md`](../CANONICAL_MAPPING.md) leaves this open).
- The dependency-inspection view (which pipelines and jobs use a profile).
- Fine-grained, per-connection or per-site authorization; the Registry is
  role-based only.
- Runtime secret delivery and the host-agent boundary (ADR-0004, **Proposed**).

## 8. Decision (approved; implemented by P3-002)

The measurement-level read path is **decided, approved and implemented** by
P3-002 (2026-10-26).
The human approved Option A1 on 2026-10-26
([`docs/decisions/phase-3-schema-discovery-gate.md`](../decisions/phase-3-schema-discovery-gate.md)
§11): a backend-authorized, GET-only metadata read that executes only fixed
`SHOW MEASUREMENTS`, `SHOW FIELD KEYS FROM <validated measurement>` and
`SHOW TAG KEYS FROM <validated measurement>` statements, under a dedicated
`schema.read` permission, with bounded timeouts, caps, truncation, cancellation,
redacted audit and the inherited pinned-peer/TLS/SSRF and credential controls.
Caller-supplied queries, POST query routes, `SELECT ... INTO`, DDL and all
writes are prohibited. A profile with no configured credential may be read
without database credentials only where the deployment permits it; a configured
credential needs an approved binding and fails closed without anonymous
fallback.

The P2C-003 v3 catalogue path/evidence correction was approved with the decision
and implemented by P3-002. The historical observations are preserved verbatim
and the correction is appended in [`docs/LIVE_PROBING.md`](../LIVE_PROBING.md)
(P2C-003 correction and authorized schema-discovery sections).

**Implementation (P3-002, 2026-10-26).**

- Routes: `GET /api/v1/registry/schema/measurements`, `…/schema/fields` and
  `…/schema/tags`, all under `schema.read` (operator, configurator and
  administrator; viewer denied). The permission is distinct from
  `profile.probe`.
- Inputs: `connection_id`, plus `measurement` for fields/tags. The endpoint,
  scope and credential come from the stored, approved profile identity; a
  caller-supplied URL, credential, query, deadline or limit is refused (400),
  as is an unknown or repeated parameter.
- Statements: only `SHOW MEASUREMENTS`, `SHOW FIELD KEYS FROM <measurement>`
  and `SHOW TAG KEYS FROM <measurement>` over GET. No caller statement, POST
  query route, `SELECT … INTO`, DDL, deletion or write exists. A field/tag read
  first requires exact membership in the observed measurement list, then
  queries with the escaped identifier; a non-member is 404
  `measurement-not-found`.
- Adapter: the edition is observed from `/health`, never taken from the profile
  label. 2.x resolves the bucket and DBRP database with read-only
  `GET /api/v2/buckets` and `GET /api/v2/dbrps` (never creating a mapping), then
  queries `GET /query`; 3.x queries `GET /api/v3/query_influxql`.
- Bounds: 15 s total deadline, 5 s per request, 8-request budget, 1 000-row
  cap, 1 MiB response-byte cap, context cancellation and an explicit
  `truncated` flag.
- Outcomes: `supported` only for a 200 with the expected shape. 401/403, 404,
  405, timeout, 5xx, transport failure or a malformed body stays `unknown`; a
  404 on a query route is not `unsupported`. A database authentication refusal
  is not an empty schema.
- Identity: every response repeats `connection_id`, the canonical `scope` and
  the observed `edition`, so a scope or selection change can invalidate the
  client's view.
- Credentials: resolved server-side only after the approved binding is
  re-verified for the exact profile, endpoint and scope. A missing, invalid or
  revoked configured credential fails closed with no anonymous fallback. A
  profile with no configured credential is read anonymously only where the
  deployment permits it.
- Audit: one redacted event per read (actor, `schema.read.<kind>`, profile
  target identity, outcome); no rows, metadata values or credentials.
- InfluxDB 1.x: a database/retention-policy scope returns `unknown` with a
  `schema-1x-unverified` observation and makes no request.

**Evidence.** The gate record §8 tests are implemented in
`backend/internal/liveprobe/schema_test.go`,
`backend/internal/liveprobe/schema_security_test.go` and
`backend/internal/registryapi/schema_test.go` (allowlist and
`SELECT … INTO` attempts, membership, denied credential, timeout, oversized
response, redirect, TLS failure, cancellation, unknown edition, missing
binding, revoked credential, SSRF policy, 1.x, authorization and audit), with
`go test -race`. The gated
`TestIntegrationSchemaDiscoveryAgainstInfluxDB2/3` and
`testing/influxdb/scripts/schema-discovery.sh` passed against the real isolated
containers: measurements `environment` and `sensor`, their field keys and tag
keys were observed with the expected edition and scope, injection and an absent
measurement were refused (404), and a viewer was denied (403).

**Limitations.** InfluxDB 1.x remains unverified and is never contacted. Only
the fixed statement allowlist is executed: there is no SQL/Flux statement read,
no sample-point retrieval and no schema creation. The mapper UI, mapping drafts
and canonical-schema versioning do not exist yet.

P3-002 satisfies entry-checklist item 1 (the read surface and permission) and
item 5 (edition, commands, bounds and what stays unknown) for the read path.
P3-003 satisfies item 2 (the unknown states are surfaced and never rendered as
support) and item 3 (the schema browser performs no historian write or schema
creation) for the read-only browser. Item 4 (a reference-only mapping draft and
an export contract) remains future work: no mapping draft, canonical artifact or
export exists yet.


## 9. Implementation (P3-003, 2026-10-26)

The Canonical Mapper reads the P3-002 metadata routes through the typed browser
client (`ui/src/api/client.ts`, `ui/src/api/registry.ts`,
`ui/src/hooks/useSchemaBrowser.ts`) and a pure presentation module
(`ui/src/features/canonical/canonicalWorkflow.ts`). The page
(`ui/src/features/canonical/CanonicalMapperPage.tsx`) does not add a backend
surface and does not change the read allowlist.

- **Selection.** Two native selects, source and destination, from the active
  mock or live profile list. The pair must be distinct; selecting the same
  profile on both sides is refused before a schema request. A selected ID that
  disappears from the list is reported as missing.
- **Reads.** Only `connection_id` (measurements) and `connection_id` plus an
  observed `measurement` (fields/tags) are requested. Field/tag reads are not
  issued until a measurement has been observed and selected.
- **Freshness.** Query keys include `connection_id`, the canonical scope and
  the measurement. Each answer is checked for matching connection, scope, kind
  and measurement; a non-matching answer is a stale state, never rendered data.
  A response marked `mock` on a live path is also stale. A selection, token or
  mode change cancels the affected in-flight queries and removes their cache
  entries; the panel key includes the profile identity and scope.
- **States.** Loading, positively-supported empty, permission/authentication,
  not-found, unknown, unsupported, truncated and stale are distinct. A 401,
  403, 404, timeout, malformed answer or unrecognized body is not an empty
  schema and is not "unsupported" unless the server actually observed
  unsupported.
- **Mock mode.** The default mode is labeled `MOCK` and uses the existing typed
  profile endpoint plus a feature-local synthetic preview. The preview sets
  `mock: true`, reports no request, shows a 1.x profile as unknown, and honors a
  mock profile's unsupported schema-discovery capability. It is never imported
  by a live path and is never rendered after a live failure.
- **Safe identity.** The page renders display name, connection ID, family/major
  version, canonical scope and observed edition. It renders no endpoint or
  credential reference and receives no credential value. The session token is
  memory-only and the input is cleared after application.
- **No writes.** Every browser request is a GET; there is no mapper POST, query
  text, `SELECT … INTO`, DDL or write. The page's mapping actions remain
  disabled.

**Evidence.** `CanonicalMapperPage.test.tsx` covers distinctness, mock/live
labels, loading, empty, permission, unknown, unsupported, truncated and stale
states, token handling, safe identity, GET-only calls, keyboard activation,
heading order, unique IDs and cancellation on a selection change.
`canonicalWorkflow.test.ts` covers the pure freshness/failure/mock-preview rules;
`schemaClient.test.ts` covers the typed client and guard. The full UI gate is
`npx eslint .`, `npx tsc -b`, `npm run build` and `npx vitest run` (223 passed,
1 gated test skipped). `testing/influxdb/scripts/canonical-mapper-smoke.sh` is
the authenticated real-backend smoke: it starts the API with a throwaway token,
seeds a server-side credential, creates a profile bound to the isolated
InfluxDB 2 OSS v2.9.1 bucket, verifies the fixed GET routes (measurements
`environment`/`sensor`, `value`, `host`/`sensor_id`) and runs the typed client
against the same API; result 6/6 PASS. See [`docs/LIVE_UI.md`](../LIVE_UI.md)
for the recorded evidence.

**Limits.** No mapping draft, canonical-schema editor, validation, sample-point
preview, save/import/export or deployment exists. InfluxDB 1.x remains
unverified and is never contacted for schema metadata. No real-browser or
screen-reader session was performed; jsdom component tests plus an explicit gap
are recorded.

## 10. Implementation (P3-004, 2026-10-28)

- A **local draft** over one source and one destination measurement, held in
  page memory only (not saved, exported or executed). It records profile
  identities and canonical scope, never endpoints or credentials.
- **Validation** is deterministic and never coerces silently: type changes need
  an explicit unit transform; unknown, unloaded, stale or unobserved schema is an
  error for the dependent rule, never a pass; unknown destinations are warnings.
- **Plan preview** is text only, shown only for a draft with no errors, and
  renders names and values quoted as data. No query, sample read or write is
  built or issued.
- Destination names use a strict identifier form; filter values are data with
  length and control-character bounds.
- Evidence and limitations: `workflow/active_work` archive for P3-004
  (`archive/active-work/P3-004-mapping-drafts-and-validation.md`).



## P3-005 architecture decision (human-approved 2026-10-28; implementation pending)

The previously open canonical-schema versioning/approval model is approved for P3-005: canonical schemas and source-specific mapping profiles are separate versioned resources; approved revisions are immutable; drafts and approval states and revision history are explicit; rollback creates a new revision rather than rewriting old history. Persistence uses the existing SQLite transactional metadata-store pattern with audit and role checks. Portable JSON import/export is deterministic and versioned, with no credentials, tokens, credential references, or endpoints; only safe connection/scope identities. This decision does **not** mean P3-005 is implemented or accepted, and authorizes no historian writes, sync or deployment. Refer to `workflow/active_work/P3-005-canonical-schema-versioning.md` and `handoff.md` for the execution boundaries. Earlier open-decision wording in section 7 describes the historical state and is superseded for this question only.
