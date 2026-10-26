# P3-001 — Roadmap reconciliation and schema-discovery architecture gate

> **State: HUMAN-AUTHORIZED ACTIVE WORK — COMPLETE, VERIFIED 2026-10-26** (archived to `archive/active-work/`). The schema-discovery gate is decided and human-approved (Option A1); the documentation is reconciled. No mapper capability is implemented and no ADR is accepted.

## Purpose
Reconcile docs/ROADMAP.md, README/ICC and Phase 3 entry contract with merged PR #7 (merge bba5a676) and P2C-001–008, without claiming Phase 2 exit fully met. Decide at an explicit documented gate between a bounded backend read-only metadata API using safe nonmutating query methods versus a narrower metadata-only GET path; clarify HTTP POST query is read-only semantically but changes P2C-003's GET-only probe boundary. Record authority, role, threat model, per-edition scope, limits, unknown states, tests and decision outcome; do not silently accept ADR-0004/0005 or authorize historian mutation.

## Acceptance criteria
Roadmap and product status accurate; all linked docs updated; documented decision with security implications and approval state. If a new protected POST-based read path or other materially new security architecture requires human decision, report TASK_BLOCKED at that gate rather than treating a proposal as accepted.

## Verification
Docs validation, link check, policy, backend/frontend quick baseline; record explicit evidence.

## Dependency and boundaries
- Dependencies: merged PR #7 and Phase 3 entry contract.
- Existing Phase 2 Registry is authoritative; registered connection IDs, canonical scope and administrator-bound credential references are used server-side.
- Preserve P2L-006/007, P2C-007/008 SSRF, TLS/pinned peer, credential RBAC, lifecycle fencing and redaction.
- Only synthetic, isolated InfluxDB 2/3 instances; no production access, historian writes, pipeline deployment, Telegraf or sync runtime operations.
- Unknown/unverified is never presented as supported. InfluxDB 1.x not established.
- Any new architecture requiring human approval: record the option and report TASK_BLOCKED; do not self-approve ADR decisions.

## CWAL lifecycle
Execute only CURRENT. On verified acceptance, archive with `git mv` into `archive/active-work/`, update `handoff.md` and Active Work README to designate the next task, commit/push and verify remote SHA. Continue without intermediate approval when within existing approved boundaries. Stop with TASK_BLOCKED when blocked. On P3-006 success clear both pointers and report WORK_PACKAGE_READY. **No agent PR creation, approval or merge.**

## Resolution

P3-001 is complete and verified. It did the two things the packet asked for:

1. **Reconciled the documentation** with merged PR #7 (`bba5a676`, P2C-001–P2C-008)
   and with the Phase 3 entry contract, without claiming Phase 2 exit is fully
   met. Updated `docs/ROADMAP.md`, `README.md`, `ICC/context/project.md`,
   `ICC/INDEX.md`, `docs/contracts/canonical-mapper.md`,
   `docs/contracts/README.md`, `docs/CANONICAL_MAPPING.md` and
   `docs/LIVE_PROBING.md`. Corrected stale Phase 2 statements (browser profile
   and credential writes are delivered; the credential lifecycle now covers
   P2C-008; PR #7 is the completion workstream).
2. **Recorded the architecture decision at an explicit gate**, with authority,
   roles, threat model, per-edition scope, limits, unknown states, required
   tests, options and approval state, in
   `docs/decisions/phase-3-schema-discovery-gate.md`.

The gate was initially reported `TASK_BLOCKED` because the measured evidence
changed its premise. The human then approved the four decisions on 2026-10-26
(decision record §11): Option A1 (backend-authorized, GET-only fixed `SHOW`
metadata statements), the P2C-003 catalogue/evidence correction, a dedicated
`schema.read` permission, and the bounds/safety set. The blocked history is
retained; §11 supersedes the pending state.

### Measured basis of the decision (2026-10-26)

Read-only GET survey of the pinned isolated InfluxDB 2 OSS v2.9.1 and 3 Core
v3.12.1 containers (loopback, synthetic, tokens read from files, no writes, no
schema changes; stack torn down afterwards):

| Edition | Request | Status | Observation |
| --- | --- | --- | --- |
| 2 OSS 2.9.1 | `GET /api/v2/measurements` | 404 | no query-free listing |
| 2 OSS 2.9.1 | `GET /query?db=ttk-test-bucket&q=SHOW MEASUREMENTS` | 200 | `environment`, `sensor` |
| 2 OSS 2.9.1 | `GET /query?…SHOW FIELD KEYS` / `SHOW TAG KEYS` | 200 | field keys+types / tag keys |
| 2 OSS 2.9.1 | `GET /api/v2/query` | 405 | `allow: OPTIONS, POST` |
| 3 Core 3.12.1 | `GET /api/v3/configure/table` | 404 | no query-free listing |
| 3 Core 3.12.1 | `GET /api/v3/query_influxql?…SHOW MEASUREMENTS` | 200 | `environment`, `sensor` |
| 3 Core 3.12.1 | `…SHOW FIELD KEYS` / `SHOW TAG KEYS` | 200 | field keys+types / tag keys |
| 3 Core 3.12.1 | `GET /api/v3/query/influxql` (catalogued path) | 404 | catalogued path mismatch |

Consequence recorded, not implemented: no query-free measurement listing was
found on the tested endpoints, both editions answer fixed GET metadata
statements, and the P2C-003 "POST-only" conclusion is contradicted by the
catalogued-path mismatch. The capability model is deliberately unchanged until
the approved correction lands under P3-002.

## Changed surfaces

- `docs/decisions/phase-3-schema-discovery-gate.md` (new decision record, §11 approval)
- `docs/contracts/canonical-mapper.md` (status, §3 rows, §6 item 1, §7, new §8 decision)
- `docs/contracts/README.md` (gate status)
- `docs/ROADMAP.md` (status, phase table, capability bullet, Phase 2 completion, Phase 3 status)
- `docs/LIVE_PROBING.md` (approved correction note, corrected limitation bullet)
- `docs/CANONICAL_MAPPING.md` (status)
- `README.md` (product status)
- `ICC/INDEX.md`, `ICC/context/project.md` (routes, status, open decisions, audit)
- `handoff.md`, `workflow/active_work/README.md` (pointer advance after this archive)

## Verification (executed 2026-10-26)

| Check | Command | Result |
| --- | --- | --- |
| Backend baseline | `make check` → `gofmt` clean, `go vet ./...`, `go build ./...`, `go test ./...` | PASS |
| Frontend baseline | `make check` → `npm run lint`, `npm run build`, `npm test` | PASS, 188/188 tests, 20 files |
| Policy / secret scan | `make check` → `bash scripts/policy-check.sh` | PASS (limited regex checks only) |
| Relative links | scripted check of the 9 touched docs | 94 links, 0 broken |
| Isolated editions | `testing/influxdb/scripts/up.sh` → GET-only survey → `teardown.sh` | healthy, measurements observed, teardown clean |
| Remote checkpoint | `git push origin HEAD:phase-3/canonical-mapper` + `git ls-remote` | confirmed (SHA recorded in the checkpoint commit) |

No `go test -race` run was required: no Go code changed in P3-001.

## Limitations and honest notes

- **Nothing is implemented.** No schema-read route, no `schema.read` permission,
  no UI, no probe correction. All of that is P3-002+.
- **The capability model is unchanged.** Dialect outcomes remain `unknown`; the
  correction is approved but not applied.
- **Absence is bounded.** "No query-free listing" was observed on the tested
  endpoints of the pinned editions only; it is not generalized to other
  editions, versions or deployments.
- **InfluxDB 1.x remains unverified.** The 1.x-compatibility `/query` route on
  2.x is a compatibility surface, not evidence of 1.x support.
- **No ADR was accepted.** ADR-0004/0005 remain Proposed.
- **No production, historian, deployment or PR action.**
