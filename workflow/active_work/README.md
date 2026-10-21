# Active Work — Telegraf Toolkit

**Designated branch:** `phase-2/registry-completion`
**CURRENT:** `P2C-008-pre-pr-lifecycle-race.md`
**Authorized queue:** P2C-008 only.

Pre-PR audit finding: ciphertext CAS does not by itself serialize credential replacement with durable metadata revocation or binding/profile changes. Implement a verified lifecycle fence without holding a database writer transaction through network validation. Follow current task for deterministic race/failure tests and real isolated InfluxDB 2/3 evidence. Preserve secure probe, binding, RBAC, secret hygiene, and no production access. Archive/commit/push/remote-confirm and clear both pointers on full success; otherwise TASK_BLOCKED. Human owns PR actions.
