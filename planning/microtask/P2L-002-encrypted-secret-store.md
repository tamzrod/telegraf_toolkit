# Operational secret store, key custody and lifecycle

**State:** PLANNING — not authorized. Human must promote to Active Work.
**Branch:** `phase-2/live-registry`.
**Architecture gate:** ADR-0002 and ADR-0003 must be explicitly human Accepted as applicable before implementation. Neither is accepted by this proposal.

## Objective
After human acceptance of ADR-0003 and its key-custody/rotation plan, implement a backend-only pluggable secret store, authenticated write-only credential entry, encryption/key separation, sanitized audit, rotation and tests proving no secret leaks.

## Acceptance and safety
- Implement actual working behavior, not in-memory or mock substitutes; clearly document coverage and unsupported editions.
- Never connect to production historians or manipulate real credentials without a separately authorized safe test environment and explicit permission. No historian writes, runtime deployments or Docker privilege expansion.
- Require server-side authorization, TLS and DNS/network SSRF protections before live probing. Verify via automated negative and isolated integration tests.
- Preserve exact dependency pins, `make check`, documented evidence and CWAL checkpoints. Human controls PR initiation, review and merge.
