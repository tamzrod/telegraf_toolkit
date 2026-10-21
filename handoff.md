# Telegraf Toolkit — CWAL handoff

## CURRENT OPERATION
**Workflow:** Phase 2 live Registry pre-PR security remediation.
**Designated branch:** `phase-2/live-registry-implementation`.
**Current human-authorized task:** `workflow/active_work/P2L-006-pre-pr-probe-security-hardening.md`.
**Authorized queue:** P2L-006 only.

## Current Active Task
**P2L-006** — Resolve HIGH-priority pre-PR vulnerabilities: DNS validation versus actual TCP peer (rebinding) and attacker-selected endpoint forwarding of server-resolved credentials. Add negative security and isolated InfluxDB 2/3 live regression tests.

## Previous checkpoints
P2L-001 through P2L-005 were archived and pushed through `dc15dccf3ce2df0645de90db5488b4d731782041`. Their tests are not a security signoff for the newly discovered risks.

## Scope
ADR-0002/0003 Accepted for single-host SQLite/envelope credentials. ADR-0004/0005 Proposed. No production access, uncontrolled network probes or credential disclosure. Only isolated testing/influxdb test environment may use test loopback policy. PR creation, review and merge human-only.

## Execution
Execute the sole P2L-006 task, verify the actual fixes and regression suite, archive via `git mv`, advance both pointers to empty queue, commit/push and verify remote checkpoint. Stop and report TASK_BLOCKED if a required security or live-test condition cannot be met. Report WORK_PACKAGE_READY only on verified completion; no auto-promotion.
