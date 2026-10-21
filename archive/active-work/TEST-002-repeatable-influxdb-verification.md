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

## Verification evidence (recorded 2026-10-21)

**Environment:** branch `test/real-influxdb-integration` at `06f8a963` (matches
`origin`); clean working tree. Docker Engine 29.8.1 / Compose v5.5.1 on the
isolated OpenHands test host; daemon started locally, scripts run with
`DOCKER_SUDO=1`. Go 1.27.3 from the official tarball. Pinned images unchanged:
`influxdb:2.9.1@sha256:cdd3dbde…716eaaac` and
`influxdb:3.12.1-core@sha256:1c8200f7…ab6bd406`. Credentials/data/reports remain
Git-ignored.

**Two consecutive full runs against retained state (the credential-reuse fix):**

- Run 1 — fresh disposable state, `./scripts/run-all.sh --teardown` → exit 0;
  InfluxDB **16 passed / 0 failed**; Toolkit smoke **2 passed / 0 failed /
  1 not implemented**; credentials created. Report
  `reports/integration-20261021T014405Z.txt`.
- Run 2 — same retained data and credentials (no regeneration) → exit 0;
  InfluxDB **16 passed / 0 failed**; Toolkit smoke **2 passed / 0 failed /
  1 not implemented**. The run logged `Reusing existing test credentials
  (values not displayed).` at both the credential step and `up.sh`. Report
  `reports/integration-20261021T014543Z.txt`.
- Idempotence: the seeded counts stayed stable across runs —
  `influxdb2 Flux count(environment.used_percent)==5` and
  `influxdb3 SQL count(environment)==5` passed in **both** runs (no duplicate
  accumulation); the second InfluxDB 3 database create returned HTTP 409
  (already exists) as expected.

**Negative credential-safety (retained data, disposable copies):**

- Partial set (removed the InfluxDB 3 token) with retained data → `up.sh` exits
  `2` with `BLOCKED: incomplete test credential set…`; `run-all.sh` also exits
  `2` and the **remaining credentials were unchanged** (sha256 identical, no
  overwrite).
- Missing set (all four files absent) with retained data → `run-all.sh` exits
  `2` with `BLOCKED: test data exists but credentials are missing…`, so it does
  not silently regenerate credentials against existing data.
- Credentials were then restored and verified byte-identical (sha256 match).

**Shared guard:** `ensure_test_credentials` (in `lib.sh`) is invoked by both
`up.sh` and `run-all.sh`, confirming direct startup and the full runner share
one guard.

**Other checks:**

- `bash -n` on all seven scripts in `testing/influxdb/scripts/` → all OK.
- `make check` → exit 0 (backend gofmt/vet/build/test; frontend npm ci/eslint/
  tsc/vite/vitest; `policy-check.sh` PASS).
- Secret hygiene: no token/password patterns in `reports/`, `logs/` or `tmp/`;
  `git status` clean and `git add -An .` stages nothing (no reports, data or
  credentials tracked).

**Result:** the credential-reuse fix is verified. Both consecutive runs pass the
original 16 InfluxDB checks and the Toolkit smoke split unchanged; negative
credential cases fail safely without overwriting or regenerating credentials.

**Limitations:** this is verification of the test harness only. No operational
Registry implementation, no production access and no ADR acceptance occurred;
ADR-0002 and ADR-0003 remain Proposed. The redactor is a targeted pattern match,
not a guarantee.

## Boundaries
Only local isolated Docker test containers for official pinned InfluxDB 2/3. No production historian/network, no real Toolkit operational credential store, no ADR acceptance, no unapproved task promotion, no PR creation/review/merge. Keep reports, credentials and test data Git-ignored. Human retains all PR authority.
