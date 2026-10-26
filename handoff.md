# Telegraf Toolkit — Operation CWAL handoff

## CURRENT OPERATION
**Branch:** `phase-3/canonical-mapper`
**Current human-authorized task:** `workflow/active_work/P3-003-mapper-selection-and-schema-browser.md`
**Authorized sequence:** P3-001 → P3-002 → P3-003 → P3-004 → P3-005 → P3-006.

## Base and context
Branch created from merged `main` at `bba5a676c40cdbb5a3af87177ddfa979e59b5fdb` (PR #7 merged). Phase 2 Registry is substantially delivered; exit remains partial because downstream Registry consumption and query/write capability verification remain incomplete. P2C-001–P2C-008 are archived. Phase 3 entry contract: `docs/contracts/canonical-mapper.md`.

## Current task
P3-003 — live dual-profile selection and typed schema browser. Read `workflow/active_work/P3-003-mapper-selection-and-schema-browser.md`. Begin only after verifying branch, clean worktree, and current pointers.

## Task status — P3-002 COMPLETE and ARCHIVED; CURRENT is P3-003 (2026-10-26)

P3-002 is accepted and archived at
`archive/active-work/P3-002-authorized-schema-discovery.md`. It implements the
approved Option A1 read path:

- `GET /api/v1/registry/schema/measurements`, `…/fields` and `…/tags` under a
  dedicated `schema.read` permission (operator/configurator/administrator;
  viewer denied), distinct from `profile.probe`.
- Fixed GET `SHOW MEASUREMENTS` / `SHOW FIELD KEYS FROM <validated measurement>` /
  `SHOW TAG KEYS FROM <validated measurement>` only; membership verification,
  identifier escaping, per-edition adapters (2.x DBRP resolution; 3.x
  `query_influxql`), bounded 15 s/5 s/8-request/1 000-row/1 MiB limits, explicit
  truncation, cancellation, three-valued observations, profile-only identity,
  binding re-verification, fail-closed credentials and redacted audit.
- The approved P2C-003 v3 catalogue correction: 3 Core InfluxQL and SQL are now
  observed `supported` through the measured GET routes; 2 OSS query dialects
  remain `unknown`; InfluxDB 1.x remains unverified and is never contacted.
  Historical observations are preserved in `docs/LIVE_PROBING.md`.

Verification: `gofmt`, `go vet`, `go test ./...`, `go test -race` on
`liveprobe`/`registryapi`/`authz`/`registryservice`, `make check`
(188/188 UI tests), policy check, a 96-link/0-broken docs check, and the live
`testing/influxdb/scripts/schema-discovery.sh` — PASS against the isolated
InfluxDB 2 OSS v2.9.1 and 3 Core v3.12.1 containers (measurements
`environment`/`sensor`, one field, tags `host`/`sensor_id`; injection and absent
measurement → 404; viewer → 403). The read path issued GET requests only; the
containers kept their synthetic state; no deployment, historian write or PR.

The mapper UI, mapping drafts, canonical-schema versioning and 1.x coverage do
not exist. CURRENT is **P3-003** — live dual-profile selection and typed schema
browser. Do not start P3-004 early.

## Authorization and execution policy
The six bounded task packets are human-authorized for **sequential** Operation CWAL execution without intermediate approval within accepted architecture and safety boundaries. Execute only CURRENT; test, archive via `git mv`, commit/push, verify remote checkpoint and advance CURRENT in this file plus the Active Work README. Do not execute later tasks early. If a task raises a new architectural approval (including introducing a protected POST query path beyond the existing GET-only probe) obtain explicit human decision via TASK_BLOCKED; the task queue is not authority to approve ADR-0004/0005.

## Security and stop criteria
Use only the isolated synthetic InfluxDB 2 OSS and 3 Core test systems. No production historian writes/credentials, no deployment, no runtime management, no secret disclosure, no weakening administrator-only binding, RBAC, SQLite lifecycle fence, pinned-peer/DNS/TLS/SSRF constraints. All capability outcomes must retain actual evidence or explicit unknown. Stop TASK_BLOCKED for unresolved architecture, unsafe access or missing mandatory verification. No agent PR actions.

## Final handoff
After P3-006, archive, leave `workflow/active_work/` with README only, set both pointers queue empty, commit/push and independently verify remote HEAD; report WORK_PACKAGE_READY. Human owns PR and merge.


## Binding approved decision (2026-10-26) — do not re-request

The human approved the four P3-001 architecture decisions. P3-001 is executed,
verified and archived; both pointers now designate P3-002. **Do not request this
approval again.** Source of truth:
`docs/decisions/phase-3-schema-discovery-gate.md` §11 (supersedes pending §9
wording).

Implement the approved **Option A1** only within its designated subsequent task: backend-authorized GET-only fixed `SHOW MEASUREMENTS`, `SHOW FIELD KEYS FROM <validated measurement>`, `SHOW TAG KEYS FROM <validated measurement>`; separate `schema.read` from `profile.probe`; prohibit passthrough, POST queries, `SELECT ... INTO`, DDL or any writes; apply timeout/caps/truncation/cancellation/audit redaction and inherited pinned-peer/TLS/SSRF protections. Correct P2C-003's mismatched v3 catalogue query routes and derived conclusions under the explicit new approval, preserving historical evidence. Toolkit RBAC is mandatory. Intentionally credential-free profiles may read without DB credentials only where the DB permits; configured credentials must have approved bindings, with invalid/revoked credentials failing closed (no anonymous fallback). Bound query-free absence claims to tested endpoints/editions.

P3-001 and P3-002 are archived. Continue P3-003–P3-006 sequentially per packets, with the same acceptance, archive, commit/push and remote-SHA confirmation for each. Stop for genuinely new architectural/security blockers. **No PR creation, approval or merge; no real historian writes or deployment.** Approval itself is not an acceptance or implemented capability. The earlier blocked-status history above is retained as historical context.
