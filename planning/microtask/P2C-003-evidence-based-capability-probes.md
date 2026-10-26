# P2C-003 — Report capability observations without edition-based assumptions

> **State: PLANNING ONLY — not authorized.** Promotion to `workflow/active_work/` requires explicit human authorization and updated CWAL handoff.

## Intent
Review backend/internal/liveprobe/probe.go, docs/LIVE_PROBING.md and UI transformations. Implement bounded nonmutating query/schema capability checks where supported, preserving pinned peer DNS protection, server-authorized profile selection and sanitized outputs.

## Starting state
Base branch: `phase-2/registry-completion`, created from `main` following merged PR #6. The operational SQLite Registry, envelope secret store, authorized APIs, secure live probing and partial live UI are implemented, with P2L-006/007 security controls. ADR-0002/0003 Accepted, ADR-0004/0005 Proposed.

## Required result
Actual read-only observations drive supported/unsupported/unknown; unknown on ambiguous or unsafe checks, no capabilities asserted solely by edition; label write protocols unverified unless separately proven by an authorized isolated write check.

## Execution boundary
- Allowed: relevant `backend/`, `ui/`, `docs/` and isolated `testing/influxdb/` artifacts as needed for this task only.
- Dependencies: P2C-001, P2C-002.
- Explicit non-goals: No production historian writes; no uncontrolled remote targets; do not infer InfluxDB 1.x; write probes are NOT allowed in read-only route.
- No unauthorized privileged or destructive actions; no production historian or credential access.

## Acceptance criteria
- [ ] Actual read-only observations drive supported/unsupported/unknown; unknown on ambiguous or unsafe checks, no capabilities asserted solely by edition; label write protocols unverified unless separately proven by an authorized isolated write check.
- [ ] Preserve server-side RBAC, administrator-only credential binding, P2L-006 pinned-peer/TLS/SSRF controls, and secrets/redaction invariants.
- [ ] Evidence corresponds to actual tested behavior; unsupported/unverified remains explicitly labelled.
- [ ] No new secrets, deployment or production historian writes.

## Verification evidence
Isolated InfluxDB 2 OSS and 3 Core probes plus malicious endpoint/timeout/negative auth tests; make check and race tests.

## Stop conditions
Stop and report `TASK_BLOCKED` for unsafe network/credential access, unresolved architecture, or required testing unavailable. No weakening tests or replacing working behavior with mocks.

## Git lifecycle
Execute only when human-promoted. Complete tests, `git mv` to `archive/active-work/`, update aligned CWAL pointers, commit/push and verify remote checkpoint. Human controls PR initiation/review/merge.
