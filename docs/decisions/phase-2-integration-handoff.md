# Phase 2 mock scope and real-integration handoff

Status: **Historical batch record — superseded by the live Registry merged in PR #6.**
Task: P2-006 (Phase 2 mock acceptance, audit and integration handoff).

> **Status update (2026-10-21, P2L-001–P2L-007):** ADR-0002 (SQLite metadata
> store) and ADR-0003 (secret store) were **accepted** by the human and are now
> **implemented**. The operational live Registry was built through authorized CWAL
> tasks P2L-001–P2L-007 and **merged into `main` by PR #6 on 2026-10-21** (merge
> commit `0ba4827`): persistent metadata store, envelope-encrypted secret store,
> server-authorized Registry API, read-only live capability probing with
> pinned-peer/SSRF controls (P2L-006) and administrator-only credential
> attachment with an approved binding (P2L-007). The tables below describe the
> state at the time of P2-006 and are kept as the historical handoff; the current
> delivery status is tracked in [`docs/ROADMAP.md`](../ROADMAP.md).

This document records what the Phase 2 mock/interface batch actually delivered, what remained mock, the real-integration work that was blocked at the time, the capability matrix used by tests, and the security boundaries. It is the handoff from the approved mock batch to the separately authorized real implementation that followed.

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

Every element above is an interface, a mock, a fixture or a test. **No element persists data, resolves a credential, probes a live endpoint or writes to any historian.** That statement describes the P2-006 batch only; the operational implementation that followed it is listed in the status update above and in [`docs/ROADMAP.md`](../ROADMAP.md).

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

## Real-integration requirements: status after PR #6

| Requirement | Status now | Where |
| --- | --- | --- |
| Persistent profile/scope/audit storage and migrations | Delivered (P2L-001) | [`docs/METADATA_STORE.md`](../METADATA_STORE.md) |
| Capability-probe result persistence and freshness/TTL | Partially delivered — probe results are returned live, but no probe-result persistence or TTL policy exists | [`docs/LIVE_PROBING.md`](../LIVE_PROBING.md) |
| Real credential storage, resolution, rotation, revocation | Delivered (P2L-002) for server-side store/resolve/rotate; revocation, browser flows and runtime injection remain future work | [`docs/SECRET_STORE.md`](../SECRET_STORE.md) |
| Runtime secret injection and config delivery | Not implemented — needs the host-agent privilege boundary (ADR-0004, **Proposed**) | [`docs/adr/0004-host-agent-privilege-boundary.md`](../adr/0004-host-agent-privilege-boundary.md) |
| Live capability probing against real InfluxDB 1.x/2.x/3.x | Delivered for 2.x OSS and 3.x Core (P2L-004/P2L-006); **InfluxDB 1.x is unverified** | [`docs/LIVE_PROBING.md`](../LIVE_PROBING.md) |
| A server-side `/api/v1/registry/profiles` endpoint | Delivered (P2L-003/P2L-007) | [`docs/REGISTRY_API.md`](../REGISTRY_API.md) |
| DNS-resolved SSRF checks | Delivered (P2L-004/P2L-006) — pinned peer validation, no proxy, redirects refused | [`docs/LIVE_PROBING.md`](../LIVE_PROBING.md) |

No ADR was accepted, marked Accepted or silently implemented by the P2-006 batch.
ADR-0002 and ADR-0003 were accepted separately by the human on 2026-10-21.

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

## Limitations at the time of P2-006 (historical)

- Mock and interface scope only; the Registry was **not** a working feature and no
  live connection existed.
- The capability matrix and edition labels were fixtures, not validated
  compatibility claims.
- Secret detection is a high-signal marker/URL heuristic, not exhaustive.
- Endpoint validation classified literal hosts and did not resolve DNS.
- Client-side selection and draft validation are input checks, not authorization.

## Current limitations (after PR #6)

- The browser ships live listing and read-only probing only; profile writes and
  credential entry/revocation in the browser remain gated.
- Authentication is **static file-provisioned bearer tokens**: no issuance, expiry,
  rotation or OIDC/OAuth.
- Capability outcomes are projected from the **detected edition**; per-capability
  evidence and a write proof are not implemented.
- **InfluxDB 1.x is unverified** (no isolated 1.x instance).
- Runtime secret injection, host-agent control and packaging remain undecided
  (ADR-0004/ADR-0005, **Proposed**).
