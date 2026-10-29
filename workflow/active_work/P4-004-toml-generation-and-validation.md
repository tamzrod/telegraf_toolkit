# P4-004-toml-generation-and-validation — Deterministic Telegraf TOML generator

**STATE:** AUTHORIZED  
**BRANCH:** `phase-4/pipeline-designer`  
**ORDER:** 4 of 7  
**DEPENDENCY:** P4-003 checkpointed  
**DESIGN AUTHORITY:** `docs/PHASE_4_PLAN.md` and existing accepted ADRs/contracts.

## Objective and allowed scope
Generator, validation, fixtures and preview UI; no deployment.

## Required implementation
Build TOML from typed model with stable ordering and safe encoding; read-only graph-mode preview; distinguish parse/config/support checks; verify representative TOML against pinned Telegraf 1.40.1 binary or approved fixture harness; no secret material in generated output.

## Acceptance and evidence
Golden fixtures and invalid-case tests, determinism roundtrips where valid, documented validation coverage; `make check`; no fictional routing semantics.

## Safety / stop conditions
Do not edit `main`, open PRs, force-push, deploy, restart Telegraf, write historians, start synchronization, expose secrets, or self-expand the work package. If required contracts conflict or a privileged action is necessary, record blocker rather than invent permissions.

## CWAL checkpoint
Run real checks; report limitations; archive this task under `archive/active-work/`, commit and push evidence and advance both handoff pointers to the next authorized task automatically. Do not request intermediate approval.