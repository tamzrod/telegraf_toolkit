# P2L-003 — Authenticated Registry API and profile management

**State:** HUMAN-AUTHORIZED ACTIVE WORK — **COMPLETE, CHECKPOINTED 2026-10-21** (archived to `archive/active-work/`). Authorized Registry API implemented.
**Branch:** `phase-2/live-registry-implementation`.
**Architecture gate:** ADR-0002 (single-host SQLite) and ADR-0003 (backend-only envelope encryption, separate key custody and rotation) were accepted by the human on 2026-10-21. Respect their exact scopes; ADR-0004/0005 remain Proposed.

## Objective
Implement versioned registry profile CRUD and credential reference workflows behind actual server-side authorization and input validation; ensure UI is not security boundary, least privilege, sanitized errors, explicit scopes and audited mutations. No permissive anonymous admin endpoints.

## Acceptance and safety
- Implement actual working behavior, not in-memory or mock substitutes; clearly document coverage and unsupported editions.
- Never connect to production historians or manipulate real credentials without a separately authorized safe test environment and explicit permission. No historian writes, runtime deployments or Docker privilege expansion.
- Require server-side authorization, TLS and DNS/network SSRF protections before live probing. Verify via automated negative and isolated integration tests.
- Preserve exact dependency pins, `make check`, documented evidence and CWAL checkpoints. Human controls PR initiation, review and merge.

## CWAL execution instructions

- Execute strictly in queue order P2L-001 → P2L-005; complete and verify the current task, `git mv` it to `archive/active-work/`, update `handoff.md` and `workflow/active_work/README.md`, commit, push and confirm remote checkpoint before advancing without intermediate human approval.
- Do not weaken backend auth or SSRF protections to allow tests. **Only** isolated `testing/influxdb/` instances are authorized; local loopback/private-address exceptions must be confined to explicit test configuration, never enabled by default on production Registry endpoints.
- Preserve existing typed contracts and mock labelling until functionality is truly connected; no mock-only substitution for an acceptance claim.
- Run `make check`, relevant integration checks and policy checks for each task; capture tested features, blocked states, and limitations without committing tokens/database volumes/logs. Stop as `TASK_BLOCKED` for unavailable safety-critical dependencies, not counterfeit success.
- No OpenHands PR create/review/merge actions; final package `WORK_PACKAGE_READY` only when all five tasks have been archived with confirmed checkpoints.

**Specific acceptance:** backend-enforced authentication/authorization with explicit scopes, validated CRUD, actual persistence, secret references only in responses, audited mutations, denial tests; no anonymously writable endpoints or browser-only access controls.

## Verification evidence (recorded 2026-10-21)

**Environment:** branch `phase-2/live-registry-implementation`; Go 1.27.3. No new
dependency (standard library only).

**Deliverables:**
- `backend/internal/authz/` — role/permission model (viewer, operator,
  configurator, administrator), deny-by-default `Authorizer`, token
  `Authenticator` and fail-closed `LoadTokenFile` (owner-only, non-empty, valid
  roles).
- `backend/internal/registryapi/` — authorized HTTP surface: authenticated
  profile CRUD, credential-reference listing and audit listing, strict body
  decoding with secret-like-key rejection, sanitized error mapping and payload
  shapes that carry references only.
- `backend/internal/httpapi/handler.go` — registers Registry data routes only
  when an authenticator is supplied.
- `backend/internal/registryservice/service.go` — actor-attributed audit for
  mutations (`CreateProfileAs`, `DeleteProfileAs`).
- `backend/cmd/api/main.go` — `-token-file` flag; without a token file the data
  routes are not registered.
- `docs/REGISTRY_API.md` — auth, roles, routes, validation, coverage, limits.

**Commands and actual outcomes:**
- `gofmt -l .` → clean; `go vet ./...` → pass; `go build ./...` → pass.
- `go test ./... -count=1` → all packages `ok`.
- `make check` → exit 0 (backend + frontend + policy).
- Process-level: anonymous `GET /profiles` → **401**; viewer `POST /profiles` →
  **403**; administrator `POST /profiles` → **201**; viewer `GET /profiles` →
  **200**; with no token file the routes are absent.

**Acceptance mapping:**
- Backend-enforced auth with explicit scopes → `authz` role matrix tests and
  `registryapi` 401/403 tests.
- Validated CRUD, actual persistence → API create/list/get/delete tests over a
  real SQLite store; contract validation rejects incomplete scope, missing
  reference and secret-like bodies.
- Secret references only in responses → `TestCreateProfileResponseCarriesReferenceNotValue`,
  `TestCredentialsRouteNeverReturnsValue`.
- Audited mutations → `TestAdminCanReadAudit` records actor `alice`.
- Denial tests → anonymous/invalid-token/viewer-write/viewer-credentials/viewer-audit
  cases; `TestRegistryDataRoutesRequireAuthentication` covers the wired handler.
- No anonymously writable endpoints → data routes are registered only with an
  authenticator; anonymous requests are 401.

**Limitations:** static file-provisioned tokens with no issuance/expiry/rotation
or OIDC/OAuth; role-based authorization only (no per-connection/site refinement);
TLS termination assumed upstream (packaging is ADR-0005, Proposed). No historian
read/write, Docker control or live probe occurred; no production credential used.
