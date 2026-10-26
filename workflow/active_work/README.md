# Active Work — Telegraf Toolkit

**Designated branch:** `phase-3/canonical-mapper`
**CURRENT:** `P3-001-roadmap-and-discovery-gate.md`
**Human-authorized queue:** P3-001 → P3-002 → P3-003 → P3-004 → P3-005 → P3-006.

P3-001 is the sole active execution target; P3-002–006 are authorized and **queued, not concurrently executable**. Follow `handoff.md` and the selected packet. Verify each task, `git mv` archive, commit/push, remote-confirm then advance both pointers. Do not require intermediate approvals unless a task reaches an architectural/security decision gate.

Phase 3 consumes the Phase 2 Registry; preserve exact profile/binding/scope and pinned-peer/TLS/SSRF controls. P3-001 must document the safe schema-discovery API decision and **must not silently authorize a new query method or Proposed ADR**. No real historian writes, deployment, production endpoints or agent-created PR. Stop TASK_BLOCKED for missing required authority or evidence. On completion clear queue and report WORK_PACKAGE_READY.
