# P0-003 — Reproducible scaffold and CI

## Intent
Create the first reproducible application/tooling skeleton for Phase 0, without implementing real platform operations.

## Starting state
`IDENTITY.md`, `docs/FRONTEND_ARCHITECTURE.md`, `docs/ROADMAP.md` select React/TypeScript/Vite and Go; exact versions are not yet pinned.

## Required result
Add a minimal UI and Go API skeleton, exact reviewed toolchain/package versions and committed lockfiles, documented local test commands, and CI formatting/lint/build/unit-test checks. Isolated fixtures must never contact real historians or Docker.

## Execution boundary
No database writes, runtime operations, privileged credentials, deployment, or integration claims. Do not choose a metadata database or host-agent interface without ADR approval. Prefer feature-boundary stubs only.

## Acceptance criteria
- [ ] Reproducible clean install/build/test for frontend and backend with pinned dependency versions.
- [ ] CI invokes real commands and fails on actual errors; document any skipped checks.
- [ ] A minimal locally runnable shell and health endpoint are labeled as scaffolding, not a connected product.
- [ ] Dependency and secret checks find no floating packages, leaked credentials or production access.

## Verification evidence
Record commands, exact versions, test outcomes, CI result and affected paths. If a required tool is unavailable, record blocker rather than claim PASS.

## Stop conditions
No available secure toolchain, incompatible selected dependencies, unresolved API/security boundary or test environment assumptions requiring a new architecture decision.

## Dependencies and Git
Human promotion required. **Must use dedicated branch, PR, review and merge**, even during repository initialization, because this is application implementation.
