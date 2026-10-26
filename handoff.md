# Telegraf Toolkit — Operation CWAL handoff

## CURRENT OPERATION
**Branch:** `phase-3/canonical-mapper`
**Current human-authorized task:** `workflow/active_work/P3-002-authorized-schema-discovery.md`
**Authorized sequence:** P3-001 → P3-002 → P3-003 → P3-004 → P3-005 → P3-006.

## Base and context
Branch created from merged `main` at `bba5a676c40cdbb5a3af87177ddfa979e59b5fdb` (PR #7 merged). Phase 2 Registry is substantially delivered; exit remains partial because downstream Registry consumption and query/write capability verification remain incomplete. P2C-001–P2C-008 are archived. Phase 3 entry contract: `docs/contracts/canonical-mapper.md`.

## Current task
P3-001 — roadmap reconciliation, live schema-discovery architecture and API/security gate. Read `workflow/active_work/P3-001-roadmap-and-discovery-gate.md`. Begin only after verifying branch, clean worktree, and current pointers.

## Task status — P3-001 COMPLETE and ARCHIVED; CURRENT is P3-002 (2026-10-26)

P3-001 is accepted and archived at
`archive/active-work/P3-001-roadmap-and-discovery-gate.md`. The roadmap, README,
ICC, Phase 3 entry contract, contract inventory, `docs/LIVE_PROBING.md` and
`docs/CANONICAL_MAPPING.md` are reconciled with merged PR #7 (P2C-001–P2C-008),
and the schema-discovery gate is decided: the human approved **Option A1** on
2026-10-26 (`docs/decisions/phase-3-schema-discovery-gate.md` §11).

Approved scope for the queued tasks: a backend-authorized, **GET-only** metadata
read executing only fixed `SHOW MEASUREMENTS`, `SHOW FIELD KEYS FROM <validated
measurement>` and `SHOW TAG KEYS FROM <validated measurement>`; a dedicated
`schema.read` permission distinct from `profile.probe`; no caller-supplied query,
POST query route, `SELECT ... INTO`, DDL, deletion, writes, sample-point
retrieval, deployment or synchronization; bounded timeouts/caps/truncation/
cancellation and redacted audit; inherited pinned-peer/TLS/SSRF and credential
controls. A profile with no configured credential may read without database
credentials only where the deployment permits it; a configured credential needs
an approved binding and fails closed with no anonymous fallback. The approved
P2C-003 v3 catalogue-path correction lands under P3-002 and preserves the
historical observations.

Nothing is implemented yet: the capability model is unchanged and dialect
outcomes remain `unknown` until P3-002. Earlier `TASK_BLOCKED` history is
retained in the archive and in the decision record; it is superseded.

## Authorization and execution policy
The six bounded task packets are human-authorized for **sequential** Operation CWAL execution without intermediate approval within accepted architecture and safety boundaries. Execute only CURRENT; test, archive via `git mv`, commit/push, verify remote checkpoint and advance CURRENT in this file plus the Active Work README. Do not execute later tasks early. If P3-001 raises a new architectural approval (including introducing a protected POST query path beyond the existing GET-only probe) obtain explicit human decision via TASK_BLOCKED; the task queue is not authority to approve ADR-0004/0005.

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

P3-001 is archived. Continue P3-002–P3-006 sequentially per packets, with the same acceptance, archive, commit/push and remote-SHA confirmation for each. Stop for genuinely new architectural/security blockers. **No PR creation, approval or merge; no real historian writes or deployment.** Approval itself is not an acceptance or implemented capability. The earlier blocked-status history above is retained as historical context.
