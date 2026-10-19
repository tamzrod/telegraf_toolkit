# Project context

Status: documented design, not implemented.

Telegraf Toolkit is a browser-based management/control plane for official Telegraf runtimes, visual configuration, a dual-database canonical mapper, reusable connection/credential registry, and independent cross-version InfluxDB historical synchronization.

**Authority:** `IDENTITY.md`. Detailed contracts remain in `docs/`. No executable application is present at bootstrap.

**Boundaries:** No managed Telegraf fork, no floating versions, no direct browser privileged credentials/Docker access, no implicit deployment or historian repair, and no count-match equivalence claim.

**Open decisions:** metadata store, host-agent interface, production packaging and concrete adapters. Preserve these as unresolved.

**Source map:** `IDENTITY.md`, `docs/ARCHITECTURE.md`, `docs/ROADMAP.md`, `docs/adr/0001-official-telegraf-and-version-pinning.md`.

**Audit:** initial bootstrap context; re-check source files before implementation.
