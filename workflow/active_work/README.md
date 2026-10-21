# Active Work — Telegraf Toolkit

**Designated branch:** `phase-2/live-registry-implementation`
**Current task:** `P2L-006-pre-pr-probe-security-hardening.md`
**Human-authorized queue:** P2L-006 only.

Security remediation before PR: close live-probe DNS-rebinding / verified-peer gap and unauthorized forwarding of resolved credentials to a caller-chosen endpoint. Preserve production-default SSRF denial and isolated-test-only exceptions. Validate with hostile-unit tests, role/scope/credential-binding tests, actual InfluxDB 2/3 test harness, `make check` and `go test -race ./...`.

Operation CWAL: read the current task, fix, verify, archive by `git mv`, update this README and handoff.md to empty queue, commit/push, confirm remote checkpoint, report WORK_PACKAGE_READY. Stop honestly for blockers. No self-promotion, production access or PR actions.
