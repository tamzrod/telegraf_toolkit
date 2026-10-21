# P2C-003 — Report capability observations without edition-based assumptions

> **State: HUMAN-AUTHORIZED ACTIVE WORK.** Approved six-task sequence on `phase-2/registry-completion`. Current task is selected by `handoff.md` and `workflow/active_work/README.md`; execute sequentially P2C-001 → P2C-006 and never skip dependencies.

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

## Refined execution and acceptance contract (P2C-003)

- Replace edition-derived supported/unsupported capability assertions with **specific nonmutating observations**. Record for each dialect/schema check what was actually attempted, status, and supported/unsupported/unknown semantics.
- Do not treat an HTTP 200, edition label, a 401/403, or an unrecognized endpoint response as proof of query/write functionality. Auth errors, timeouts, malformed/ambiguous data, or untested functionality yield **unknown**.
- Prober remains GET/read-only; never issue test writes or requests to arbitrary user-selected endpoints. Any write-protocol capability without an independently authorized synthetic test is **unknown**, not supported or unsupported.
- Preserve profile-based destination selection, approved credential binding and one-time pinned peer/TLS/SSRF checks on **every** outgoing request; prevent redirects/proxies and bound response size, time and request budget.
- Ensure API wire format, UI three-state mapping and docs agree (unknown must never collapse to unsupported or supported).
- Verify against synthetic isolated InfluxDB 2 OSS / 3 Core; test untrusted/mixed DNS, auth denied, timeouts, redirects, inconclusive responses and edition variation. InfluxDB 1.x is unverified.
- Deliver a capability evidence matrix and exact executed test output; run make check and race tests.

## Stop conditions
Stop and report `TASK_BLOCKED` for unsafe network/credential access, unresolved architecture, or required testing unavailable. No weakening tests or replacing working behavior with mocks.

## Git lifecycle
Human-promoted and authorized as part of the six-task queue. Execute only when designated CURRENT; advance one task at a time. Complete tests, `git mv` to `archive/active-work/`, update aligned CWAL pointers, commit/push and verify remote checkpoint. Human controls PR initiation/review/merge.
