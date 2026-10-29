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

## Verification appended 2026-10-29

Added `backend/internal/pipelinecontract` and `docs/contracts/pipeline.md`. Graph and manual documents are versioned. Plugin instances, `processor_order`, filters, and aggregator settings are separate from canvas coordinates. Parsing rejects an `edges` field. Secret options accept credential reference IDs only.

The catalog was read from official Telegraf tag `v1.40.1`, commit `26b8f4478b676f4f5e5d8ce1622cdf4f6c273bda`. Supported set: inputs `cpu`, `mem`, `disk`; processor `rename`; aggregator `basicstats`; outputs `file` and `influxdb_v2`. `basicstats` stat `variance` is rejected because `parseStats` accepts `s2`. Omitted options are not filled with upstream defaults.

`make check` exited 0.
- Backend `gofmt` check, `go vet`, `go build`, and `go test ./...` passed. `pipelinecontract` tests passed.
- UI lint, build, and test passed: 25 files passed, 1 skipped; 271 tests passed, 1 skipped. The skip is `src/api/schema.live.integration.test.ts` because `TTK_LIVE_*` was unset.
- `scripts/policy-check.sh` printed `policy-check: PASS`.

Limitations: the catalog is a subset of Telegraf 1.40.1; filter globs are `*` and `?` only; manual TOML is not converted to a graph; aggregation windows are recorded and not simulated; no TOML renderer, deployment endpoint, historian write, or secret lookup. Live InfluxDB was not contacted.