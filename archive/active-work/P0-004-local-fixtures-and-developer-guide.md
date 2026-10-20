# P0-004 — Isolated development fixtures and onboarding

## Intent
Make Phase 0 repeatable for subsequent OpenHands tasks with test-only data and a developer guide.

## Starting state
`docs/ROADMAP.md`, `docs/FRONTEND_ARCHITECTURE.md`, `docs/DATABASE_REGISTRY.md`, and scaffolding from P0-003.

## Required result
Document setup, clean environment creation, exact check commands and sanitized mock InfluxDB 1.x/2.x/3.x capability fixture shapes. Add deterministic local fixtures and tests for source/destination selection and count-match-versus-verification naming.

## Execution boundary
Fixtures/mock adapters only. No real DB credentials, live historian endpoints, Docker socket access or production runtime operations.

## Acceptance criteria
- [ ] README links to a reproducible developer setup guide and sample test commands.
- [ ] All fixture connections are plainly mock/test-only.
- [ ] Fixture tests cover distinct source/destination scopes, unsupported capabilities and `COUNT_MATCH` semantics.
- [ ] CI succeeds using local isolated fixtures, without external services.

## Verification evidence
Record clean-environment commands, fixture test outcomes and portability limits.

## Stop conditions
If tests require real systems, stop and propose a mock/isolated substitute without claiming integration coverage.

## Dependencies and Git
After P0-003. Human promotion required; use task branch/PR.
