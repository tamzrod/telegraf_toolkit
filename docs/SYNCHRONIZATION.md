# Cross-Version InfluxDB Synchronization

Status: **Design proposal**.

## Goal
Copy and reconcile historical data between selectable InfluxDB databases (1.x, 2.x, 3.x, depending on capabilities), at measurement/table and field selection granularity, with a persistent time-block index.

## Selection workflow
1. Register two database connection profiles.
2. Drag a source database, measurement, or field onto a target database/measurement in the Sync Designer.
3. Choose destination names and optional canonical mapping profile.
4. Choose time range, block duration, direction, verification policy, and whether the job is one-time or scheduled.
5. Preview and explicitly start; show per-block progress and errors.

A drag/drop creates a **job proposal**, not an unreviewed write.

## Block identity
A block belongs to a job + source measurement + mapped selection + `[start,end)` interval. Blocks are per measurement, not global to a database. All boundaries are UTC with configurable block sizes. Keep the in-progress/current block unfinalized until a lateness/grace window expires.

## Comparison
- Discover eligible measurements and the source time coverage.
- Compare point counts for each matching time block on source and destination.
- `equal count` means **count-match**, not verified equivalence. Count equality can conceal changed values, tags, and timestamps; query representation may differ across versions.
- `unequal count` or missing range marks a block as requiring inspection.
- Optional deeper verification compares normalized point identities (measurement + tags + timestamp) and field values, or a deterministic canonical digest where supported.
- For multi-field points, define whether the counter tracks rows, field values, or unique series/timestamp identities; do not assume InfluxDB query engines count them identically.

## Proposed block state machine
`PENDING -> SCANNING -> COUNT_MATCH | DIRTY | MISSING | ERROR`
`DIRTY/MISSING -> QUEUED -> COPYING -> VERIFYING -> VERIFIED | NEEDS_REVIEW | ERROR`

The state `COUNT_MATCH` must not be displayed as fully `VERIFIED` without the requested verification policy being satisfied.

## Repair and resume
- Fetch source points using source-specific query adapters (InfluxQL, Flux, SQL as available and compatible).
- Normalize selected points to a common mapping model, preserving timestamp precision, tags, field types and series identity.
- Write via a destination-compatible transport, possibly Telegraf.
- Record attempt IDs, read/write progress, checkpoint/watermark, last error and verification result.
- Retries must be idempotent. Define handling for existing points, collisions, field-type conflicts, duplicate timestamps and out-of-order writes before enabling automatic overwrite.
- Never delete destination-only data without an explicit separate policy.
- Backpressure and bounded batches prevent repair from starving live collection.

## Version considerations
A common line-protocol representation simplifies many writes, but does not guarantee full feature parity. Query endpoints, retention policies, buckets/databases, auth, timestamps, schema behavior and write support differ by InfluxDB version and product edition. Capability-check each connection before offering a job.

## Acceptance scenarios
- Entire measurement absent from target
- One historical hour missing in otherwise complete history
- Source/target counts equal but values differ
- Target has extra points
- Partially completed repair followed by restart
- Ongoing current block while new writes arrive
- Field rename and kW-to-MW transformation during backfill
- Cross-version incompatible source field type or unsupported query syntax
