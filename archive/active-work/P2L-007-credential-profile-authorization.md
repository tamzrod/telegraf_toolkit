# P2L-007 — Enforce credential-to-profile authorization at profile creation

**State:** HUMAN-AUTHORIZED ACTIVE WORK — **COMPLETE, CHECKPOINTED 2026-10-21** (archived to `archive/active-work/`). Credential-to-profile binding finding fixed with adversarial regression coverage.
**Branch:** `phase-2/live-registry-implementation`
**Authorized queue:** P2L-007 only.

## Independent pre-PR security finding (HIGH)
P2L-006 stopped callers directly overriding the probe endpoint/credential, but profile creation still accepts `endpoint` and arbitrary `credential_ref_id` together. `configurator` has `profile.write` / `credential.read` permissions but not `credential.resolve`; it can enumerate reference IDs, create a profile pointing at its own external endpoint using a different stored credential reference, and invoke `profile.probe` to cause backend credential forwarding. `registryapi/payload.go` accepts the reference, `registryservice.CreateProfileAs` simply stores it, and `handleProbe` resolves it server-side. Prevent this server-side, rather than relying only on the request shape at probe time.

## Implementation and acceptance
1. Design and implement authoritative credential binding to a trusted, approved profile endpoint and scope. Prevent a `profile.write` / `profile.probe` principal without credential-management authority from attaching another profile's existing secret to any attacker-controlled or otherwise unauthorized endpoint. Do not allow privilege escalation by create/delete/recreate, stale refs, or forged scope fields. Bind credential use to the trusted endpoint/scope and enforce again at probe time; do not merely check that a credential-ref ID exists.
2. Preserve secure administrative enrollment/rotation and least-privilege role distinctions. If necessary split untrusted profile creation from privileged credential attachment and require an explicit authenticated administrator binding operation. The design must not depend on browser-side checks or public-host SSRF policy alone.
3. Add adversarial tests demonstrating the real attempted path: `configurator` lists references, creates a profile with another credential ID and attacker-controlled endpoint, calls probe, and receives denial with **zero Authorization headers** delivered to attacker host. Also exercise admin approved association, operator probe of properly approved profile, incorrect scope/origin, deleted/recreated profiles, and sanitized errors.
4. Confirm P2L-006 DNS pinning/redirect/TLS regressions still pass. Run `make check`, `go test -race ./...`, isolated InfluxDB 2/3 integration and Toolkit live-registry harness; document evidence and limitations. Only test-host targets; no production endpoints or actual production credentials.
5. On success archive this task via `git mv`, synchronize handoff and Active Work pointers to empty queue, commit/push and confirm remote SHA; report `WORK_PACKAGE_READY`. Stop with `TASK_BLOCKED` when security acceptance or safe tests cannot be met. No PR actions.

**Scope boundaries:** ADR-0002/0003 stay Accepted; ADR-0004/0005 Proposed. No blanket secret resolution for configurators/operators, no permissive test-policy defaults, no production network access, no unrelated features.

## Verification evidence (recorded 2026-10-21)

**State:** COMPLETE, CHECKPOINTED. The credential-to-profile binding finding is
fixed with adversarial regression coverage; all checks pass.

**Environment:** branch `phase-2/live-registry-implementation`; Go 1.27.3 (official
tarball, SHA-256 verified); Node.js 24.21.0 / npm 11.21.0; Docker Engine 29.9.1 /
Compose v5.6.0 on the isolated test host.

### Fix — server-authoritative credential binding

- New administrator-only permission `credential.attach` (`authz`). A
  `configurator` keeps `profile.write`/`profile.probe` but cannot attach a
  credential reference.
- New approved-binding model (`registrycontract.CredentialBinding`) recording the
  credential reference, connection ID, endpoint and canonical scope, persisted in
  a new `credential_bindings` table (migration v5) with a memory adapter for
  tests.
- `registryservice.CreateProfileAs` refuses a profile carrying a credential
  reference without a matching approved binding. `CreateProfileWithBindingAs`
  records the binding in the same operation (privileged enrollment).
  `ApproveCredentialBinding`/`RevokeCredentialBinding`/`ListCredentialBindings`
  manage bindings. `VerifyProfileCredentialBinding` re-checks the binding at
  probe time.
- `registryapi`: `POST /profiles` requires `credential.attach` when a body carries
  a credential reference (403 otherwise); `GET/POST
  /api/v1/registry/credential-bindings` manage bindings; the probe handler
  re-verifies the binding before resolving any credential. A profile may be
  created unbound.
- A binding must match all of reference, connection ID, endpoint and canonical
  scope, so create/delete/recreate with a changed endpoint or scope, a stale
  reference, or forged scope fields cannot reuse an approval.

### Adversarial tests

- `registryapi/binding_test.go` reproduces the exact path: a configurator
  enumerates the reference, attempts to attach it to an attacker endpoint, and
  probes. Asserts the create is refused (403), the attacker host receives **zero**
  Authorization headers, and the attacker profile is not created. Also covers the
  admin-approved path (operator probe delivers the credential only to the
  profile's own endpoint), the probe-time denial when a binding is missing, scope
  change invalidation, and the binding route being administrator-only.
- `registryapi/registryapi_test.go` updated: a configurator may create an unbound
  profile but is refused when attaching a credential; the admin audit records the
  acting principal.
- `registrycontract` updated: a profile may be unbound; a missing endpoint is
  still rejected.

### Commands and actual outcomes
- `gofmt -l .` → clean; `go vet ./...` and `go vet -tags integration ./...` → pass;
  `go build ./...` → pass.
- `go test ./... -count=1` → all packages `ok`.
- `go test -race ./... -count=1` → all packages `ok`.
- `make check` → exit 0 (backend + frontend 15 files / 152 tests +
  `policy-check.sh` PASS).
- `testing/influxdb/scripts/live-probe.sh` → exit 0 (P2L-006 regressions intact:
  2/2 probes + default-policy rejection).
- `testing/influxdb/scripts/toolkit-live-registry.sh` → exit 0 (**10 checks**):
  anonymous 401; admin create 201; **configurator cannot attach stored credential
  → 403**; **attacker profile not created**; listing; viewer 403; live probe
  `edition=influxdb-2.x-oss` `mock=false`; caller-supplied endpoint rejected 400;
  profile survives restart.
- Secret hygiene: no `.key`/`.db`/token/secret/report/log artifact is tracked;
  credential values in tests are test-only literals.

### Limitations
- Tokens remain static file-provisioned; no issuance/expiry/rotation or OIDC.
- Authorization is role-based; per-connection/per-site refinement is future work.
- InfluxDB 1.x remains unverified. Only Linux amd64 verified. No production
  access, historian write or Docker privilege expansion occurred.
