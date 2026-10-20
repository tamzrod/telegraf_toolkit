# Telegraf Toolkit — CWAL handoff

## CURRENT OPERATION
**Workflow:** isolated real-database integration testing.
**Designated branch:** `test/real-influxdb-integration`.
**Human-authorized task:** `TEST-001-real-influxdb-containers-and-toolkit-smoke.md` in `workflow/active_work/`.

## Current Active Task
**TEST-001** — deploy isolated Docker Compose InfluxDB 2 OSS + InfluxDB 3 Core test containers, seed/query synthetic data and run honest Toolkit smoke/integration checks. Sole task; no subsequent queue.

## Authority / safety
Human authorized this bounded **test-host-only** container work. No production historians, remote/prod servers, broad Docker privileges or real operational Toolkit Registry changes are authorized. Requires exact version/digest pins, initial secret provisioning before exposure, restricted loopback network and no tracked secrets. If the execution host lacks Docker or safe isolation, stop with actual evidence rather than claiming deployment.

ADR-0002 and ADR-0003 remain Proposed; this task does not accept them. Current Toolkit Registry is mock/interface scope; test and report live integration as NOT IMPLEMENTED if code lacks it, rather than inventing success.

## CWAL checkpoint
Verify results, archive TEST-001 by git mv, record actual test outputs, update pointers, commit/push/remote-confirm and report WORK_PACKAGE_READY only if acceptance achieved. Human controls all PR actions. No auto-promotion.
