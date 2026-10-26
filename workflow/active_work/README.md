# Active Work — Telegraf Toolkit

**Designated branch:** `phase-3/canonical-mapper`
**CURRENT:** `P3-004-mapping-drafts-and-validation.md`
**Human-authorized queue:** P3-004 → P3-005 → P3-006.

P3-004 is the sole active execution target; P3-005–006 are authorized and **queued, not concurrently executable**. P3-002 and P3-003 are implemented, verified and archived (`archive/active-work/P3-002-authorized-schema-discovery.md`, `archive/active-work/P3-003-mapper-selection-and-schema-browser.md`). Follow `handoff.md` and the selected packet. Verify each task, `git mv` archive, commit/push, remote-confirm then advance both pointers. Do not require intermediate approvals unless a task reaches an architectural/security decision gate.

Phase 3 consumes the Phase 2 Registry; preserve exact profile/binding/scope and pinned-peer/TLS/SSRF controls. P3-001 documented the safe schema-discovery API decision and **must not be read as silently authorizing a new query method or Proposed ADR**. No real historian writes, deployment, production endpoints or agent-created PR. Stop TASK_BLOCKED for missing required authority or evidence. On completion clear queue and report WORK_PACKAGE_READY.


## Approved gate — decided 2026-10-26; P3-004 is CURRENT

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
InfluxDB 1.x remains unverified. CURRENT is **P3-004**; do not start P3-005
early. No agent PR actions.
