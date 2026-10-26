# Telegraf Toolkit — Operation CWAL handoff

## CURRENT OPERATION
**Designated branch:** `phase-2/registry-completion`
**Current authorized task:** `workflow/active_work/P2C-008-pre-pr-lifecycle-race.md`
**Authorized queue:** P2C-008 only.

## Status
P2C-001–P2C-007 are complete and archived. P2C-008 remains **ACTIVE / NOT COMPLETE**. The deletion of obsolete `backend/internal/registryservice/race_hardening_test.go` at commit `784f291635b897acea2035fe085625364d09ac46` was a maintenance checkpoint **within** P2C-008, not proof that lifecycle race hardening or CI acceptance has passed. The deleted tests must be replaced with tests against the current service API, not omitted.

## Execution
Switch to this branch if necessary, pull using fast-forward only, and execute Operation CWAL for the sole current task as specified in `workflow/active_work/P2C-008-pre-pr-lifecycle-race.md` and `workflow/active_work/README.md`.

Required scope: close replacement-versus-revocation and binding/profile mutation races, preserve authorization/SSRF/TLS protections, add deterministic concurrency and fault tests against current interfaces, and run supported Linux race/build/check plus isolated synthetic InfluxDB 2/3 tests. Document actual executed results and skipped checks.

If all acceptance criteria pass: archive the task with `git mv`, update both handoff pointers to queue empty, commit/push/verify remote HEAD, and report `WORK_PACKAGE_READY`. Otherwise stop with `TASK_BLOCKED` and evidence.

## Restrictions
No production endpoints, historian writes, unapproved architecture or privilege changes, agent-created PR, or merge. Human retains PR control.
