# P4-007-FU1 — Targeted pre-PR validation and UI safety corrections

**STATE:** AUTHORIZED
**BRANCH:** `phase-4/pipeline-designer`
**CURRENT:** P4-007-FU1
**DEPENDENCY:** P4-001–P4-007 archived, Phase 4 independent pre-PR audit at `853b187716fbb8a1566f2be87d0573eec78ac9bd`.
**SCOPE:** One bounded implementation-and-regression follow-up; no expansion of Phase 4 features.

## Findings and required outcomes
1. **Manual TOML validity (high):** `backend/internal/pipelinecontract/validate.go` currently performs size/secret-pattern checks but no real TOML syntax parse. Reject syntactically malformed manual TOML at least at approval, preferably on save, with a sanitized, actionable diagnostic. Make an explicit distinction between TOML syntax validity and Telegraf 1.40.1 plugin-semantic validation; do not claim unsupported configurations are semantically validated. Add positive/negative tests, including malformed tables, duplicate keys, and valid manual TOML outside the graph catalog. Check behavior at save/approve/export boundaries.
2. **Unsaved UI approval (medium):** `ui/src/features/pipelines/PipelinesPage.tsx` can approve `serverRev` even when the authoritative graph or manual TOML has changed since save. Track saved-document equivalence across graph edits, manual changes and mode switches. Disable or refuse approval when dirty; show a clear message. Test save → edit → attempted approve is blocked, then save → approve succeeds, for both modes where applicable. Do not trust client-side protection alone for server generation/state invariants.
3. **Manual credential safety (medium):** `validateManual` uses heuristic matching. Strengthen bounded structural checking where possible without creating a false promise that arbitrary TOML is secret-free. Explicitly document and test the assurance limits; refuse known inline credential forms and keep credential references opaque. Review approval/export safety statements and error redaction. Do not attempt to implement comprehensive secret scanning with unsupported guarantees.

## Allowed work
Relevant pipeline contract/service/API and pipeline UI files, focused tests, and narrowly required documentation/evidence. Avoid unrelated refactors, new plugins, new discovery features, runtime/deploy work, historian writes or production connections. Preserve prior checkpoints and existing user changes.

## Verification and closure
- Show exact commands and results for targeted Go/UI regression tests.
- Run `make check` and `cd backend && go test -race ./...`; report actual Go/Node versions, skips and unresolved verification limits.
- Archive this task to `archive/active-work/` **only after passing checks**, commit/push normally, verify remote contents, and synchronize `handoff.md` and `workflow/active_work/README.md` to CURRENT/NEXT none, `WORK_PACKAGE_READY`.
- If a substantive architectural decision is needed or a check fails without safe within-scope repair, preserve evidence and report `TASK_BLOCKED`. Never falsely claim readiness.

**CWAL authority:** This follow-up is explicitly human-authorized on the named branch. Execute it autonomously, without intermediate human permission. PR creation, approval and merge remain human-only; do not force-push or modify `main`.
