# Active Work — Telegraf Toolkit

**Designated branch:** `phase-2/registry-completion`
**CURRENT:** `P2C-007-pre-pr-credential-lifecycle-hardening.md`
**Authorized queue:** P2C-007 only.

Previous P2C-001–P2C-006 tasks are complete and archived. Independent pre-PR audit found credential validation using potentially public edition health, deterministic shared staging ID under concurrent replacement, and revocation's out-of-transaction dependency check / secret-first deletion that can disconnect remaining profiles. Resolve the findings in the current task with adversarial tests, Linux race and isolated real InfluxDB authorization checks. Do not weaken P2L-006/007 SSRF/TLS, binding and permissions. On success archive via `git mv`, clear this pointer and handoff.md, commit/push and verify checkpoint. Stop with TASK_BLOCKED for unsatisfied safety/tests. No PR actions.
