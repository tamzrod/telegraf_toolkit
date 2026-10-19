# ADR-0001: Official Telegraf Runtime and Immutable Version Policy

- **Status:** Accepted
- **Date:** 2026-10-19
- **Scope:** All deployed Toolkit and Telegraf runtime dependencies

## Decision

Telegraf Toolkit is an independent management application. **Use upstream official Telegraf release binaries/images**, not a maintained source fork or an in-house Telegraf compilation, for the initial implementation.

The **initial tested Telegraf baseline is exactly 1.40.1**. This is a project compatibility baseline, **not** an instruction to upgrade when a newer version is published. Support for other versions requires explicit testing and documentation. Managed remote installations may report other versions; detect their capabilities rather than assuming all plugins exist.

## Mandatory version pinning

- Never use `latest`, `stable`, `edge`, `main`, `master`, wildcard tags, version ranges, or other floating identifiers for runtime dependencies in production/deployment manifests.
- Pin Docker images to **explicit patch-level versions** (e.g. `telegraf:1.40.1`). For production reproducibility, additionally pin by image **SHA256 digest**, resolved and verified against the target platform during deployment; never invent a digest.
- Pin native binary versions explicitly and verify release checksums.
- Pin application base images, database images, runtime tools and build dependencies. Commit lockfiles for language dependencies. Pin GitHub Actions to full commit SHAs in operational workflows wherever feasible.
- All upgrades are intentional: change version pin, review upstream changes, test supported plugins/configs/sync paths, then promote. No automatic version drift.
- A floating tag must never appear in a runnable example, deployment template, or CI release job.

## Runtime integration

```text
Telegraf Toolkit UI + Backend (separate processes)
  |-- Instance Manager -> Telegraf v1.40.1 official binary/container
  |-- TOML Config Editor -> validate, stage, deploy, rollback
  |-- Canonical Registry -> mapping profiles
  +-- Independent Sync Controller -> read adapters / time-block index / backfill / verification

Telegraf v1.40.1: inputs -> processors -> outputs -> data destinations
```

The Toolkit does not depend on Telegraf internal Go packages as a control-plane API. Prefer native TOML, explicit process/container lifecycle controls, version-aware plugin discovery and external extension mechanisms. Build upstream source only if a demonstrated requirement cannot be met otherwise and capture that exception in a new ADR.

## Example Compose (development baseline)

```yaml
services:
  telegraf:
    image: telegraf:1.40.1
    restart: unless-stopped
    volumes:
      - ./telegraf:/etc/telegraf:ro
```

This example is **illustrative**, not a complete deployable Toolkit stack. Production manifests must also use a verified immutable digest and deliberate deployment-specific configuration.

## Rationale

Official upstream builds avoid maintenance of a fork, simplify security servicing, and keep the Toolkit focused on management, canonical mapping and selective historical synchronization. Immutable version selection makes validation, incident response and rollback reproducible.

## Consequences

- Initial development and test fixture target: Telegraf 1.40.1.
- Introduce a version/capability matrix before claiming support for older/newer installations.
- Track upstream security fixes but do not roll forward without an approved version change.
- Telegraf remains independently restartable from the Toolkit UI and historical sync worker.
