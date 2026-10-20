# P2-006 — Phase 2 mock acceptance, audit and integration handoff

**State:** HUMAN-AUTHORIZED ACTIVE WORK — explicitly approved mock/interface-only Phase 2 batch. ADR-0002 and ADR-0003 remain Proposed; no persistence, live secrets or real historian connections.
**Proposed branch:** `phase-2/database-registry`.
**Sequence:** P2-006 within a potential human-approved P2 work package.

## Objective and bounded scope
Execute end-to-end isolated mock workflows across Go/API/UI interfaces; document open real-integration requirements, ADR dependencies, supported/unsupported capability matrix and security boundaries. Update roadmap only with evidence for implemented features.

## Acceptance criteria
Make check passes; test evidence and limitations recorded; proposed ADRs remain Proposed without human action; reviewed diff contains only authorized scope; archive/commit/push confirmed; report WORK_PACKAGE_READY only for approved mock/contract scope, not live Registry readiness.

## Dependencies and prohibitions
- Read relevant accepted repository architecture and BLACK SHEEP WALL context. Do not infer that ADR-0002 or ADR-0003 is accepted merely because it contains a recommendation.
- Architecture gate: persistent profile storage, real secret storage/key custody and any live credential resolution require separate human acceptance of the relevant ADR. If acceptance has not occurred, deliver only isolated interfaces/mocks/tests and stop at the genuine dependency.
- No live InfluxDB historian reads/writes or Docker/Telegraf control; no credential harvesting, secret display or unsafe TLS bypass. Keep capabilities per connection and mock responses distinguishable from live probes.
- Run acceptance tests and `make check`, record actual evidence and limitations. Upon authorization and verification, `git mv` this task into `archive/active-work/`, update `handoff.md` and Active Work pointers, commit/push and remote-confirm before advancing.
- Never self-promote or create/open/approve/merge a PR. Stop for conflict, an unresolved required architecture decision, unverified checks or a real-system privilege requirement.

## Verification evidence (recorded 2026-10-20)

**Environment:** branch `phase-2/database-registry`; Node.js 24.21.0 / npm 11.19.1; Go 1.27.3 from the official tarball. No dependency versions changed.

**Deliverables:**
- `backend/internal/registrycontract/integration_test.go` — `TestMockRegistryEndToEnd` drives the full interface-only flow (register references, register profiles, probe per edition, enforce selection/capability/role rules, validate an endpoint, confirm resolution is blocked, record an audit event) with no persistence, network or credential value.
- `docs/decisions/phase-2-integration-handoff.md` — what is delivered, the mock capability matrix, the exact blocked real-integration requirements, the verified security boundaries and limitations.
- `docs/ROADMAP.md` — Phase 2 marked at mock/interface scope with an explicit status note that exit criteria are not met for live readiness; remaining items annotated as blocked or partially delivered.
- `ICC/INDEX.md` — route to the integration handoff.

**Commands and actual outcomes:**
- `go test ./internal/registrycontract/ -count=1` → `ok`; the package now contains 37 tests.
- `make check` → exit 0: backend `gofmt -l` clean, `go vet`, `go build`, `go test`; frontend `npm ci`, `eslint` (0 errors, 0 warnings), `tsc -b && vite build`, `vitest run` → `Test Files 13 passed (13)`, `Tests 142 passed (142)`; `policy-check.sh` → PASS.
- `grep -n "Status:" docs/adr/0002-metadata-store-selection.md docs/adr/0003-secret-store-integration.md` → both still `Proposed`.

**Acceptance mapping:**
- Make check passes: exit 0 above.
- Evidence and limitations recorded: this section plus the integration handoff document.
- Proposed ADRs remain Proposed: confirmed unchanged; no ADR was edited to Accepted.
- Reviewed diff contains only authorized scope: changes are the registrycontract package, the registry contract/gate/handoff docs, the roadmap status, the ICC index, the UI registry workflow, and the two directly related Phase 1 shell test repairs.
- Archive/commit/push confirmed: recorded in the package-level report.
- WORK_PACKAGE_READY reported only for mock/contract scope: stated explicitly in the final report, not as live Registry readiness.

**Limitations:** no real integration exists; the handoff document enumerates every blocked item. The capability matrix is a fixture. No live InfluxDB, Docker, Telegraf or historian contact was made, and no credential value was created, stored or resolved at any point.
