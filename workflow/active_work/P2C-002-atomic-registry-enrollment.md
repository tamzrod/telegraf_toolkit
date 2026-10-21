# P2C-002 — Make Registry enrollment and bindings transactional

> **State: HUMAN-AUTHORIZED ACTIVE WORK.** Approved six-task sequence on `phase-2/registry-completion`. Current task is selected by `handoff.md` and `workflow/active_work/README.md`; execute sequentially P2C-001 → P2C-006 and never skip dependencies.

## Intent
Inspect backend/internal/registryservice/service.go and SQLite repositories for multi-step create/bind/audit and delete/recreate failure risks. Implement an all-or-nothing transactional service/storage operation preserving administrator-only credential.attach permission and exact credential/endpoint/scope binding.

## Starting state
Base branch: `phase-2/registry-completion`, created from `main` following merged PR #6. The operational SQLite Registry, envelope secret store, authorized APIs, secure live probing and partial live UI are implemented, with P2L-006/007 security controls. ADR-0002/0003 Accepted, ADR-0004/0005 Proposed.

## Required result
Fault injection at every write; no orphaned profile/binding, no privilege bypass, verified restart and concurrent operations; add migration only if needed.

## Execution boundary
- Allowed: relevant `backend/`, `ui/`, `docs/` and isolated `testing/influxdb/` artifacts as needed for this task only.
- Dependencies: P2C-001.
- Explicit non-goals: No production DB, no new credential endpoint, no weakening P2L-007 protections.
- No unauthorized privileged or destructive actions; no production historian or credential access.

## Acceptance criteria
- [ ] Fault injection at every write; no orphaned profile/binding, no privilege bypass, verified restart and concurrent operations; add migration only if needed.
- [ ] Preserve server-side RBAC, administrator-only credential binding, P2L-006 pinned-peer/TLS/SSRF controls, and secrets/redaction invariants.
- [ ] Evidence corresponds to actual tested behavior; unsupported/unverified remains explicitly labelled.
- [ ] No new secrets, deployment or production historian writes.

## Verification evidence
Go unit/integration and race tests pass, make check passes; record isolation level and rollback evidence.

## Stop conditions
Stop and report `TASK_BLOCKED` for unsafe network/credential access, unresolved architecture, or required testing unavailable. No weakening tests or replacing working behavior with mocks.

## Git lifecycle
Human-promoted and authorized as part of the six-task queue. Execute only when designated CURRENT; advance one task at a time. Complete tests, `git mv` to `archive/active-work/`, update aligned CWAL pointers, commit/push and verify remote checkpoint. Human controls PR initiation/review/merge.
