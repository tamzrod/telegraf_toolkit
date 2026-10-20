# ADR-0004: Host-Agent Privilege Boundary and Control Interface

- **Status:** Proposed
- **Date:** 2026-10-20
- **Scope:** How the Toolkit backend performs privileged host/runtime operations (Telegraf container lifecycle, config deployment) without exposing Docker control to the browser or the public API.
- **Deciders:** Human architecture owner (acceptance required).

## Context

[`IDENTITY.md`](../../IDENTITY.md) requires managing many independent Telegraf runtimes (one container per runtime by default) via single-host Docker Compose, while explicitly forbidding browser access to the Docker daemon. [`docs/ARCHITECTURE.md`](../ARCHITECTURE.md) states that Docker operations belong in a separately authorized management service or restricted host agent, and that remote process control must not be assumed universally available.

This ADR defines the **privilege boundary** and a **control interface shape**; it does not select a concrete agent protocol or implementation.

## Constraints

- The browser must never receive Docker socket/daemon control.
- The management API must not expose unrestricted Docker access.
- Privileged operations (deploy, restart, rollback) must be authorized and audited server-side.
- One runtime equals one container by default; independent lifecycles; a failure must not require restarting unrelated runtimes.
- Runtime images must be pinned to exact versions; production images to verified digests (see [ADR-0001](0001-official-telegraf-and-version-pinning.md)).
- Must work on a single host with Docker Compose; no Kubernetes/Swarm requirement.
- Config containing secrets must be delivered to runtimes through a protected mechanism, never world-readable Compose/TOML.

## Considered options

| Option | Summary | Trade-offs |
| --- | --- | --- |
| A. Backend holds restricted Docker API access | Backend talks to a scoped Docker endpoint | Simplest on single host; must tightly scope and isolate credentials/permissions |
| B. Separate restricted host agent | A minimal local agent exposes a narrow, authorized operation set | Clean privilege boundary; extra component to build, secure and pin |
| C. Compose file generation only | Toolkit writes Compose/config; external process applies it | Strong isolation; weak interactive lifecycle control and status feedback |
| D. Full Docker socket passthrough to backend | Backend mounts the Docker socket directly | Maximum capability; largest blast radius if the backend is compromised |

## Trade-offs

- Option A is pragmatic for the single-host foundation but must not degrade into Option D; the Docker surface must be scoped and isolated from the public API path.
- Option B gives the best least-privilege boundary and a natural path to multi-host management later, at the cost of an additional component.
- Option C is the safest but sacrifices the interactive runtime management the product requires.

## Security implications

- Least privilege: the control path must expose only the operations required (create/start/stop/restart/inspect/logs), not arbitrary Docker or host commands.
- Authorization and audit are enforced in the backend, not the UI.
- Secret injection into runtime configs must use restricted delivery.
- The privileged control component must be unreachable from the browser and, where possible, from the general network.
- No privileged container mode or host filesystem access beyond what a runtime strictly needs.

## Validation criteria

- A test proves the browser/API cannot reach Docker directly and that only authorized, audited operations are possible.
- A failure-isolation test proves restarting one runtime does not restart unrelated runtimes.
- A secret-injection test proves runtime config secrets are delivered through the protected mechanism and are absent from logs and Git.
- The control interface is documented with an explicit operation allow-list.

## Recommendation

**Proposed, not accepted.** Recommend Option B (a separate restricted host agent with a narrow, authorized operation set) as the target boundary, with Option A permitted only as a strictly scoped interim on the single-host foundation and Option D explicitly rejected. The concrete agent protocol is intentionally left unresolved. **Human acceptance is required** before implementation.

## Decisions requiring human acceptance

- Whether the accepted boundary is A (scoped backend) or B (separate agent).
- The concrete control-interface protocol and its operation allow-list.
- Whether multi-host management is in scope for a later phase.

## References

- [`docs/contracts/README.md`](../contracts/README.md) — runtime revision contract and ADR-C candidate
- [`docs/ARCHITECTURE.md`](../ARCHITECTURE.md) — containerized runtime isolation and orchestration
- [`IDENTITY.md`](../../IDENTITY.md) — no browser Docker control
- [ADR-0001](0001-official-telegraf-and-version-pinning.md) — pinned runtime versions
