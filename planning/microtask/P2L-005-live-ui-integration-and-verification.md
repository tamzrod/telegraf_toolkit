# Live Registry UI, integration testing and security gate

**State:** PLANNING — not authorized. Human must promote to Active Work.
**Branch:** `phase-2/live-registry`.
**Architecture gate:** ADR-0002 and ADR-0003 must be explicitly human Accepted as applicable before implementation. Neither is accepted by this proposal.

## Objective
Wire typed client to authenticated endpoints, distinguish actual health from unknown/unsupported, write-only secrets and rotation UI; verify real persistence, restart, logs/exports redaction, negative auth tests, integration tests, and make check. End with CWAL WORK_PACKAGE_READY for human PR audit, no agent PR.

## Acceptance and safety
- Implement actual working behavior, not in-memory or mock substitutes; clearly document coverage and unsupported editions.
- Never connect to production historians or manipulate real credentials without a separately authorized safe test environment and explicit permission. No historian writes, runtime deployments or Docker privilege expansion.
- Require server-side authorization, TLS and DNS/network SSRF protections before live probing. Verify via automated negative and isolated integration tests.
- Preserve exact dependency pins, `make check`, documented evidence and CWAL checkpoints. Human controls PR initiation, review and merge.
