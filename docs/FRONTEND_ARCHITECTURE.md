# Frontend Architecture and Development Plan

Status: **Selected design direction; implementation not started**.

## Technology stack
- **React + TypeScript** for one browser-based single-page application.
- **Vite** for development and production builds.
- **Tailwind CSS** and **shadcn/ui** for styling and accessible reusable components.
- **React Flow (`@xyflow/react`)** for the input -> processor -> output graph editor.
- **dnd-kit** for moving measurements/fields/tags between database explorers.
- **Monaco Editor** for Telegraf TOML editing and diff/preview.
- **TanStack Query** for API data fetching, caching, retries and refresh.
- **Go backend API** for privileged operations and database access (adopted as the design direction; implementation details remain TBD).

Dependencies must use **exact, deliberately reviewed versions**, with a committed lockfile. No floating package versions, dependency ranges or `latest` tags in runnable build/deployment definitions. Select and verify exact frontend package versions when creating the initial scaffold; this document does not invent package version numbers.

## Application model
Build **one SPA**, with modules for Overview, Runtimes, Pipeline Designer, Databases, Credentials, Taglist/Canonical Mapper, Synchronizer, and Audit/Settings. Shared connection profiles, permissions and canonical schemas must behave consistently across modules.

```text
React SPA
  ├── Overview / Runtime Manager
  ├── Pipeline Designer + TOML Editor
  ├── Database Registry + Credential References
  ├── Dual-database Taglist / Canonical Mapper
  └── Synchronizer + Time-block Explorer
              |
         HTTPS JSON API
      + SSE/WebSocket events
              |
           Go backend
     ├── Database adapters
     ├── Protected secrets store
     ├── Docker/host-agent controller
     └── Sync job engine
```

The browser must never connect directly to databases using stored API keys, resolve raw vault secrets, or access the Docker daemon. The backend enforces authentication, authorization, validation and auditable write/deploy operations. Prefer SSE for server-to-browser status/job progress when adequate; use WebSockets only for genuinely bidirectional updates.

## Suggested frontend structure

```text
ui/
  src/
    app/             # routes, navigation, providers
    components/
      layout/
      common/
      dialogs/
    features/
      overview/
      runtimes/
      pipelines/
      databases/
      credentials/
      canonical/
      synchronizer/
    api/             # typed clients
    hooks/
    types/
    styles/
  public/
  package.json
  vite.config.ts
```

Prefer feature-local UI, hooks, schema validation and tests. Keep the backend's API contracts typed, versioned, and independent of UI component state.

## First build milestones
1. Application shell: navigation, desktop-friendly responsive design, theme, empty/error/loading states and simulated authorization.
2. Database Registry and credential-reference workflows using clearly labeled mock data.
3. **Dual-database mapper**: select Source Database and Destination Database before any schema browsing; load measurements, field keys and tag keys; create and validate draft canonical mappings.
4. Pipeline Designer: visual plugin graph, version-aware plugin settings, TOML generate/preview/diff, safe deploy workflow.
5. Runtime Manager: independently managed Compose Telegraf containers, pinned runtime versions, status, logs, restart and config revision history.
6. Synchronizer: drag from source to destination, draft job review, time-block differences and separate verification/backfill progress.
7. Integration, auth/RBAC hardening, tests and production packaging.

Mock UI must never claim to have contacted real systems. A mapping Save operation does not start a sync or deploy a live pipeline.

## Packaging
In production, build the SPA to static assets and **optionally serve them from the Go application**, yielding a simple management deployment separate from Telegraf data-plane containers. Exact packaging choice awaits implementation/testing; all container images must be pinned, with verified digests in production.

## Quality gates
- Keyboard/accessibility support alongside drag-and-drop.
- Responsive layout for narrow and wide screens.
- Unit tests for mapping/validation; end-to-end tests for source/destination switching, save draft, apply confirmation and failure states.
- Version-aware plugin editing; no silent modifications to active pipeline.
- Sanitized API errors; no secrets in browser storage, URL, logs, snapshots, or exported schema files.

See [UI_DESIGN.md](UI_DESIGN.md), [ARCHITECTURE.md](ARCHITECTURE.md) and [DATABASE_REGISTRY.md](DATABASE_REGISTRY.md).
