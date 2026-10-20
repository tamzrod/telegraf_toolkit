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

## Verification evidence (recorded 2026-10-20)

**Environment:** branch `phase-2/database-registry`; Node.js 24.21.0 / npm 11.19.1 (pinned, matched CI). Go 1.27.3 was absent from the image and `/usr/local` is not writable for the non-root user, so the official `go.dev` `linux-amd64` tarball was installed into `$HOME/toolchain` (SHA256 `65efbc80…9b40` matched the published digest). No package versions changed.

**Deliverables:**
- `docs/decisions/phase-2-architecture-gate.md` — human-readable decision checklist for ADR-0002 (metadata store) and ADR-0003 (secret store/key custody): options, what each blocks, what is permitted while unresolved, and the exact Phase 2 work blocked by an unaccepted decision. Neither ADR is accepted or marked Accepted.
- `docs/contracts/registry.md` — interface-only refinement of the Database Registry slice (profile identity/scope, probed capabilities, credential references, sanitized errors, audit semantics) separating **Fixed** invariants from **Interface** (provisional) shapes and **Open** items.
- `docs/contracts/README.md` — cross-link to the Phase 2 refinement (inventory unchanged otherwise).
- `ICC/INDEX.md` — semantic route to the new contract/gate documents.
- `backend/internal/registrycontract/contract.go` — declared interfaces (`ProfileStore`, `CapabilityProber`, `CredentialRefStore`, `AuditSink`), `SanitizedError` with enforcement `Validate()`, `CredentialRef` (no value field), `AuditEvent` with secret rejection, and `ValidateEndpoint` (TLS-by-default, SSRF-aware, credential-URL rejection). Reuses the existing `internal/fixtures` profile shape.
- `backend/internal/registrycontract/contract_test.go` — 8 tests covering selection, capability (non-version-inferred), endpoint/TLS/SSRF, sanitized-error leakage, and audit/credential no-secret guarantees.

**Commands and actual outcomes:**
- `make check` → exit 0. Backend `gofmt -l` clean, `go vet`, `go build`, `go test` all pass; frontend `npm ci` + `eslint` + `tsc -b && vite build` + `vitest run` → `Test Files 11 passed (11)`, `Tests 123 passed (123)`; `scripts/policy-check.sh` → `PASS`.
- `go test ./...` → all packages `ok`; `registrycontract` contributes 8 passing tests.

**Acceptance mapping:**
- Profile identity/scope, capabilities, credential references, sanitized errors and audit semantics are each covered and separated into fixed/interface/open in `docs/contracts/registry.md`, with matching declarations and tests.
- No proposed ADR is accepted: ADR-0002/0003 remain **Proposed**; the gate document records them as unresolved and enumerates the blocked work.
- Capabilities are probed per connection and not inferred from version: `TestCheckCapabilitiesReportsUnsupportedWithoutVersionInference` proves a 3.x SQL-only profile fails an InfluxQL requirement.
- No real credential value, live InfluxDB, historian write, Docker or Telegraf control was used; `CredentialRef` has no value-bearing field.

**Limitations:** interfaces and mock adapters only; no persistence, key custody or live probe exists by design. The `SanitizedError`/`AuditEvent` secret markers are high-signal heuristics, not exhaustive secret detection. No live InfluxDB capability was probed, so all capability fixtures remain clearly labeled mock.
