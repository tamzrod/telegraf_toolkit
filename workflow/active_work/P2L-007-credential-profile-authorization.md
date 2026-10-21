# P2L-007 — Enforce credential-to-profile authorization at profile creation

**State:** HUMAN-AUTHORIZED ACTIVE WORK under standing instruction: substantial security audit corrections via Operation CWAL.
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
