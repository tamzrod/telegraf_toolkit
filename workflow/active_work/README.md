# Active Work — Telegraf Toolkit

This persistent folder is the sole ledger for human-promoted, incomplete microtasks on the **human-designated work ref**. The work ref may be `main` for promotion or an explicitly human-approved package branch. Agent execution authority is limited to that designated branch and the task queue's authorized scope.

**Current task on `main`:** none.
**Queue on `main`:** empty.
**Phase 0 work ref:** `phase-0/foundation`, with its own human-promoted task queue and branch-local pointers. Do not infer that the empty `main` ledger cancels or expands branch-specific human authorization.

Only humans promote task `*.md` files. `README.md` is not a task. Each branch's current task and queue must agree with its `handoff.md`. Agents read only the current task body, never future task bodies.

After verified implementation, archive, commit, push and confirm each microtask checkpoint on its authorized branch. Automatically advance to the next approved task. An archived task on an unmerged branch is `TASK_CHECKPOINTED`, not `INTEGRATED`.

When all tasks are checkpointed, validate the package and report `WORK_PACKAGE_READY`; **humans alone initiate, approve and merge PRs**. If no work is authorized on the selected ref, stop.
