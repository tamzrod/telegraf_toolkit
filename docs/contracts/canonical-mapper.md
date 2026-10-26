# Phase 3 Canonical Mapper — entry contract

Status: **Entry contract — no implementation. The measurement-level read path is approved (Option A1, 2026-10-26) and not implemented.**
Task: P2C-006 (Phase 2 integration acceptance and Phase 3 handoff); reconciled by P3-001.
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

The first increment may:

- read the **metadata** of the selected source and destination (measurements,
  field keys, tag keys) through a read-only, authorized path;
- build a mapping **draft** in the browser and validate it locally;
- show the source and destination metadata side by side.

It must not:

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
| A **measurement-level** schema listing (measurements, field keys, tag keys) | **approved, not implemented** | P3-002 will add a backend-authorized GET-only read of fixed `SHOW` statements under `schema.read`; no Registry route serves it yet — see §8 |
| A **query dialect** (InfluxQL, Flux, SQL) proven to work | **unknown in the capability model** | The probe records `unknown`; the approved P2C-003 path/evidence correction is queued under P3-002 (§8) |
| A **write** protocol | **unknown** | A read-only probe never writes; no authorized synthetic write check exists |
| InfluxDB 1.x | **unverified** | No isolated 1.x instance exists |

The mapper therefore starts with an explicit **unknown** state for anything the
Registry has not observed. "Unknown" must never be rendered as "supported" or
"unsupported": it is the honest answer, and the UI vocabulary already carries
it (P2C-003).

## 4. The unknown states the mapper must model

| Unknown | What it means for the mapper |
| --- | --- |
| The deployment's edition is unknown | The mapper cannot assume a dialect or a schema endpoint; it must ask, or stop |
| The query dialect is unknown | No preview of sample points can be promised; a draft stays a draft |
| The schema listing is unknown | The measurement, field and tag lists are empty **with an explicit reason**, not silently empty |
| The credential is not approved for the profile | The mapper may not read anything from that profile; the selection is refused server-side (403) |
| The profile has no credential | The mapper may read only what the deployment serves anonymously, and must say so |

Every one of these states is a **first-class answer**, not an error to be
hidden: a draft that cannot be validated says why.

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
  behind `schema.read`. Implementation is P3-002; see §8.
- How a canonical schema is versioned, shared and approved
  ([`docs/CANONICAL_MAPPING.md`](../CANONICAL_MAPPING.md) leaves this open).
- The dependency-inspection view (which pipelines and jobs use a profile).
- Fine-grained, per-connection or per-site authorization; the Registry is
  role-based only.
- Runtime secret delivery and the host-agent boundary (ADR-0004, **Proposed**).

## 8. Decision (approved, not implemented)

The measurement-level read path is **decided and approved**, not implemented.
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

The P2C-003 v3 catalogue path/evidence correction is also approved and queued
under P3-002, preserving the historical observations.

No entry-checklist item above is satisfied yet for measurement-level discovery:
P3-002 implements it and must produce the tests listed in the gate record §8.
