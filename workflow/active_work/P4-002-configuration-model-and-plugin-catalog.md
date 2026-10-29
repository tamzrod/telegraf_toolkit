# P4-002-configuration-model-and-plugin-catalog — Typed pipeline model and verified plugin catalog

**STATE:** AUTHORIZED  
**BRANCH:** `phase-4/pipeline-designer`  
**ORDER:** 2 of 7  
**DEPENDENCY:** P4-001 checkpointed  
**DESIGN AUTHORITY:** `docs/PHASE_4_PLAN.md` and existing accepted ADRs/contracts.

## Objective and allowed scope
Pipeline domain contracts, focused plugin metadata, validators and tests; no deployment endpoints.

## Required implementation
Define versioned graph/manual modes and plugin instances separate from layout; explicit Telegraf execution order, filtering, aggregator semantics, secret references; verify exact selected plugin options against official Telegraf 1.40.1 before marking supported; reject unsupported versions/plugins.

## Acceptance and evidence
Positive/negative contract tests, multiple instances and order/filter fixtures; `make check`; document catalog provenance/limitations; no inferred secret values or arbitrary visual routing.

## Safety / stop conditions
Do not edit `main`, open PRs, force-push, deploy, restart Telegraf, write historians, start synchronization, expose secrets, or self-expand the work package. If required contracts conflict or a privileged action is necessary, record blocker rather than invent permissions.

## CWAL checkpoint
Run real checks; report limitations; archive this task under `archive/active-work/`, commit and push evidence and advance both handoff pointers to the next authorized task automatically. Do not request intermediate approval.