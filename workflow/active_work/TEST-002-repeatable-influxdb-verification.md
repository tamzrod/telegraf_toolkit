# TEST-002 — Verify repeatable real InfluxDB test runs

**Status:** HUMAN-AUTHORIZED CURRENT TASK
**Branch:** `test/real-influxdb-integration`
**Scope:** verification only; no operational Registry implementation, production access, or PR actions.

## Objective
On the authorized isolated Docker/OpenHands test host, validate the credential-reuse fix in commit `8efc3ae8f392469a9494b8a7c1377e560628a8c1` by running two consecutive full InfluxDB 2 OSS / InfluxDB 3 Core test cycles **against the same retained database state**. Confirm the original tests still pass after the shared guard change.

## Procedure
1. Verify checkout is the designated branch and no unrelated worktree edits exist; preserve any pre-existing edits. Verify Docker/Compose availability and local isolated test-host scope, image digest pins and Git-ignored credential/data directories.
2. On fresh disposable TEST-ONLY state, run `cd testing/influxdb && ./scripts/run-all.sh --teardown` (use `DOCKER_SUDO=1` and `GO_BIN` only if required by host). Do not regenerate credentials between runs; `--teardown` intentionally retains data and secrets.
3. Run `./scripts/run-all.sh --teardown` again against the retained data and credentials. Both executions must exit zero and report all 16 real InfluxDB checks passed, zero failed, and Toolkit smoke 2 passed / 0 failed / 1 explicitly NOT IMPLEMENTED. Verify the second run actually logs reuse of existing credentials. Record sanitized run evidence in the archived task; do not commit secrets, local reports or runtime data.
4. Check idempotence of the 16 cases, especially record count after repeated seeding. If the second run fails, diagnose and make the **minimum scoped fix** on this branch, re-run both consecutively from an isolated reset and verify. Do not falsify pass counts, weaken assertions, or change the test semantics to hide a failure.
5. Verify a **missing or partial credential set with retained data** exits nonzero and does not overwrite remaining credentials (use a safe disposable copy/sandbox or safely restore files without affecting test state). Validate startup `up.sh` and runner `run-all.sh` share the same guard. Confirm shell syntax (`bash -n` on all scripts), run `make check`, policy check, and Git secret hygiene inspection. Document any inability to run a check explicitly.
6. On success, `git mv` this task to `archive/active-work/`, update `handoff.md` and `workflow/active_work/README.md` to empty queue, commit, push, verify remote HEAD matches local, and report `WORK_PACKAGE_READY`. Stop, with honest evidence, if safe Docker execution cannot be achieved or verification remains failing.

## Boundaries
Only local isolated Docker test containers for official pinned InfluxDB 2/3. No production historian/network, no real Toolkit operational credential store, no ADR acceptance, no unapproved task promotion, no PR creation/review/merge. Keep reports, credentials and test data Git-ignored. Human retains all PR authority.
