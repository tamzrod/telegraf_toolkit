# Active Work — Telegraf Toolkit

This persistent folder is the sole ledger for human-promoted, incomplete microtasks on the **human-designated work ref**. The human has explicitly promoted P0-001 through P0-004 to the designated `phase-0/foundation` branch for Phase 0 delivery. Agent execution authority is limited to that branch and the queue's authorized scope.

**Current task on this branch:** none. All four human-promoted Phase 0 microtasks are checkpointed; the queue is empty.

**Authorized incomplete queue (strict sequence):**
None. No further work is authorized on this ref; do not self-promote.

**Checkpointed on this branch (no longer Active Work):**
- P0-001-domain-contract-inventory.md — `TASK_CHECKPOINTED`; archived at `archive/active-work/P0-001-domain-contract-inventory.md`.
- P0-002-architecture-decision-records.md — `TASK_CHECKPOINTED`; archived at `archive/active-work/P0-002-architecture-decision-records.md`.
- P0-003-scaffold-toolchain-and-ci.md — `TASK_CHECKPOINTED`; archived at `archive/active-work/P0-003-scaffold-toolchain-and-ci.md`.
- P0-004-local-fixtures-and-developer-guide.md — `TASK_CHECKPOINTED`; archived at `archive/active-work/P0-004-local-fixtures-and-developer-guide.md`.

Only humans promote task `*.md` files. `README.md` is not a task. Read only the current task body; never preload queued task bodies. Never select Planning work. If no work is authorized on the selected ref, stop.

After verified implementation, archive, commit, push and confirm each microtask checkpoint on this branch, then automatically advance to the next approved task. An archived task on an unmerged branch is `TASK_CHECKPOINTED`, not `INTEGRATED`. When all tasks are checkpointed, validate the package and report `WORK_PACKAGE_READY`; **humans alone initiate, approve and merge PRs**.
