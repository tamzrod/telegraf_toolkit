# P0-001 — Domain/API contract inventory

## Intent
Define a reviewable catalog of the domain entities and API boundaries required by Phase 0. This is a **planning proposal**, not authorized execution.

## Starting state
Read `IDENTITY.md`, `docs/ARCHITECTURE.md`, `docs/DATABASE_REGISTRY.md`, `docs/CANONICAL_MAPPING.md`, `docs/SYNCHRONIZATION.md`. These are proposals, not implemented APIs.

## Required result
Create `docs/contracts/README.md` containing a versioned contract inventory for: connection profiles, logical database scopes, credential references, probed capabilities, canonical schemas, mapping profiles, runtime revisions, sync jobs/UTC half-open blocks, verification states and audit events. Include ownership, explicit source/destination semantics, security boundary and unresolved schema details for each.

## Execution boundary
Documentation only. Do not invent final wire formats, Go types, REST routes, database schemas or auth mechanisms before accepted decisions. Do not create runnable adapters or services.

## Acceptance criteria
- [ ] All listed domains are accounted for with source references and planned/decided separation.
- [ ] Source and destination selected explicitly; schemas and mapping profiles remain separate portable artifacts.
- [ ] `COUNT_MATCH` remains distinct from `VERIFIED`; no secrets in contract examples.
- [ ] Open decisions are explicitly marked and cross-linked to ADR candidates.

## Verification evidence
Review all cited specs, links and terminology; report a domain-coverage checklist and changed paths. No runtime tests required for documentation-only output.

## Stop conditions
If documents conflict on an established invariant, stop and report it rather than choosing an implementation.

## Dependencies and Git
No application dependencies. Human promotion required; use a task branch/PR for contract delivery unless this is separately approved as initialization-only foundation documentation.
