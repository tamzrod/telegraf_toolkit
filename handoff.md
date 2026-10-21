# Telegraf Toolkit — CWAL handoff

## CURRENT OPERATION
**Workflow:** Phase 2 live Registry pre-PR authorization security remediation.
**Designated branch:** `phase-2/live-registry-implementation`.
**Current human-authorized task:** `workflow/active_work/P2L-007-credential-profile-authorization.md`.
**Authorized queue:** P2L-007 only.

## Current Active Task
**P2L-007:** Prevent stored credential exfiltration through malicious profile creation: unprivileged configurators can currently supply an existing `credential_ref_id` and independently chosen endpoint, then trigger probe. Require trusted server-side credential-to-endpoint/scope binding and enforced roles, including negative end-to-end tests.

## Previous checkpoints
P2L-001–P2L-006 archived and remote-confirmed through `e6e90fc9d1f296f3cab39559e1f32a91996276ff`. P2L-006 DNS/peer pinning and direct probe-parameter fixes must be preserved.

## Scope and execution
ADR-0002/0003 Accepted; ADR-0004/0005 Proposed. Only isolated InfluxDB 2/3 test harness, no production access. Execute sole task, add adversarial regression tests, run `make check`, race and live integration checks. Archive by `git mv`, update both queue pointers to empty, commit/push/verify checkpoint and report `WORK_PACKAGE_READY`. Stop with `TASK_BLOCKED` if security acceptance cannot be achieved. No agent PR actions.
