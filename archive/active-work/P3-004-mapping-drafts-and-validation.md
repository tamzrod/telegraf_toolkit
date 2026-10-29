# P3-004 — Editable typed mapping drafts and validation

> **State: ACCEPTED and ARCHIVED (2026-10-28).** Executed under Operation CWAL on `phase-3/canonical-mapper`. Authorized sequence P3-001 → P3-006. This packet did not approve any proposed ADR or unsafe capability expansion.

## Purpose
Implement browser/local draft mapping of measurement, field and tag names, tag values, filters and explicit unit transforms as applicable to confirmed schema types. Validate type compatibility, duplicate destination names, collisions, unit transform declarations, missing fields, stale scopes and unknown capabilities. No write query, deployment or sync.

## Acceptance criteria
No silent coercion; deterministic validation diagnostics; preserving draft while editing; controlled drag/drop with keyboard alternative; preview of mapping plan only unless separately authorized real read-only sample query with proven dialect.

## Verification
Unit/property/UI tests of mapping rules, injection strings, scope invalidation, lint/build and policy.

## Dependency and boundaries
- Dependencies: P3-001 through P3-003.
- Existing Phase 2 Registry is authoritative; registered connection IDs, canonical scope and administrator-bound credential references are used server-side.
- Preserve P2L-006/007, P2C-007/008 SSRF, TLS/pinned peer, credential RBAC, lifecycle fencing and redaction.
- Only synthetic, isolated InfluxDB 2/3 instances; no production access, historian writes, pipeline deployment, Telegraf or sync runtime operations.
- Unknown/unverified is never presented as supported. InfluxDB 1.x not established.
- Any new architecture requiring human approval: record the option and report TASK_BLOCKED; do not self-approve ADR decisions.

## CWAL lifecycle
Execute only CURRENT. On verified acceptance, archive with `git mv` into `archive/active-work/`, update `handoff.md` and Active Work README to designate the next task, commit/push and verify remote SHA. Continue without intermediate approval when within existing approved boundaries. Stop with TASK_BLOCKED when blocked. On P3-006 success clear both pointers and report WORK_PACKAGE_READY. **No agent PR creation, approval or merge.**

---

## Implementation and acceptance evidence (2026-10-28)

**Delivered (UI only, local, no persistence):**
- `ui/src/features/canonical/mappingDraft.ts` — pure draft model: one source and one destination measurement, field rules (destination name, declared type, `none` or explicit unit transform), tag rules, tag-value filters. Immutable edit helpers. Deterministic `validateDraft` returning typed errors and warnings. `planLines` renders names and values as quoted data only.
- `ui/src/features/canonical/MappingDraftEditor.tsx` — editor embedded in the Canonical Mapper page. Reads schema only through the existing GET hooks (`useSchemaMeasurements`, `useSchemaFields`, `useSchemaTags`). Draft lives in component memory and is keyed by source and destination identity, so a profile or scope change discards it. Drag-and-drop of a source field or tag is accepted only for an observed name; every drop has a keyboard `Map field …` / `Map tag …` / `Filter on tag …` button alternative.
- Mock-mode and live-mode both labelled. Unknown, unloaded, stale or failed schema blocks the affected rules and is never treated as support.
- Page action group: "Validate mapping" removed from the disabled list (now implemented locally); "Save mapping" and "Export profile" remain disabled for P3-005.

**Real verification (run on this branch):**
- Baseline before changes: `npx vitest run` 223 passed / 1 skipped; `npm run lint` clean; `npm run build` OK.
- After changes: `npx vitest run` **270 passed / 1 skipped** (25 files). New tests: 37 unit (`mappingDraft.test.ts`), 9 page-level mock-mode (`CanonicalMapperPage.draft.test.tsx`), 1 live GET-only validation (`CanonicalMapperPage.test.tsx`).
- `npx tsc -b` exit 0; `npx eslint .` clean; `npm run build` OK (392.65 kB JS).
- `make check` exit 0 (backend format/check, frontend lint/test/build, `policy-check.sh` PASS with its documented limited regex scope).
- Network/write audit of new production files: no `fetch`, `POST`, `PUT`, `DELETE`, `PATCH`, `localStorage`, `sessionStorage` or `innerHTML` added; only existing `refetch()` GET reads are present.
- Injection strings: destination names such as `x"; DROP MEASUREMENT sensor; --` are refused as invalid identifiers; filter values containing quotes or SQL-shaped text are accepted only as data and rendered as quoted literal text; control characters are refused.

**Acceptance criteria mapping:**
- No silent coercion: `type-mismatch` blocks a type change without a unit transform (unit test and page test).
- Deterministic diagnostics: repeated-validation test with identical output.
- Draft preserved while editing: edits use immutable updates; page test confirms draft survives typing.
- Controlled drag/drop with keyboard alternative: jsdom synthetic drop tests plus button path.
- Preview is a plan only: `planLines` text only; no query, sample read or write is built or issued.
- Scope invalidation: identity-keyed editor plus `stale-scope` diagnostic (unit test); page test confirms draft discarded on profile change.

**Limitations and honest gaps:**
- No property-based tests: no property-testing dependency exists in `ui/package.json`, and none was installed (tooling install not authorized). Coverage is example-based unit and UI tests.
- No real browser or screen-reader pass. Drag/drop and accessibility are verified only in jsdom.
- Live validation was exercised only against the stubbed in-process backend in `CanonicalMapperPage.test.tsx`, not against the real InfluxDB 2/3 containers in this task. No container, sync, deployment or historian write occurred.
- Single source and single destination measurement per draft; multi-measurement drafts are not implemented.
- Draft persistence, versioning, import/export and save/rollback are not implemented (P3-005).
- No sample-point query preview; the approved read path returns metadata only.
- InfluxDB 1.x remains unverified and is shown as unknown.
- No PR was created, approved or merged; no `main` change.

