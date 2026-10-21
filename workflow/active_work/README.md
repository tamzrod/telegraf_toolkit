# Active Work — Telegraf Toolkit

**Designated branch:** `test/real-influxdb-integration`
**Current task:** `TEST-002-repeatable-influxdb-verification.md`
**Human-authorized queue:** TEST-002 only.

Execute the task's two sequential real Docker integration runs on the isolated OpenHands test host, retaining state and credentials between runs; verify safeguards, shell syntax, `make check`, test evidence and no Git-tracked secrets. Make only minimum scoped fixes required by actual failures, and rerun acceptance.

Once passing, archive task via `git mv`, update this README and `handoff.md` to current task none / queue empty, commit and push, confirm remote checkpoint, report `WORK_PACKAGE_READY` and stop. Block honestly if Docker cannot safely run or tests fail. No production access, ADR acceptance, self-promotion or PR actions.
