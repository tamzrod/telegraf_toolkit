# P2-002 — Registry backend domain and isolated persistence-free implementation

**State:** PLANNING PROPOSAL — NOT AUTHORIZED. Only human promotion into `workflow/active_work/` enables OpenHands execution.
**Proposed branch:** `phase-2/database-registry`.
**Sequence:** P2-002 within a potential human-approved P2 work package.

## Objective and bounded scope
Add Go typed profile/scope/credential-reference definitions and strict validation with an in-memory test-only repository interface; scoped create/read/update/list semantics can be exposed only via explicitly mock-labeled test handlers. Do not choose actual metadata store or process real credentials.

## Acceptance criteria
Unique IDs, explicit scope, read/write separation, no password/token values in profile JSON, validation tests and sanitized failures; no production persistent registry claimed; make check passes.

## Dependencies and prohibitions
- Read relevant accepted repository architecture and BLACK SHEEP WALL context. Do not infer that ADR-0002 or ADR-0003 is accepted merely because it contains a recommendation.
- Architecture gate: persistent profile storage, real secret storage/key custody and any live credential resolution require separate human acceptance of the relevant ADR. If acceptance has not occurred, deliver only isolated interfaces/mocks/tests and stop at the genuine dependency.
- No live InfluxDB historian reads/writes or Docker/Telegraf control; no credential harvesting, secret display or unsafe TLS bypass. Keep capabilities per connection and mock responses distinguishable from live probes.
- Run acceptance tests and `make check`, record actual evidence and limitations. Upon authorization and verification, `git mv` this task into `archive/active-work/`, update `handoff.md` and Active Work pointers, commit/push and remote-confirm before advancing.
- Never self-promote or create/open/approve/merge a PR. Stop for conflict, an unresolved required architecture decision, unverified checks or a real-system privilege requirement.
