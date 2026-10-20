# P2-001 — Architecture decision gate and Registry contract inventory

**State:** HUMAN-AUTHORIZED ACTIVE WORK — explicitly approved mock/interface-only Phase 2 batch. ADR-0002 and ADR-0003 remain Proposed; no persistence, live secrets or real historian connections.
**Proposed branch:** `phase-2/database-registry`.
**Sequence:** P2-001 within a potential human-approved P2 work package.

## Objective and bounded scope
Review current docs/contracts/README.md, docs/DATABASE_REGISTRY.md and Proposed ADR-0002/0003; produce a human-readable decision checklist/options and refine interface-only contracts without accepting any ADR. Identify exact tasks that require an accepted metadata/secret design.

## Acceptance criteria
Contract coverage includes profile identity/scope, capabilities, refs, sanitized errors and audit semantics; no proposed ADR silently accepted; mock-only work may proceed but persistent credentials and live secrets are blocked until explicit human acceptance.

## Dependencies and prohibitions
- Read relevant accepted repository architecture and BLACK SHEEP WALL context. Do not infer that ADR-0002 or ADR-0003 is accepted merely because it contains a recommendation.
- Architecture gate: persistent profile storage, real secret storage/key custody and any live credential resolution require separate human acceptance of the relevant ADR. If acceptance has not occurred, deliver only isolated interfaces/mocks/tests and stop at the genuine dependency.
- No live InfluxDB historian reads/writes or Docker/Telegraf control; no credential harvesting, secret display or unsafe TLS bypass. Keep capabilities per connection and mock responses distinguishable from live probes.
- Run acceptance tests and `make check`, record actual evidence and limitations. Upon authorization and verification, `git mv` this task into `archive/active-work/`, update `handoff.md` and Active Work pointers, commit/push and remote-confirm before advancing.
- Never self-promote or create/open/approve/merge a PR. Stop for conflict, an unresolved required architecture decision, unverified checks or a real-system privilege requirement.
