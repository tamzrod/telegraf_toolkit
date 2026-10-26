# Telegraf Toolkit — Operation CWAL handoff

## OPERATION CWAL — EXECUTION DIRECTIVE (binding)

When the operator enters `operation cwal`, **perform the current authorized task immediately**. This is an execution command, not a request to restate status, ask if work should begin, or ask for another approval. The existing human authorization of P3-001 through P3-006 is sufficient for each packet once it becomes CURRENT in BOTH pointers.

For the present checkpoint, CURRENT is **P3-004**. Implement editable typed mapping drafts and validation within its packet; run prescribed tests, document real evidence, and accept/archive it only if criteria pass. Then `git mv` to `archive/active-work/`, update both pointers to P3-005, commit, push and confirm remote SHA. Continue P3-005 and P3-006 sequentially in the same operation when feasible, respecting the one-CURRENT-at-a-time rule. On completed P3-006 report WORK_PACKAGE_READY. If blocked, report a specific TASK_BLOCKED reason with evidence and leave pointers unchanged.

**Human approval is NOT required for each micro-task, for task acceptance after passing its prescribed criteria, for archival, or for pointer advancement.** Stop only for a genuinely new architecture/security authorization, unsafe operation, or failed mandatory verification. Never create, approve or merge a PR; never write to a historian or deploy. The word "queued" in packet boilerplate means awaiting designation as CURRENT, not awaiting repeated approval.

This directive is the standing instruction for resumed CWAL execution at P3-004. The CURRENT task packet's initial queued wording is a template state, superseded by both CURRENT pointers. Successful task acceptance does not require a separate approval when all prescribed checks pass. Record any verification gap honestly and stop at an actual new decision gate.

Read this directive before the historical status narrative below. Do not reply with only a status summary to an `operation cwal` command.


**Branch:** `phase-3/canonical-mapper`
**Current human-authorized task:** `workflow/active_work/P3-004-mapping-drafts-and-validation.md`
**Authorized sequence:** P3-001 → P3-002 → P3-003 → P3-004 → P3-005 → P3-006.

## Base and context
Branch created from merged `main` at `bba5a676c40cdbb5a3af87177ddfa979e59b5fdb` (PR #7 merged). Phase 2 Registry is substantially delivered; exit remains partial because downstream Registry consumption and query/write capability verification remain incomplete. P2C-001–P2C-008 are archived. Phase 3 entry contract: `docs/contracts/canonical-mapper.md`.

## Current task
P3-004 — editable typed mapping drafts and validation. Read `workflow/active_work/P3-004-mapping-drafts-and-validation.md`. Begin only after verifying branch, clean worktree, and current pointers.

## Task status — P3-003 COMPLETE and ARCHIVED; CURRENT is P3-004 (2026-10-26)

P3-003 is accepted and archived at
`archive/active-work/P3-003-mapper-selection-and-schema-browser.md`. The
Canonical Mapper now has a read-only schema browser:

- Explicit source and distinct destination profile selection with stored
  canonical scope; the same profile on both sides is refused.
- Typed measurement/field/tag groups over the P3-002 fixed GET routes under
  `schema.read`; the browser sends only `connection_id` and an observed
  `measurement`, never an endpoint, credential or query.
- Loading, positively-supported empty, permission, not-found, unknown,
  unsupported, truncated and stale states; unknown is never support, and live
  failure never falls back to the labeled mock preview.
- Query keys and panel identity include connection ID and canonical scope (plus
  measurement for detail reads); selection/token/mode changes cancel and remove
  affected answers, with a delayed-request test proving the signal aborts.
- No endpoint or credential reference rendered; token memory-only; every
  browser request is a GET; mapping actions remain disabled.
- Accessibility: native controls, `aria-pressed`, labelled lists and
  alert/status regions; keyboard activation, heading order and unique IDs tested.

Verification: UI lint, typecheck and build; `npx vitest run` 223 active tests
passed / 1 gated test skipped; `make check` exit 0; policy check PASS; 93
relative links / 0 broken; authenticated real-backend smoke
`testing/influxdb/scripts/canonical-mapper-smoke.sh` 6/6 PASS against isolated
InfluxDB 2 OSS v2.9.1, and the typed browser client read the same live profile.
Manual real-browser/screen-reader evidence is an explicit gap (jsdom tests
only). The read path issued GETs only; the containers kept their synthetic
state; no deployment, historian write or PR.

Mapping drafts, validation, versioned persistence/export and InfluxDB 1.x
coverage do not exist. CURRENT is **P3-004** — editable typed mapping drafts and
validation. Do not start P3-005 early.

## Authorization and execution policy
The six bounded task packets are human-authorized for **sequential** Operation CWAL execution without intermediate approval within accepted architecture and safety boundaries. Execute only CURRENT; test, archive via `git mv`, commit/push, verify remote checkpoint and advance CURRENT in this file plus the Active Work README. Do not execute later tasks early. If a task raises a new architectural approval (including introducing a protected POST query path beyond the existing GET-only probe) obtain explicit human decision via TASK_BLOCKED; the task queue is not authority to approve ADR-0004/0005.

## Security and stop criteria
Use only the isolated synthetic InfluxDB 2 OSS and 3 Core test systems. No production historian writes/credentials, no deployment, no runtime management, no secret disclosure, no weakening administrator-only binding, RBAC, SQLite lifecycle fence, pinned-peer/DNS/TLS/SSRF constraints. All capability outcomes must retain actual evidence or explicit unknown. Stop TASK_BLOCKED for unresolved architecture, unsafe access or missing mandatory verification. No agent PR actions.

## Final handoff
After P3-006, archive, leave `workflow/active_work/` with README only, set both pointers queue empty, commit/push and independently verify remote HEAD; report WORK_PACKAGE_READY. Human owns PR and merge.


## Binding approved decision (2026-10-26) — do not re-request

The human approved the four P3-001 architecture decisions. P3-001, P3-002 and P3-003 are executed,
verified and archived; both pointers now designate P3-004. **Do not request this
approval again.** Source of truth:
`docs/decisions/phase-3-schema-discovery-gate.md` §11 (supersedes pending §9
wording).

Implement the approved **Option A1** only within its designated subsequent task: backend-authorized GET-only fixed `SHOW MEASUREMENTS`, `SHOW FIELD KEYS FROM <validated measurement>`, `SHOW TAG KEYS FROM <validated measurement>`; separate `schema.read` from `profile.probe`; prohibit passthrough, POST queries, `SELECT ... INTO`, DDL or any writes; apply timeout/caps/truncation/cancellation/audit redaction and inherited pinned-peer/TLS/SSRF protections. Correct P2C-003's mismatched v3 catalogue query routes and derived conclusions under the explicit new approval, preserving historical evidence. Toolkit RBAC is mandatory. Intentionally credential-free profiles may read without DB credentials only where the DB permits; configured credentials must have approved bindings, with invalid/revoked credentials failing closed (no anonymous fallback). Bound query-free absence claims to tested endpoints/editions.

P3-001, P3-002 and P3-003 are archived. Continue P3-004–P3-006 sequentially per packets, with the same acceptance, archive, commit/push and remote-SHA confirmation for each. Stop for genuinely new architectural/security blockers. **No PR creation, approval or merge; no real historian writes or deployment.** Approval itself is not an acceptance or implemented capability. The earlier blocked-status history above is retained as historical context.
