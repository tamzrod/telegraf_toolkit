# P2-005 — Database Registry UI and typed mock API flows

**State:** HUMAN-AUTHORIZED ACTIVE WORK — explicitly approved mock/interface-only Phase 2 batch. ADR-0002 and ADR-0003 remain Proposed; no persistence, live secrets or real historian connections.
**Proposed branch:** `phase-2/database-registry`.
**Sequence:** P2-005 within a potential human-approved P2 work package.

## Objective and bounded scope
Implement navigable Registry forms, explicit source/destination scope selectors and capability/status views wired to typed test-only API adapters. Distinguish unsaved/local demo state, absent credentials and actual backend. No real endpoint probing or credential storage.

## Acceptance criteria
Accessible form validation, empty/loading/error, mock badge and connection capability statuses, no secret values, no live/prod writes, UI tests and make check pass.

## Dependencies and prohibitions
- Read relevant accepted repository architecture and BLACK SHEEP WALL context. Do not infer that ADR-0002 or ADR-0003 is accepted merely because it contains a recommendation.
- Architecture gate: persistent profile storage, real secret storage/key custody and any live credential resolution require separate human acceptance of the relevant ADR. If acceptance has not occurred, deliver only isolated interfaces/mocks/tests and stop at the genuine dependency.
- No live InfluxDB historian reads/writes or Docker/Telegraf control; no credential harvesting, secret display or unsafe TLS bypass. Keep capabilities per connection and mock responses distinguishable from live probes.
- Run acceptance tests and `make check`, record actual evidence and limitations. Upon authorization and verification, `git mv` this task into `archive/active-work/`, update `handoff.md` and Active Work pointers, commit/push and remote-confirm before advancing.
- Never self-promote or create/open/approve/merge a PR. Stop for conflict, an unresolved required architecture decision, unverified checks or a real-system privilege requirement.

## Verification evidence (recorded 2026-10-20)

**Environment:** branch `phase-2/database-registry`; Node.js 24.21.0 / npm 11.19.1; Go 1.27.3 from the official tarball. No dependency versions changed.

**Deliverables:**
- `ui/src/api/registry.ts` — typed Registry contract mirroring the backend interface: three-valued `CapabilityOutcome`, `RegistryProfile`, `RegistryCredentialRef` (no value field), runtime guards, and `hasSecretLikeKey` which rejects any payload carrying a secret-like key.
- `ui/src/api/client.ts` — `fetchRegistryProfiles` routed through the single typed boundary.
- `ui/src/api/queryKeys.ts`, `ui/src/hooks/useRegistryProfiles.ts` — query key and TanStack Query hook.
- `ui/src/api/mock/registry.ts` — isolated, mock-labeled profiles (one deliberately without a credential reference).
- `ui/src/features/databases/registryWorkflow.ts` — pure selection/draft/capability/credential logic.
- `ui/src/features/databases/DatabasesPage.tsx` — navigable Registry view: mock badge, capability badges, explicit source/destination selectors, unsaved local draft form, disabled mutation controls.
- Tests: `registryWorkflow.test.ts` (11), `DatabasesPage.test.tsx` (8).
- Test repairs (directly related): `moduleEntryPoints.test.tsx` and `App.test.tsx` asserted the Databases module had no interactive content; that Phase 1 assumption is superseded by the authorized P2-005 workflow, so the inert-shell invariant is retained for the seven remaining modules and Databases' safety boundary (mock-only notice, disabled mutations, typed-endpoint-only call) is asserted explicitly.

**Commands and actual outcomes:**
- `npx vitest run` → `Test Files 13 passed (13)`, `Tests 142 passed (142)`.
- `make check` → exit 0: backend gofmt/vet/build/test; frontend `npm ci` + `eslint` (0 errors, 0 warnings) + `tsc -b && vite build` + `vitest run`; `policy-check.sh` → PASS.

**Acceptance mapping:**
- Accessible form validation: labelled inputs/selects with `aria-invalid` and `aria-describedby`; validation messages use `role="alert"`; the distinct-pair confirmation uses `role="status"` (asserted in the selection test).
- Empty/loading/error: `LoadingState`, `EmptyState`, `ErrorState` with working retry are all wired and tested (the retry test proves a second request).
- Mock badge and capability statuses: every profile renders a `MOCK` badge; capability rows render three-valued badges; `registryWorkflow.test.ts` proves `unknown` and `unsupported` are never "confirmed".
- No secret values: the runtime guard rejects secret-like keys; `credentialState` exposes only reference id/alias/role and the page renders "No credential reference attached" for the reference-less profile; the credential test asserts no `value` field.
- No live/prod writes: mutation controls are `disabled` with `aria-disabled="true"`; the page calls only `/api/v1/registry/profiles` and the test asserts exactly one request to that path.
- Distinguish unsaved local state: the draft section states it is not saved or sent, and the status text says "It is still unsaved and cannot be submitted."

**Limitations:** the UI is mock-only; the registry endpoint is not implemented server-side, so the hook is exercised with a stubbed `fetch`. No real capability probe, credential resolution or persistence occurs. Selection and draft validation are client-side input checks, not authorization.
