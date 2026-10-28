# Active Work — Telegraf Toolkit

**Designated branch:** `phase-3/canonical-mapper`
**CURRENT:** `P3-005-canonical-schema-versioning.md` — **ARCHITECTURE APPROVED; implementation pending**
**Human-authorized queue:** P3-005 → P3-006 (execute sequentially, subject to verification and any genuinely new gate).

When `operation cwal` is invoked, execute the task marked CURRENT in both pointer files. This is the existing authorization to perform the designated task, run its verification, and on acceptance archive, commit, push, confirm the remote SHA and advance both pointers. Do not require an additional approval merely because the task packet originally said queued. Continue P3-005 and P3-006 in order, stopping for a new architectural or security decision or a failed required check. No PR actions.

P3-005 is designated CURRENT and its architecture decision was approved on 2026-10-28. Execute and verify P3-005 before P3-006. P3-001 through P3-004 are implemented, verified and archived (`archive/active-work/P3-002-authorized-schema-discovery.md`, `archive/active-work/P3-003-mapper-selection-and-schema-browser.md`, `archive/active-work/P3-004-mapping-drafts-and-validation.md`). Follow `handoff.md` and the selected packet. Verify each task, `git mv` archive, commit/push, remote-confirm then advance both pointers. Do not require intermediate approvals unless a task reaches an architectural/security decision gate.

Phase 3 consumes the Phase 2 Registry; preserve exact profile/binding/scope and pinned-peer/TLS/SSRF controls. P3-001 documented the safe schema-discovery API decision and **must not be read as silently authorizing a new query method or Proposed ADR**. No real historian writes, deployment, production endpoints or agent-created PR. Stop TASK_BLOCKED for missing required authority or evidence. On completion clear queue and report WORK_PACKAGE_READY.


## Gate history — P3-001 approved 2026-10-26; P3-005 approved 2026-10-28

P3-001 was verified and archived on 2026-10-26. The human approved Option A1,
the P2C-003 catalogue/evidence correction, a dedicated `schema.read`, and bounded
fixed GET `SHOW` statements, including deployment-dependent database
authentication without silent fallback from invalid or revoked credentials.
**Do not ask for this approval again.** Follow
`docs/decisions/phase-3-schema-discovery-gate.md` §11 and `handoff.md`.

P3-002 and P3-003 are likewise verified, archived and checkpointed: the schema
read is implemented, the P2C-003 catalogue correction is applied, and the
read-only Canonical Mapper schema browser consumes that read with explicit
distinct profiles, honest states, labeled mock mode and race-safe cancellation.
InfluxDB 1.x remains unverified. P3-004 is complete and archived. P3-005 has since received an explicit human architecture approval; begin implementation on the next `operation cwal`. No agent PR actions.
