# Phase 2 mock scope and real-integration handoff

Status: **Interface/mock scope only — not a live Registry.**
Task: P2-006 (Phase 2 mock acceptance, audit and integration handoff).

> **Status update (2026-10-21, P2L-001):** ADR-0002 (SQLite metadata store) and
> ADR-0003 (secret store) have since been **accepted** by the human, and P2L-001
> has implemented the ADR-0002 persistence boundary — profiles, credential
> *references* and audit events now persist in SQLite with versioned migrations
> ([`docs/METADATA_STORE.md`](../METADATA_STORE.md)). The tables below describe
> the state at the time of P2-006; the rows that were "blocked by ADR-0002" are
> now unblocked, and secret handling/live probing remain gated on the ADR-0003
> implementation task (P2L-002) and later tasks.

This document records what the Phase 2 mock/interface batch actually delivers, what remains mock, the exact real-integration work that is blocked, the capability matrix used by tests, and the security boundaries. It is the handoff from the approved mock batch to a future, separately authorized real implementation.

- Decision gate: [`docs/decisions/phase-2-architecture-gate.md`](../decisions/phase-2-architecture-gate.md).
- Interface contracts: [`docs/contracts/registry.md`](../contracts/registry.md).
- Parent inventory: [`docs/contracts/README.md`](../contracts/README.md).

## What this batch delivers (implemented and verified)

| Area | Location | Nature |
| --- | --- | --- |
| Interface-only Registry contracts | `docs/contracts/registry.md` | Documentation |
| Decision checklist for ADR-0002/0003 | `docs/decisions/phase-2-architecture-gate.md` | Documentation |
| Declared interfaces + sanitized errors + endpoint validation | `backend/internal/registrycontract/contract.go` | Interface + tests |
| Typed validation and in-memory adapters | `backend/internal/registrycontract/repository.go` | Mock + tests |
| Capability probe surface and edition matrix | `backend/internal/registrycontract/probe.go` | Mock + tests |
| Secret boundary, redaction, threat model, rotation | `backend/internal/registrycontract/security.go` | Interface + tests |
| End-to-end mock flow | `backend/internal/registrycontract/integration_test.go` | Test |
| Typed browser Registry contract | `ui/src/api/registry.ts` | Interface + tests |
| Databases mock workflow | `ui/src/features/databases/` | Mock UI + tests |

Every element above is an interface, a mock, a fixture or a test. **No element persists data, resolves a credential, probes a live endpoint or writes to any historian.**

## Capability matrix (mock expectation fixture)

The matrix in `probe.go` is a **test expectation**, not a validated support statement. It exists to prove capabilities are keyed by edition rather than major version.

| Edition | Major | InfluxQL | Flux | SQL | Schema discovery |
| --- | --- | --- | --- | --- | --- |
| 1.x OSS | 1 | supported | unsupported | unsupported | supported |
| 1.x Enterprise | 1 | supported | unsupported | unsupported | supported |
| 2.x OSS | 2 | supported | supported | unsupported | supported |
| 2.x Cloud | 2 | unknown | supported | unsupported | unknown |
| 3.x Core | 3 | supported | unsupported | supported | unsupported |
| 3.x Cloud Serverless | 3 | unsupported | unsupported | supported | unsupported |

`unknown` is distinct from `unsupported` and is never treated as support. A real support matrix is ADR-E, which is **not yet proposed**.

## Real-integration requirements still blocked

| Requirement | Blocked by | Why it cannot proceed now |
| --- | --- | --- |
| Persistent profile/scope/job/audit storage and migrations | ADR-0002 (Proposed) | Store, schema and migration tooling are unchosen |
| Capability-probe result persistence, freshness/TTL | ADR-0002 (Proposed) | Result storage shape depends on the store |
| Real credential storage, resolution, rotation, revocation | ADR-0003 (Proposed) | Key custody and encryption mechanism are unchosen |
| Runtime secret injection and config delivery | ADR-0003 (Proposed) | Delivery mechanism depends on the secret store |
| Live capability probing against real InfluxDB 1.x/2.x/3.x | ADR-0003 (Proposed) | Requires real scoped credentials |
| A server-side `/api/v1/registry/profiles` endpoint | ADR-0002 (Proposed) | Needs the metadata store and a decided response shape |
| DNS-resolved SSRF checks | ADR-0003 (Proposed) | Requires a live, authenticated probe |

No ADR was accepted, marked Accepted or silently implemented by this batch.

## Security boundaries verified in this batch

- A profile and any reference carry a credential **reference** only; `CredentialRef` has no value-bearing field.
- The only implementations of `SecretResolver`/`SecretWriter` are permanently-blocked stubs.
- Serialized profiles, references, reports and audit events are scanned and rejected if they carry secret-like markers or credential-bearing URLs.
- Endpoint validation is deny-by-default for loopback, private, link-local and metadata targets, requires https unless an explicit exception is recorded, and rejects credential-bearing URLs.
- Capability reports and audit events are always labeled mock.
- Audit events record credential references, never values.

## Verification summary

- `make check` → exit 0: backend `gofmt -l` clean, `go vet`, `go build`, `go test`; frontend `npm ci`, `eslint`, `tsc -b && vite build`, `vitest run`; `policy-check.sh` → PASS.
- Backend `registrycontract` package: 37 tests, including the end-to-end mock flow.
- Frontend: 13 test files, 142 tests.

## Limitations

- Mock and interface scope only; the Registry is **not** a working feature and no live connection exists.
- The capability matrix and edition labels are fixtures, not validated compatibility claims.
- Secret detection is a high-signal marker/URL heuristic, not exhaustive.
- Endpoint validation classifies literal hosts and does not resolve DNS.
- Client-side selection and draft validation are input checks, not authorization.
