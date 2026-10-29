# P3-003 — Live dual-profile selection and typed schema browser

> **State: HUMAN-AUTHORIZED ACTIVE WORK — COMPLETE, VERIFIED 2026-10-26** (archived to `archive/active-work/`). The Canonical Mapper now selects an explicit distinct source and destination and browses their measurements, field keys and tag keys through the authorized P3-002 read path. It does not create mapping drafts, canonical schemas, exports or runtime operations; InfluxDB 1.x remains unverified. This packet does not approve any proposed ADR or unsafe capability expansion.

## Purpose
Create a genuine Live-mode Canonical Mapper UI for explicit distinct source and destination Registry profiles with scope. Show their measurement, field-key and tag-key metadata in separate typed groups backed by P3-002. Support loading, empty, permission, unknown, unsupported and stale states without mock fallback. Preserve labeled Mock mode.

## Acceptance criteria
Ensure source/destination distinct; invalidate cache and dependent drafts whenever selected profile/scope/identity changes; never expose secret/ref values beyond safe identity; no implicit writes; keyboard navigation, accessible lists/errors, race-safe request cancel.

## Verification
Vitest/typed API tests, lint/typecheck/build, authenticated real-backend smoke and manual a11y evidence or explicit gap.

## Dependency and boundaries
- Dependencies: P3-001 through P3-002.
- Existing Phase 2 Registry is authoritative; registered connection IDs, canonical scope and administrator-bound credential references are used server-side.
- Preserve P2L-006/007, P2C-007/008 SSRF, TLS/pinned peer, credential RBAC, lifecycle fencing and redaction.
- Only synthetic, isolated InfluxDB 2/3 instances; no production access, historian writes, pipeline deployment, Telegraf or sync runtime operations.
- Unknown/unverified is never presented as supported. InfluxDB 1.x not established.
- Any new architecture requiring human approval: record the option and report TASK_BLOCKED; do not self-approve ADR decisions.

## CWAL lifecycle
Execute only CURRENT. On verified acceptance, archive with `git mv` into `archive/active-work/`, update `handoff.md` and Active Work README to designate the next task, commit/push and verify remote SHA. Continue without intermediate approval when within existing approved boundaries. Stop with TASK_BLOCKED when blocked. On P3-006 success clear both pointers and report WORK_PACKAGE_READY. **No agent PR creation, approval or merge.**

## Resolution

P3-003 is complete and verified. It delivered the read-only mapper increment and
did not start P3-004 or any other later task. The page consumes the P3-002
routes; it adds no backend route, no query passthrough and no write.

1. **Explicit distinct selection.** Two native selects choose the source and
   destination from the active profile list (mock or live). The same profile on
   both sides is refused before any schema request; a selected ID that has
   disappeared from the list is reported separately.
2. **Typed schema groups.** Each side shows measurements, field keys (with the
   reported type) and tag keys as separate labelled groups. A field/tag read is
   issued only after a measurement observed on that side is selected. The
   browser sends only the stored `connection_id` and that measurement.
3. **Honest states.** Loading, positively-supported empty, 401/403, not-found,
   unknown, unsupported, truncated and stale are distinct. A failed or unknown
   read never renders as an empty schema or as support, and live failures never
   fall back to the mock preview.
4. **Cache and race safety.** Query keys include connection ID, canonical scope
   and (for detail reads) the measurement. The panel key includes connection ID
   plus scope. A selection, token or mode change cancels and removes the affected
   schema queries; an in-flight answer cannot render into the new selection.
5. **Safe identity and no writes.** The page renders display name, connection
   ID, family/major version, canonical scope and observed edition. It renders no
   endpoint or credential reference and receives no credential value. The token
   is memory-only and cleared from the input after application. Every browser
   request is a GET.
6. **Labeled Mock mode.** The default mode uses the existing typed mock profile
   path plus a feature-local synthetic preview. The preview is always marked
   `mock`, reports no request, shows a 1.x profile as Unknown, honors an
   unsupported schema-discovery capability, and is never imported by a live path.
7. **Accessibility and keyboard.** Native selects/buttons with visible focus,
   `aria-pressed` on mode and measurement controls, labelled lists and
   `alert`/`status` regions; tests cover keyboard selection, one `h1`, heading
   order and unique IDs.

### Acceptance criteria → evidence

| Criterion | Implemented behaviour | Evidence |
| --- | --- | --- |
| Source/destination distinct | Both sides required; same connection and scope refused; missing profile reported. | `canonicalWorkflow.test.ts` selection tests; `CanonicalMapperPage.test.tsx` distinctness test |
| Cache invalidation on profile/scope/identity change | Keys and panel identity include connection + canonical scope (+ measurement); token/mode/selection changes cancel and remove schema entries. | `canonicalWorkflow.test.ts` freshness tests; `CanonicalMapperPage.test.tsx` stale and cancellation tests |
| No secret/ref exposure beyond safe identity | No endpoint or credential reference rendered; token input cleared; secret-like payload keys rejected by the client guard. | `CanonicalMapperPage.test.tsx` live identity test; `schemaClient.test.ts` secret-like-key test; typed guard |
| No implicit writes | Only three GET client functions; no POST/query/DDL path; all calls asserted GET and bodyless. | `schemaClient.test.ts`; `CanonicalMapperPage.test.tsx` GET-only assertion |
| Keyboard navigation and accessible lists/errors | Native controls, `aria-pressed`, labelled measurement/tag lists, alert/status regions; keyboard Enter test; heading/ID checks. | `CanonicalMapperPage.test.tsx` accessibility test; `moduleEntryPoints.test.tsx`; `App.test.tsx`; `accessibility.test.tsx` |
| Race-safe request cancel | Queries receive `AbortSignal`; selection change cancels the previous connection's reads; delayed fields test asserts the signal is aborted and old data is absent. | `schemaClient.test.ts` signal test; `CanonicalMapperPage.test.tsx` cancellation test |
| Loading/empty/permission/unknown/unsupported/stale without mock fallback | Distinct states; 1.x unknown; 3.x mock capability unsupported; live failure asserts no mock preview. | `CanonicalMapperPage.test.tsx` state tests; `canonicalWorkflow.test.ts`; `mockSchemaPreview` tests |
| Typed API tests | Wire types and runtime guard; GET-only client; fixed query parameters. | `schemaClient.test.ts` (7 tests) |
| Real-backend smoke | Authenticated API + isolated InfluxDB 2 OSS, then the typed client against the same API. | `testing/influxdb/scripts/canonical-mapper-smoke.sh` → 6/6 PASS; `schema.live.integration.test.ts` |
| Manual a11y evidence or explicit gap | No browser engine/screen reader was driven; jsdom component/accessibility tests are recorded. | Explicit gap below |

### Changed surfaces

- `ui/src/api/registry.ts` — schema wire types, runtime guard, safe field/observation validation
- `ui/src/api/client.ts` — three GET schema client functions with `AbortSignal`
- `ui/src/api/queryKeys.ts` — schema prefix and identity/scope/measurement keys
- `ui/src/hooks/useSchemaBrowser.ts` — measurements/fields/tags query hooks
- `ui/src/features/canonical/canonicalWorkflow.ts` — selection, freshness, failure and outcome logic
- `ui/src/features/canonical/mockSchemaPreview.ts` — labeled synthetic preview
- `ui/src/features/canonical/CanonicalMapperPage.tsx` — live/mock browser, states, cache cancellation and a11y
- `ui/src/app/modules.ts` — Canonical Mapper is now `partial`
- `ui/src/features/moduleEntryPoints.test.tsx`, `ui/src/app/App.test.tsx` — updated entry-point/network boundaries
- `ui/src/features/canonical/CanonicalMapperPage.test.tsx`, `canonicalWorkflow.test.ts`, `ui/src/api/schemaClient.test.ts` — new tests
- `ui/src/api/mock/schema.ts` — test-only typed live fixtures
- `ui/src/api/schema.live.integration.test.ts` — gated authenticated real-backend client smoke
- `testing/influxdb/scripts/canonical-mapper-smoke.sh`, `testing/influxdb/README.md` — smoke harness
- `docs/LIVE_UI.md`, `docs/CANONICAL_MAPPING.md`, `docs/ROADMAP.md`, `docs/contracts/canonical-mapper.md`, `docs/contracts/README.md`, `README.md`, `ICC/context/project.md`
- `handoff.md`, `workflow/active_work/README.md` — pointer advance after archive

### Verification (executed 2026-10-26)

| Check | Command | Result |
| --- | --- | --- |
| Lint | `cd ui && npx eslint .` | pass |
| Typecheck | `cd ui && npx tsc -b` | pass |
| Build | `cd ui && npm run build` | pass (`vite build`, 95 modules) |
| Full UI suite | `cd ui && npx vitest run` | 23 files passed, 1 skipped; **223/223 active tests passed**, 1 gated test skipped (224 total) |
| Full repository gate | `make check` | exit 0 (backend format/vet/build/tests, UI lint/build/test, policy check) |
| Policy / secret scan | `bash scripts/policy-check.sh` | PASS (limited regex checks only) |
| Relative links | scripted check of touched docs | 93 links, 0 broken |
| Authenticated real-backend smoke | `./testing/influxdb/scripts/canonical-mapper-smoke.sh` | **6/6 PASS**; measurements `environment`+`sensor`, field `value`, tags `host`+`sensor_id`, `outcome=supported`, `mock=false`; typed client passed against the same API |
| Live containers | Isolated `ttk-influxdb2` (InfluxDB 2 OSS v2.9.1), synthetic data | used read-only for the smoke; kept running; no historian write |

The smoke created a throwaway backend token/key and a profile bound to the
isolated bucket under `testing/influxdb/tmp/` (Git-ignored). It seeded a
server-side credential for that profile; the credential value was not printed or
passed in argv. Every deployment request in the read path was a GET. No
deployment, Telegraf runtime action, synchronization or historian write occurred.

### Manual accessibility evidence — explicit gap

No real browser or screen reader was available in this environment. The
recorded checks are jsdom tests only: keyboard activation of a measurement,
native select/button semantics, one `h1`, no heading-level skips, unique IDs and
labelled `alert`/`status` regions. A real-browser and assistive-technology pass
remains outstanding; this is a verification gap, not a passed manual check.

### Limitations and honest notes

- **No mapping draft.** The page browses metadata only. Editable drafts,
  validation, type checks, unit transforms, previews, canonical-schema
  versioning, import/export and save/rollback do not exist.
- **No sample-point preview.** The approved read path returns metadata only; no
  SQL/Flux read and no sample retrieval exist.
- **InfluxDB 1.x remains unverified.** A live 1.x schema read returns Unknown with
  the P3-002 `schema-1x-unverified` observation and no request; the mock 1.x
  preview is also Unknown.
- **Mock mode depends on the existing typed profile endpoint.** In a deployment
  without a test adapter, the profile list can error; the page shows that error
  and does not fabricate a list. The synthetic schema preview itself is local and
  labeled.
- **No real-browser visual/screen-reader validation.** See the explicit gap above.
- **No PR.** The work is checkpointed on `phase-3/canonical-mapper`; PR creation,
  approval and merge remain human-only. `main` was not changed.
