# ADR-0005: Initial Production Packaging

- **Status:** Proposed
- **Date:** 2026-10-20
- **Scope:** How the Toolkit management application (Go backend + React SPA) is packaged and deployed relative to the isolated Telegraf runtime containers.
- **Deciders:** Human architecture owner (acceptance required).

## Context

[`IDENTITY.md`](../../IDENTITY.md) and [`docs/FRONTEND_ARCHITECTURE.md`](../FRONTEND_ARCHITECTURE.md) state that the Toolkit management application is deployed separately from isolated Compose-managed Telegraf runtime containers, and that the SPA may optionally be served by the Go application. Exact packaging remains explicitly undecided. All container images must be pinned, with verified digests in production ([ADR-0001](0001-official-telegraf-and-version-pinning.md)).

## Constraints

- Management plane separate from data plane; Telegraf keeps collecting if the Toolkit is unavailable.
- Single-host Docker Compose is the initial deployment target.
- All images pinned to exact versions; production images additionally use verified SHA256 digests; never floating tags.
- Frontend dependencies use exact, reviewed versions with a committed lockfile.
- No secrets baked into images or Compose files.
- Must support the privilege boundary in [ADR-0004](0004-host-agent-privilege-boundary.md) (no unrestricted Docker access to the app).

## Considered options

| Option | Summary | Trade-offs |
| --- | --- | --- |
| A. Two images: Go API + static SPA (served by nginx or similar) | Separate API and static-asset images | Clear separation and independent scaling; two deploy units |
| B. Single image: Go API serves built SPA | One binary/image serves API and static assets | Simple single deploy unit; couples SPA release to backend |
| C. Native binaries (no containers) | Run Go binary + static assets directly | Least container overhead; weaker reproducibility/isolation |
| D. Kubernetes/Helm chart | Orchestrated deployment | Unjustified for single-host; explicitly deferred by architecture |

## Trade-offs

- Option A gives the cleanest separation and is straightforward with Compose, at the cost of an extra image.
- Option B is the simplest and is explicitly anticipated by [`docs/FRONTEND_ARCHITECTURE.md`](../FRONTEND_ARCHITECTURE.md) ("optionally serve them from the Go application"), but ties frontend and backend release cadence together.
- Option C is not recommended where reproducible, digest-pinned deployment is required.
- Option D is out of scope until genuine multi-host/HA requirements appear.

## Security implications

- Images must be minimal, non-root where possible, and pinned by digest in production.
- No secrets in image layers, Compose files or environment dumps.
- The management app must not be granted Docker control beyond the ADR-0004 boundary.
- TLS termination and secure cookie/session handling are required at the deployment edge.

## Validation criteria

- A reproducible build produces digest-pinned images for the accepted option.
- The SPA is served correctly and API/static separation (if Option A) is verified.
- A deployment test confirms no secrets in images/Compose and correct privilege scoping.
- Rollback to a previous pinned image is demonstrated.

## Recommendation

**Proposed, not accepted.** Recommend Option B (single Go image serving the built SPA) as the initial single-host packaging for simplicity, with Option A as a straightforward later split if independent scaling is required. Option D is explicitly out of scope for Phase 0. **Human acceptance is required** before implementation.

## Decisions requiring human acceptance

- Whether the accepted initial packaging is A or B.
- Whether the SPA is served by Go or by a dedicated static-asset image.
- The base-image and digest-pinning procedure.

## References

- [`docs/contracts/README.md`](../contracts/README.md) — runtime revision contract and ADR-D candidate
- [`docs/FRONTEND_ARCHITECTURE.md`](../FRONTEND_ARCHITECTURE.md) — packaging and quality gates
- [`docs/ARCHITECTURE.md`](../ARCHITECTURE.md) — separate management/data-plane deployment
- [ADR-0001](0001-official-telegraf-and-version-pinning.md) — immutable version/digest policy
- [ADR-0004](0004-host-agent-privilege-boundary.md) — privilege boundary
