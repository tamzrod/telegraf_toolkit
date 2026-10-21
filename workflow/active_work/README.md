# Active Work — Telegraf Toolkit

**Designated branch:** `phase-2/live-registry-implementation`
**Current task:** none — the authorized queue is empty; `WORK_PACKAGE_READY`.
**Human-authorized queue:** empty.

P2L-007 (credential-to-profile authorization) is complete, verified and archived.
No task remains authorized.

Remediation delivered: attaching a credential reference to a profile now requires
the administrator-only `credential.attach` permission and a server-authoritative
approved binding (credential reference + connection ID + endpoint + canonical
scope). The binding is enforced at profile creation and re-verified at probe
time, so a principal with `profile.write`/`profile.probe` cannot attach another
principal's stored secret to an endpoint it controls. P2L-006 DNS-pinning and
probe-parameter protections are preserved. Validated with adversarial tests
(attacker host receives zero Authorization headers), the isolated InfluxDB 2/3
harness, `make check`, `go test -race ./...` and `scripts/policy-check.sh`.

Operation CWAL: the sole task is fixed, verified, archived by `git mv`, both
pointers set to an empty queue, committed/pushed and remote-confirmed. Report
`WORK_PACKAGE_READY`. No self-promotion, production access or PR actions.
