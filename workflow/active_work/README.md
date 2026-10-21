# Active Work — Telegraf Toolkit

**Designated branch:** `phase-2/registry-completion`
**Current task:** `P2C-005-credential-enrollment-lifecycle.md`
**Human-authorized sequential queue:** P2C-001 → P2C-002 → P2C-003 → P2C-004 → P2C-005 → P2C-006.

P2C-001 (roadmap, contract and architecture reconciliation with merged PR #6 and P2L-001–007), P2C-002 (transactional Registry enrollment, bindings and audit, with binding referential integrity in migration version 6), P2C-003 (observation-based capability probes with a measured evidence matrix from the real isolated InfluxDB 2/3 containers) and P2C-004 (browser create/delete of unbound profiles) are complete, verified and archived.

All six tasks are human-authorized. Execute **only** CURRENT, then validate, archive by `git mv`, commit/push and confirm remote checkpoint before updating `handoff.md` and this README to the next task. Do not pause for intermediate approval if criteria pass. Do not promote planning tasks or reorder. Stop with `TASK_BLOCKED` if security architecture, mandatory verification, or safe environment is unavailable.

Preserve backend RBAC, secret isolation, credential bindings, DNS peer pinning and TLS/SSRF denial. Isolated synthetic-data InfluxDB 2/3 tests only; no production writes, unapproved ADR decisions or agent-created PR. Upon the final task, leave an empty queue and report `WORK_PACKAGE_READY`.
