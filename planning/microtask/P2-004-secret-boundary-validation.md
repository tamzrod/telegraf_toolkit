# P2-004 — Credential-reference security and endpoint-validation boundary

**State:** PLANNING PROPOSAL — NOT AUTHORIZED. Only human promotion into `workflow/active_work/` enables OpenHands execution.
**Proposed branch:** `phase-2/database-registry`.
**Sequence:** P2-004 within a potential human-approved P2 work package.

## Objective and bounded scope
Model server-side secret references only and protected operation interfaces; add input validation, TLS policy expectations, SSRF-aware endpoint validation and test-only credential rotation scenario where safe. Do not store/resolve actual credential values or implement encryption-key custody before ADR-0003 is accepted.

## Acceptance criteria
No raw credential value emitted to UI, tests or logs; deny localhost/metadata/private destinations per declared threat model unless explicitly approved, documented overrides, negative tests, no uncontrolled outbound requests; stronger credential storage implementation blocked on human ADR decision.

## Dependencies and prohibitions
- Read relevant accepted repository architecture and BLACK SHEEP WALL context. Do not infer that ADR-0002 or ADR-0003 is accepted merely because it contains a recommendation.
- Architecture gate: persistent profile storage, real secret storage/key custody and any live credential resolution require separate human acceptance of the relevant ADR. If acceptance has not occurred, deliver only isolated interfaces/mocks/tests and stop at the genuine dependency.
- No live InfluxDB historian reads/writes or Docker/Telegraf control; no credential harvesting, secret display or unsafe TLS bypass. Keep capabilities per connection and mock responses distinguishable from live probes.
- Run acceptance tests and `make check`, record actual evidence and limitations. Upon authorization and verification, `git mv` this task into `archive/active-work/`, update `handoff.md` and Active Work pointers, commit/push and remote-confirm before advancing.
- Never self-promote or create/open/approve/merge a PR. Stop for conflict, an unresolved required architecture decision, unverified checks or a real-system privilege requirement.
