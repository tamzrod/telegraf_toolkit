# Active Work — Telegraf Toolkit

**Designated branch:** `phase-3/canonical-mapper`
**CURRENT:** `P3-003-mapper-selection-and-schema-browser.md`
**Human-authorized queue:** P3-003 → P3-004 → P3-005 → P3-006.

P3-003 is the sole active execution target; P3-004–006 are authorized and **queued, not concurrently executable**. P3-002 is implemented, verified and archived (`archive/active-work/P3-002-authorized-schema-discovery.md`). Follow `handoff.md` and the selected packet. Verify each task, `git mv` archive, commit/push, remote-confirm then advance both pointers. Do not require intermediate approvals unless a task reaches an architectural/security decision gate.

Phase 3 consumes the Phase 2 Registry; preserve exact profile/binding/scope and pinned-peer/TLS/SSRF controls. P3-001 must document the safe schema-discovery API decision and **must not silently authorize a new query method or Proposed ADR**. No real historian writes, deployment, production endpoints or agent-created PR. Stop TASK_BLOCKED for missing required authority or evidence. On completion clear queue and report WORK_PACKAGE_READY.


## Approved gate — decided 2026-10-26; P3-002 is CURRENT

P3-001 was verified and archived on 2026-10-26. The human approved Option A1,
the P2C-003 catalogue/evidence correction, a dedicated `schema.read`, and bounded
fixed GET `SHOW` statements, including deployment-dependent database
authentication without silent fallback from invalid or revoked credentials.
**Do not ask for this approval again.** Follow
`docs/decisions/phase-3-schema-discovery-gate.md` §11 and `handoff.md`.

P3-002 is now likewise verified, archived and checkpointed: the schema read is
implemented and the P2C-003 catalogue correction is applied, while InfluxDB 1.x
remains unverified. CURRENT is **P3-003**; do not start P3-004 early. No agent
PR actions.
