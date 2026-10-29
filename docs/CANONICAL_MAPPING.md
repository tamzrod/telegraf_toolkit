# Canonical Registry and Taglist Mapper

Status: **Phase 3 complete on `phase-3/canonical-mapper` (P3-001–P3-006).** Read-only schema browser (P3-003) and mapping drafts/validation (P3-004) are implemented; portable canonical schema and mapping profile persistence with versioning, approval states/history, rollback via new revision, transactional SQLite metadata store with RBAC/audit, and deterministic safe JSON import/export are implemented (P3-005). Integrated acceptance completed (P3-006). Measurement-level metadata read path is implemented (Option A1, P3-002) behind `schema.read` for the pinned InfluxDB 2 OSS and 3 Core editions. InfluxDB 1.x remains unverified. Mapping compilation, deployment and historian sync are out of scope for Phase 3. See [`docs/decisions/phase-3-schema-discovery-gate.md`](decisions/phase-3-schema-discovery-gate.md).

## Terminology
InfluxDB has **measurements**, **fields** (measured values), and **tags** (indexed metadata). The UI may call its browsing panel a *Taglist*, but must distinguish the three types.

## Required source and destination selection

The mapper must **first select a Source Database and a Destination Database** from the shared Database Registry, with the appropriate database/bucket scope on each side. No hard-coded source or destination, and no fixed field list.

The source explorer loads actual measurements, field keys and tag keys from the selected source database. The destination explorer loads existing destination measurements and their field/tag schema or allows a new destination measurement name. Capability and authorization checks use their registered connection profiles.

A mapping profile records the source and target connection references/scopes as well as measurement/field/tag mapping rules. Selecting a different database invalidates cached schema and prompts to revalidate or discard draft mappings. Secrets must not appear in mapping exports.

## Implemented increment: read-only schema browser (P3-003)

The Canonical Mapper page now performs the first two steps — explicit selection
and metadata browsing — without creating a mapping:

- The operator selects a **source** and a **distinct destination** from the
  active Registry profile list (the labeled mock list in Mock mode, the live API
  list in Live mode). The same profile on both sides is refused.
- Each side shows measurements, field keys and tag keys as separate typed
  groups. Field keys retain the reported type; tag keys are a distinct labelled
  list. A field/tag read is issued only after a measurement observed on that
  side is selected.
- The live read is the authorized P3-002 GET route under `schema.read`. The
  browser supplies only the stored connection ID and an observed measurement;
  the endpoint, scope and credential are resolved server-side. No query,
  endpoint or credential is accepted from the page.
- Loading, observed-empty, permission, unknown, unsupported, not-found,
  truncated and stale answers are distinct states. Unknown is never rendered as
  support, and a failed live read never falls back to the synthetic mock
  preview.
- Query keys and panel identity include the connection ID and canonical scope
  (plus measurement for detail reads). A selection, scope, token or mode change
  cancels and removes the affected answers so an in-flight result cannot render
  into the new selection.
- Mock mode remains explicitly labeled `MOCK`; its preview is generated locally
  from the typed mock profile list, is always marked `mock`, and is never used
  on a live path. A 1.x mock profile is shown as Unknown.

There is still **no mapping draft**, no canonical-schema editor, no sample-point
preview, no save/import/export and no deployment action. The page's planned
actions remain disabled. The design below is the intended next increment
(P3-004/P3-005), not current behaviour.

## Interaction model
Browse source database -> measurement -> field/tag lists while inspecting the separately selected destination database and measurement. Drag one or multiple selected source fields onto a destination measurement; optionally rename destination measurement, fields and tag keys, preserve or set tag values, apply transforms, and choose a canonical target. Dragging only changes a mapping draft; preview, validate and explicitly save/deploy.

Support:
- Source field -> canonical field rename
- Source tag key -> destination tag key rename
- Fixed/derived tag values and per-series filter conditions
- Unit scaling/conversion (e.g. kW -> MW) with explicit configured rule
- Data type validation and destination collision detection
- Preview with sample points and exact output naming

## Portable artifacts
Keep the *canonical registry* (shared approved schema) separate from the *mapping profile* (site/device/source-specific adapter). Import/export JSON or YAML with schema version; CSV field list import/export may be offered with limited metadata. No secrets in exports.

Example canonical schema (illustrative, not established project contract):

```yaml
schema_version: 1
measurements:
  plant_metrics:
    fields:
      PlantActivePower: {type: float, unit: MW}
      PlantReactivePower: {type: float, unit: Mvar}
    tags:
      site: {type: string}
```

Example mapping profile:

```yaml
schema_version: 1
profile: meter-a-to-plant-metrics
source:
  measurement: line_power_meter
target:
  measurement: plant_metrics
fields:
  P_total: {destination: PlantActivePower, scale: 0.001}
  Q_total: {destination: PlantReactivePower, scale: 0.001}
tags:
  site: {destination: site}
```

## Contracts and safety
- Canonical names and expected units are explicit; never infer unit scaling solely from a field name.
- Reject duplicate destination identities unless conflict handling is configured.
- Support versioned profile changes, diff, rollback and validation before deployment.
- Reuse mapping semantics consistently for live collection and historical synchronization.
- Never mutate source historian data by default.
- The read-only browser already enforces the inherited Registry controls: no
  caller-supplied query, endpoint or credential; GET-only fixed metadata
  statements; server-side authorization; and no mock fallback on live failure.

## Implemented evidence and limits

The P3-003 component/unit tests cover distinctness, live/mock labeling, loading,
empty, permission, unknown, unsupported, stale and truncated states, cache
invalidation and cancellation, GET-only calls, safe identity and keyboard
activation. The authenticated real-backend smoke starts the Toolkit API, creates
a profile bound to the isolated InfluxDB 2 OSS container, reads measurements,
field keys and tag keys over the fixed GET routes, and then exercises the typed
browser client against the same API. See [`LIVE_UI.md`](LIVE_UI.md) for the
recorded commands and results.

Limitations: InfluxDB 1.x remains unverified; there is no SQL/Flux read, no
sample-point retrieval and no schema creation; the browser is not a security
boundary; and no real-browser/screen-reader session was performed (the manual
accessibility evidence is an explicit gap). Mapping drafts and canonical
artifacts are not implemented.
