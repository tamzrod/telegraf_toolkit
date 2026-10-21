# P2C-007 — Pre-PR credential lifecycle security hardening

> **State: HUMAN-AUTHORIZED ACTIVE WORK** under the standing user decision to route substantial pre-PR audit corrections through Operation CWAL. **Branch:** `phase-2/registry-completion`. **Current task:** P2C-007 only.

## Findings to resolve
1. **HIGH — Replacement validation does not establish credential authorization.** In `backend/internal/registryapi/registryapi.go`, the validation callback accepts an edition-identifying health probe, which may succeed without authenticating the submitted token. Require a read-only protected endpoint/operation proving that the candidate credential authenticates *and has the needed access for the bound profile's role/scope*. An inconclusive, anonymous or permission-denied response must fail closed, preserving the old secret. Never mutate an actual historian. If safe privilege validation cannot be proven with read-only calls, report the limitation and stop rather than claim success.
2. **HIGH — Nonunique replacement staging reference.** `backend/internal/registryservice/service.go` uses `staged-` + the existing reference ID and deletes it unconditionally before each attempt. Concurrent replacements can delete or interfere with another attempt's candidate, or allow racing swaps of different approved candidates. Use collision-resistant per-attempt staging references and serialized/CAS-protected replacement of an unchanged current credential; isolate cleanup and verify failure/retry/restart behavior.
3. **HIGH — Revocation check/use race and destructive detachment.** `RevokeCredential` checks approved bindings outside the subsequent metadata unit of work, then deletes the secret before transactional cleanup and silently detaches profiles still naming the reference. Enforce referential safety and profile dependency policy transactionally. Prefer refuse-and-preserve while *any* profile references or binding uses a credential, not merely while approved bindings exist. Never erase a working secret before safe, durable authorization/metadata transition; handle failures and simultaneous bind/revoke operations. Align docs/UI with actual revoke behavior.

## Acceptance criteria
- [ ] Negative test: a fake/wrong token against a public `/health` endpoint must **never** replace an existing usable credential. Valid candidate must demonstrate actual protected authorization, or replacement fails closed.
- [ ] Concurrent replacements/revoke/enrollment use unique stage IDs and safe atomic transition; tests induce timing races, failures, restart and retries without losing valid material.
- [ ] Revocation of credential referenced by any profile/binding is refused without detaching that profile; concurrent attach/revoke cannot bypass this; standalone unreferenced revocation is idempotent or precisely documented with failure recovery.
- [ ] Preserve all P2L-006/P2L-007 SSRF/DNS/TLS, auth and binding rules. Explicitly test role denials, cross-profile credential handling, secret leakage and audit behavior.
- [ ] Re-run `make check`, `go test -race ./...` in a supported Linux environment, isolated InfluxDB 2 OSS/3 Core authorization and lifecycle tests, and policy/secret checks. Report actual executed versus skipped results, with reasons; do not claim Linux race pass if not run.
- [ ] Update API/Secret Store docs and acceptance evidence; after success archive via `git mv`, clear CWAL queue pointers, commit/push and remote-verify. Report `WORK_PACKAGE_READY`; otherwise report `TASK_BLOCKED`.

## Boundaries
Synthetic isolated credentials and InfluxDB containers only; no production access, destructive historian writes, collector/runtime privileges, new ADR approvals or PR actions. Do not weaken guards to satisfy tests. Execute only this one task.
