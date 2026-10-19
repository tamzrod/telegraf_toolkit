# Canonical Registry and Taglist Mapper

Status: **Design proposal**.

## Terminology
InfluxDB has **measurements**, **fields** (measured values), and **tags** (indexed metadata). The UI may call its browsing panel a *Taglist*, but must distinguish the three types.

## Required source and destination selection

The mapper must **first select a Source Database and a Destination Database** from the shared Database Registry, with the appropriate database/bucket scope on each side. No hard-coded source or destination, and no fixed field list.

The source explorer loads actual measurements, field keys and tag keys from the selected source database. The destination explorer loads existing destination measurements and their field/tag schema or allows a new destination measurement name. Capability and authorization checks use their registered connection profiles.

A mapping profile records the source and target connection references/scopes as well as measurement/field/tag mapping rules. Selecting a different database invalidates cached schema and prompts to revalidate or discard draft mappings. Secrets must not appear in mapping exports.

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
