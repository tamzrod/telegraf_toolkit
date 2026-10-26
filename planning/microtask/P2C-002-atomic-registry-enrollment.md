# P2C-002 — Make Registry enrollment and bindings transactional

> **State: PLANNING ONLY — not authorized.** Promotion to `workflow/active_work/` requires explicit human authorization and updated CWAL handoff.

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
Execute only when human-promoted. Complete tests, `git mv` to `archive/active-work/`, update aligned CWAL pointers, commit/push and verify remote checkpoint. Human controls PR initiation/review/merge.
