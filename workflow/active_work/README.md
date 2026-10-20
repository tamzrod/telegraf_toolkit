# Active Work — Telegraf Toolkit

**Human-authorized work package:** Phase 2 Database Registry, strictly **mock/interface-only**, branch `phase-2/database-registry`.
**Current task:** `P2-004-secret-boundary-validation.md`.
**Authorized queue (strict sequence):**
1. P2-001-architecture-and-contract-gate.md — **done (verified, archived)**
2. P2-002-registry-backend-mock-contracts.md — **done (verified, archived)**
3. P2-003-capability-probe-fixtures.md — **done (verified, archived)**
4. P2-004-secret-boundary-validation.md — **current**
5. P2-005-registry-ui-mock-workflows.md
6. P2-006-registry-quality-gate.md

The human explicitly approved and promoted all six tasks as one bounded batch. The tasks exist in this ledger, not Planning. Read only the current task body; after acceptance tests succeed, archive with `git mv`, update this pointer and `handoff.md`, commit/push, verify the remote checkpoint and automatically advance to the next authorized task.

**Nonnegotiable architecture gate:** ADR-0002 and ADR-0003 remain **Proposed**. Do not select or implement persistent metadata storage, real credential storage/resolution/key custody, or live InfluxDB access. Work only with mock/in-memory adapters, interfaces, fixtures, negative validation and clearly labeled demo UI. If a task's objective requires a human-only architecture decision, stop that part rather than silently accept an ADR.

Only humans initiate, review, approve and merge PRs. Finish with package verification and report `WORK_PACKAGE_READY`; do not open or modify a PR.
