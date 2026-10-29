# Phase 4 — Authorized Pipeline Designer Work Package

**Authorization:** Human-designated Phase 4 work on `phase-4/pipeline-designer`; tasks P4-001–P4-007 are promoted under `workflow/active_work/` in strict order. Operation CWAL automatically checkpoints and advances; no per-task human permission or PR gate. PR and merge remain human-only.

## Decisions
- Full-width application shell, desktop sidebar anchored to the far left and collapsible; accessible mobile drawer.
- Layout A: central React Flow canvas + contextual right-side node settings. Keyboard-accessible equivalent.
- Typed, versioned pipeline configuration model independent of canvas coordinates. No fictional graph routing semantics; explicit Telegraf processor order, filters, and aggregator semantics.
- Graph-managed drafts generate deterministic read-only TOML. Manual-TOML-managed drafts permit advanced editing, but exactly one authoritative mode per revision; explicit transitions, recoverable snapshots, refusal of unrepresentable reverse conversion, no silent data loss.
- Focused, **verified against official Telegraf 1.40.1** plugin subset: representative inputs, processors, aggregator and outputs. Candidate plugin names are not acceptance evidence; inspect released upstream specs before support claims. Exact dependency versions.
- Approved Phase 3 canonical mapping references compiled only when transformations are supported and semantics preserved.
- Backend owns RBAC, validation, immutable approved revisions, history, atomic saves and audits. Secrets remain protected references only; no plaintext credentials in exports/logs/browser persistence.
- Phase 4 authors, validates, approves and exports **configuration artifacts only**. No Telegraf deployment, Docker/host-agent control, historian writes or synchronization. Phase 5 manages runtime; Phase 6 handles database/measurement/field historical sync.

## Execution and gates
P4-001 shell → P4-002 typed model/catalog → P4-003 canvas/settings → P4-004 TOML/validation → P4-005 manual mode/revisions → P4-006 canonical mapper integration → P4-007 integrated acceptance. Each task has a bounded acceptance gate, test evidence and checkpoint. A task can apply directly necessary test fixes, never bypass blockers or invent test results. No self-generated follow-up work packages or unapproved scope expansion.

**Exit:** A user creates a supported pipeline visually, configures plugins, reviews correct Telegraf 1.40.1 TOML, sees unsupported cases rejected, explicitly transitions to manual mode, safely saves/reloads/revises/approves/exports configuration, and references supported approved mappings. Full tests and recorded limitations, no deploy or historian writes.

**Historical integration note:** PR #8 was merged to main; earlier P3 handoff state is superseded on this designated Phase 4 branch only.