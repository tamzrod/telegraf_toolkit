# P2L-004 — Real InfluxDB probes with network protections

**State:** HUMAN-AUTHORIZED ACTIVE WORK — execute in the sequence declared in the handoff.
**Branch:** `phase-2/live-registry-implementation`.
**Architecture gate:** ADR-0002 (single-host SQLite) and ADR-0003 (backend-only envelope encryption, separate key custody and rotation) were accepted by the human on 2026-10-21. Respect their exact scopes; ADR-0004/0005 remain Proposed.

## Objective
Implement consented read-only capability probes for tested InfluxDB 1.x/2.x/3.x deployments, using scoped credentials, endpoint allowlisting, DNS resolution and network-peer SSRF restrictions, TLS defaults, bounded timeouts and sanitized responses. Integration tests against explicitly isolated test instances; never infer support by version alone.

## Acceptance and safety
- Implement actual working behavior, not in-memory or mock substitutes; clearly document coverage and unsupported editions.
- Never connect to production historians or manipulate real credentials without a separately authorized safe test environment and explicit permission. No historian writes, runtime deployments or Docker privilege expansion.
- Require server-side authorization, TLS and DNS/network SSRF protections before live probing. Verify via automated negative and isolated integration tests.
- Preserve exact dependency pins, `make check`, documented evidence and CWAL checkpoints. Human controls PR initiation, review and merge.

## CWAL execution instructions

- Execute strictly in queue order P2L-001 → P2L-005; complete and verify the current task, `git mv` it to `archive/active-work/`, update `handoff.md` and `workflow/active_work/README.md`, commit, push and confirm remote checkpoint before advancing without intermediate human approval.
- Do not weaken backend auth or SSRF protections to allow tests. **Only** isolated `testing/influxdb/` instances are authorized; local loopback/private-address exceptions must be confined to explicit test configuration, never enabled by default on production Registry endpoints.
- Preserve existing typed contracts and mock labelling until functionality is truly connected; no mock-only substitution for an acceptance claim.
- Run `make check`, relevant integration checks and policy checks for each task; capture tested features, blocked states, and limitations without committing tokens/database volumes/logs. Stop as `TASK_BLOCKED` for unavailable safety-critical dependencies, not counterfeit success.
- No OpenHands PR create/review/merge actions; final package `WORK_PACKAGE_READY` only when all five tasks have been archived with confirmed checkpoints.

**Specific acceptance:** exercise real read-only authenticated probes against the PR #5 pinned InfluxDB 2 OSS and 3 Core test containers. Explicitly distinguish unsupported/unverified InfluxDB 1.x; no claim of coverage without a real 1.x instance. Enforce destination DNS/IP peer checks, TLS defaults and tight timeout/budget. Do not silently allow remote/private hosts.
