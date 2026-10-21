# Telegraf Toolkit — CWAL handoff

## CURRENT OPERATION
**Workflow:** operational Phase 2 Database Registry integration.
**Designated branch:** `phase-2/live-registry-implementation` (based on `main` after merged PR #5).
**Current human-authorized task:** `workflow/active_work/P2L-005-live-ui-integration-and-verification.md`.
**Authorized batch:** P2L-001 through P2L-005, strictly sequential; human-approved 2026-10-21.

## Current Active Task
**P2L-005** — Connect the UI to the live Registry API (profiles, credential references, capability probes) and verify end-to-end against the isolated harness, replacing mock labeling only where functionality is truly connected.

**Completed and checkpointed:** P2L-001 (persistent SQLite metadata), P2L-002 (envelope-encrypted secret store), P2L-003 (server-authorized Registry API) and P2L-004 (read-only live capability probing) — archived to `archive/active-work/`.

## Authorized queue
1. `P2L-001-persistence-migrations.md` — DONE (archived, checkpointed)
2. `P2L-002-encrypted-secret-store.md` — DONE (archived, checkpointed)
3. `P2L-003-server-authorized-registry-api.md` — DONE (archived, checkpointed)
4. `P2L-004-live-capability-probing.md` — DONE (archived, checkpointed)
5. `P2L-005-live-ui-integration-and-verification.md` — CURRENT

## Accepted architecture
Human accepted ADR-0002: SQLite control-plane metadata for initial single-host Registry, future PostgreSQL migration only by separate decision.
Human accepted ADR-0003: backend-only envelope-encrypted secret store, separately provisioned protected host/Compose key file, key recovery and stage/rewrap/verify/retire rotation. Keys never in metadata DB/backups; no plaintext browser outputs.
ADR-0004/0005 remain Proposed.

## Integration targets and safety
Real isolated InfluxDB 2 OSS v2.9.1 and InfluxDB 3 Core v3.12.1 test containers are merged into main via PR #5; use ONLY the non-production `testing/influxdb/` stack. No production historians, real production credentials, uncontrolled remote probing, privilege expansion or live runtime deployment. Probe loopback exceptions must be explicit test-only, never a production-default bypass. InfluxDB 1.x is not a verified live target.

## Execution
Human promoted the entire five-task package, so OpenHands may execute sequentially without intermediate human gates, with per-task verification, `git mv` archive, commit, push and confirmed remote checkpoint. At completion update pointers to empty queue and report `WORK_PACKAGE_READY`; stop with `TASK_BLOCKED` for genuine security/host blockers. Never create/review/merge a PR.
