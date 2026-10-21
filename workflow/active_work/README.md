# Active Work — Telegraf Toolkit

**Designated branch:** `phase-2/live-registry-implementation`.
**Current task:** `P2L-002-encrypted-secret-store.md`.
**Human-authorized batch:** P2L-001 through P2L-005, strictly sequential, approved 2026-10-21.

## Ordered active queue
1. `P2L-001-persistence-migrations.md` — DONE (archived, checkpointed 2026-10-21)
2. `P2L-002-encrypted-secret-store.md` — CURRENT
3. `P2L-003-server-authorized-registry-api.md` — PENDING
4. `P2L-004-live-capability-probing.md` — PENDING
5. `P2L-005-live-ui-integration-and-verification.md` — PENDING

ADR-0002 (single-host SQLite metadata store) is implemented and checkpointed in P2L-001 (`docs/METADATA_STORE.md`). ADRs 0002/0003 are Accepted for single-host SQLite and backend-only envelope-encrypted credentials with separately provisioned master keys, documented recovery and rotation. ADRs 0004/0005 remain Proposed. Live database testing is authorized only against the isolated PR #5 InfluxDB 2/3 harness, not production servers. No unrestricted SSRF/private-host bypass; scope any test loopback exception to test-only configuration.

Operation CWAL: read the current task only, implement real behavior, verify `make check` and applicable live integration/security tests, archive by `git mv`, advance pointers in both documents, commit/push and verify remote checkpoint before advancing to the next task automatically. Report `WORK_PACKAGE_READY` only when all tasks complete. Humans own PR actions.
