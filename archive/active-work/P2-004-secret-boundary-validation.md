# P2-004 — Credential-reference security and endpoint-validation boundary

**State:** HUMAN-AUTHORIZED ACTIVE WORK — explicitly approved mock/interface-only Phase 2 batch. ADR-0002 and ADR-0003 remain Proposed; no persistence, live secrets or real historian connections.
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

## Verification evidence (recorded 2026-10-20)

**Environment:** branch `phase-2/database-registry`; Node.js 24.21.0 / npm 11.19.1; Go 1.27.3 from the official tarball. No package versions changed.

**Deliverables:**
- `backend/internal/registrycontract/security.go` — `SecretResolver`/`SecretWriter` interfaces with permanently-blocked `BlockedSecretResolver`/`BlockedSecretWriter` returning sanitized `ErrSecretResolutionBlocked`; `Redactor` (credential-URL + literal redaction); `ThreatModel` (deny-by-default loopback/private/metadata/insecure-TLS with mandatory override reason) and `ValidateEndpointAgainst`; `ValidateProfileInput`; rotation ordering (`RotationSteps`, `ValidRotationOrder`) and a test-only `MemoryRotationScenario`.
- `backend/internal/registrycontract/security_test.go` — 9 tests: permanent resolution block, redaction, deny-by-default classes, override-reason requirement, credential-URL/insecure-scheme rejection, input validation, rotation ordering, rotation reassignment without disturbing unrelated dependents, and no-secret emission.
- `docs/contracts/registry.md` — recorded the enforced secret boundary and rotation ordering rules.

**Commands and actual outcomes:**
- `gofmt -l .` → clean; `go vet ./...` → pass; `go test ./internal/registrycontract/ -count=1` → `ok`, 36 tests passing in the package.
- `make check` → exit 0 (backend gofmt/vet/build/test; frontend `npm ci` + lint + build + `vitest run` → 11 files / 123 tests; `policy-check.sh` → PASS).

**Acceptance mapping:**
- No raw credential value emitted: `BlockedSecretResolver.Resolve` and `BlockedSecretWriter.Store` always return `ErrSecretResolutionBlocked` and never a value (`TestSecretResolutionIsPermanentlyBlocked`); `TestSecuritySurfacesEmitNoSecretValues` scans serialized surfaces.
- Deny localhost/metadata/private: `TestThreatModelDeniesLoopbackPrivateAndMetadata` covers `localhost`, `127.0.0.1`, `10.x`, `192.168.x`, `169.254.169.254` and `metadata.google.internal`.
- Documented overrides: `TestThreatModelRequiresOverrideReason` requires an `OverrideReason` whenever a denied class is permitted.
- Negative tests: credential-bearing URL, insecure scheme and non-HTTP scheme are rejected (`TestThreatModelRejectsCredentialURLAndInsecureScheme`); leaky input is rejected (`TestValidateProfileInputRejectsCredentialURL`).
- No uncontrolled outbound requests: validation is pure classification — no dialer, HTTP client or DNS lookup is used anywhere in the package.
- Stronger storage blocked: the only implementations of the secret boundaries are the blocked stubs; a real store cannot be added without ADR-0003.

**Limitations:** the rotation scenario is in-memory and reference-only; no real credential value exists anywhere. Endpoint validation classifies literal hosts/IPs and does not resolve DNS, so a hostname that resolves to a private address is not caught here (a resolved-address check belongs with a live probe, which is blocked by ADR-0003). Secret detection remains a high-signal heuristic, not exhaustive.
