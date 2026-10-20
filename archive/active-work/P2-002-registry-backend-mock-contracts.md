# P2-002 — Registry backend domain and isolated persistence-free implementation

**State:** HUMAN-AUTHORIZED ACTIVE WORK — explicitly approved mock/interface-only Phase 2 batch. ADR-0002 and ADR-0003 remain Proposed; no persistence, live secrets or real historian connections.
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

## Verification evidence (recorded 2026-10-20)

**Environment:** branch `phase-2/database-registry`; Node.js 24.21.0 / npm 11.19.1; Go 1.27.3 from the official tarball installed into `$HOME/toolchain`. No package versions changed.

**Deliverables:**
- `backend/internal/registrycontract/repository.go` — typed scope/direction validation (`ValidateScope`, `ValidateProfile`, `ValidateCredentialRef`, `ValidateRoleUsage`) and test-only, in-memory adapters (`MemoryProfileStore`, `MemoryCredentialRefStore`, `MemoryAuditSink`, `MockCapabilityProber`). No persistence, no network, no secret values.
- `backend/internal/registrycontract/repository_test.go` — 12 tests: unique identity, deterministic listing, no-secret profile JSON, secret/credential-URL rejection, complete-scope/reference enforcement, per-kind scope, read/write separation, reference-only storage, audit secret rejection, mock prober labeling.
- `backend/internal/registrycontract/contract.go` — added `invalid-profile`, `scope-incomplete`, `credential-role-mismatch`, `secret-material-rejected` codes; sanitized-error messages reworded to stay marker-clean.
- `docs/contracts/registry.md` — error-code table extended to the implemented set.

**Commands and actual outcomes:**
- `gofmt -l .` → clean; `go vet ./...` → pass; `go test ./internal/registrycontract/ -count=1` → `ok` (20 tests total in the package).
- `make check` → exit 0 (backend gofmt/vet/build/test; frontend `npm ci` + lint + build + `vitest run` → 11 files / 123 tests; `policy-check.sh` → PASS).

**Acceptance mapping:**
- Unique IDs: `TestMemoryProfileStoreEnforcesUniqueIdentity` rejects a duplicate `connection_id` with `duplicate-profile`.
- Explicit scope: `TestValidateScopePerKind` and `TestValidateProfileRequiresCompleteScopeAndReference` require kind-complete scopes.
- Read/write separation: `TestReadWriteSeparationIsEnforced` rejects a write reference for a source and a read reference for a destination.
- No password/token values in profile JSON: `TestProfileJSONContainsNoSecretValues` scans serialized mock profiles; `TestValidateProfileRejectsSecretMaterial` rejects a credential-bearing URL and a secret marker in a profile note.
- Sanitized failures: every returned error is a `*SanitizedError` whose `Validate()` passes (`assertCode`).
- No production persistent registry claimed: adapters are `Memory*` and documented test-only.

**Limitations:** in-memory only; the store is not concurrency-benchmarked and is not a metadata-store decision. Secret detection is a high-signal marker/credential-URL heuristic, not exhaustive. No live InfluxDB, Docker or Telegraf contact; all capabilities remain clearly labeled mock.
