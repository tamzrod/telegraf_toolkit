# Telegraf Toolkit — Operation CWAL handoff

## CURRENT OPERATION
**Branch:** `phase-3/canonical-mapper`
**Current human-authorized task:** `workflow/active_work/P3-001-roadmap-and-discovery-gate.md`
**Authorized sequence:** P3-001 → P3-002 → P3-003 → P3-004 → P3-005 → P3-006.

## Base and context
Branch created from merged `main` at `bba5a676c40cdbb5a3af87177ddfa979e59b5fdb` (PR #7 merged). Phase 2 Registry is substantially delivered; exit remains partial because downstream Registry consumption and query/write capability verification remain incomplete. P2C-001–P2C-008 are archived. Phase 3 entry contract: `docs/contracts/canonical-mapper.md`.

## Current task
P3-001 — roadmap reconciliation, live schema-discovery architecture and API/security gate. Read `workflow/active_work/P3-001-roadmap-and-discovery-gate.md`. Begin only after verifying branch, clean worktree, and current pointers.

## Task status — TASK_BLOCKED at the P3-001 gate (2026-10-26)

P3-001 is **not accepted and not archived**. `CURRENT` remains P3-001;
P3-002–P3-006 remain queued and must not start. The authorized part of P3-001 is
complete — roadmap, README/ICC, Phase 3 entry contract, contract inventory,
`docs/LIVE_PROBING.md` and `docs/CANONICAL_MAPPING.md` reconciled with PR #7 —
and the decision record is written:
`docs/decisions/phase-3-schema-discovery-gate.md`.

Measured finding (isolated synthetic InfluxDB 2 OSS v2.9.1 and 3 Core v3.12.1,
GET-only, loopback, no writes, 2026-10-26): neither edition exposes a query-free
measurement listing, but both answer fixed **GET** metadata statements
(`GET /query` on 2 OSS with the DBRP database name; `GET /api/v3/query_influxql`
on 3 Core), returning measurements, field keys/types and tag keys. This
contradicts the published P2C-003 conclusion that the editions serve a query
endpoint only for POST, and shows the probe catalogue's v3 paths are mismatched
(`/api/v3/query/{sql,influxql}` return 404; the serving paths use an underscore).
Correcting the catalogue changes published capability outcomes and is outside
P3-001's authority.

Human decisions required before P3-002 may start: (1) authorize a metadata
query-execution read path at all, even GET/SHOW-only; (2) authorize correction of
the P2C-003 evidence and the probe's v3 query paths; (3) choose the permission
(reuse `profile.probe` or introduce `schema.read`); (4) confirm the fixed
statement allowlist and bounds. The capability model is unchanged until then; no
ADR is accepted and ADR-0004/0005 remain Proposed.

## Authorization and execution policy
The six bounded task packets are human-authorized for **sequential** Operation CWAL execution without intermediate approval within accepted architecture and safety boundaries. Execute only CURRENT; test, archive via `git mv`, commit/push, verify remote checkpoint and advance CURRENT in this file plus the Active Work README. Do not execute later tasks early. If P3-001 raises a new architectural approval (including introducing a protected POST query path beyond the existing GET-only probe) obtain explicit human decision via TASK_BLOCKED; the task queue is not authority to approve ADR-0004/0005.

## Security and stop criteria
Use only the isolated synthetic InfluxDB 2 OSS and 3 Core test systems. No production historian writes/credentials, no deployment, no runtime management, no secret disclosure, no weakening administrator-only binding, RBAC, SQLite lifecycle fence, pinned-peer/DNS/TLS/SSRF constraints. All capability outcomes must retain actual evidence or explicit unknown. Stop TASK_BLOCKED for unresolved architecture, unsafe access or missing mandatory verification. No agent PR actions.

## Final handoff
After P3-006, archive, leave `workflow/active_work/` with README only, set both pointers queue empty, commit/push and independently verify remote HEAD; report WORK_PACKAGE_READY. Human owns PR and merge.
