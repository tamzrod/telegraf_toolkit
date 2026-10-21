# P2C-008 — Pre-PR replacement-versus-revocation race hardening

> **State: HUMAN-AUTHORIZED ACTIVE WORK** under standing approval for substantive pre-PR security audit corrections through Operation CWAL.
**Designated branch:** `phase-2/registry-completion`. **Current and only task:** P2C-008.

## Independent audit finding (HIGH)
`registryservice.ReplaceCredential` reads profile/binding/reference and an existing envelope before running network verification. It then swaps the envelope using ciphertext CAS (`ReplaceIfCurrent`) without atomically checking that the credential reference remains active and approved. Concurrent `RevokeCredential` commits reference removal first, then deletes the envelope. The replacement can race into that window, possibly reporting success on a revoked reference, re-creating or leaving unreachable credential material, or interacting with revocation cleanup. The existing CAS only protects against other envelope replacements, not metadata revocation.

## Required fix
- Establish an atomic lifecycle fence/locking scheme for replacement, revocation, binding/rebinding, profile deletion and enrollment. The replacement commit must verify the same active reference, binding and endpoint/scope identity it validated against, under concurrency protection compatible with SQLite transaction behavior. Do not hold a long-lived SQLite writer transaction across network calls; use versioned state, lifecycle lock or equivalent verified design.
- Preserve `VerifyCredential` protected read-only authentication proof, pinned peer TLS/SSRF, administrator-only attachment and write privileges, and unique per-attempt stage material.
- Ensure post-revocation cleanup cannot destroy a successfully committed new credential or leave a credential active on a deleted reference. Explicitly document revocation retry semantics.
- Add deterministic race tests: pause candidate validation; revoke credential in parallel; finish validation; assert no successful replacement of a revoked reference, no reachable/orphan usable secret, and safe retry. Also test replacement vs profile deletion, binding revocation, two replacements and concurrent enrollment. Include failure injection before/after metadata commit and envelope cleanup.
- Run `make check`, `go test -race ./...` on supported Linux, policy/secret checks, isolated synthetic InfluxDB 2/3 lifecycle verification. Record actual outcomes and skipped tests honestly.

## Lifecycle
No production connection, historian writes, collector control or PR action. Stop TASK_BLOCKED for unsatisfied security requirements. On success archive via `git mv`, update handoff and Active Work README to empty queue, commit/push and independently verify HEAD; report WORK_PACKAGE_READY.
