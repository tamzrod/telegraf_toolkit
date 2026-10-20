/**
 * Single source of truth for the application shell's primary modules.
 *
 * Navigation, routing and the Overview module list all read from this registry
 * so the shell cannot drift from what is actually reachable. Phase 1 modules are
 * shells only: every entry is either the existing scaffold or explicitly
 * "not implemented". Nothing here implies a live backend connection.
 */

export type ModuleStatus = "scaffold" | "not implemented";

export type ModuleDefinition = {
  /** Stable identifier, also used as a React key and in tests. */
  id: string;
  /** Router path for the module entry point. */
  path: string;
  /** Navigation label. */
  label: string;
  /** One-line statement of the module's intended scope. */
  description: string;
  /** Honest implementation status; never presented as a live capability. */
  status: ModuleStatus;
};

export const MODULES: ModuleDefinition[] = [
  {
    id: "overview",
    path: "/",
    label: "Overview",
    description: "Runtime health, ingestion and sync progress.",
    status: "scaffold",
  },
  {
    id: "runtimes",
    path: "/runtimes",
    label: "Runtimes",
    description:
      "Isolated Telegraf runtimes: status, logs, restart and pinned image version.",
    status: "not implemented",
  },
  {
    id: "pipelines",
    path: "/pipelines",
    label: "Pipelines",
    description:
      "Pipeline Designer: inputs, processors and outputs with TOML preview and validation.",
    status: "not implemented",
  },
  {
    id: "databases",
    path: "/databases",
    label: "Databases",
    description:
      "InfluxDB connection profiles, dialect capabilities, health and dependencies.",
    status: "not implemented",
  },
  {
    id: "credentials",
    path: "/credentials",
    label: "Credentials",
    description:
      "Secret references, permission scopes, usage, rotation and revocation.",
    status: "not implemented",
  },
  {
    id: "canonical-mapper",
    path: "/canonical-mapper",
    label: "Canonical Mapper",
    description:
      "Dual-database measurement, field and tag mapping to a canonical schema.",
    status: "not implemented",
  },
  {
    id: "synchronizer",
    path: "/synchronizer",
    label: "Synchronizer",
    description:
      "Source/target selection, time-block comparison, backfill and verification.",
    status: "not implemented",
  },
  {
    id: "settings",
    path: "/settings",
    label: "Audit / Settings",
    description: "Change logs, permissions and version policy.",
    status: "not implemented",
  },
];

export function findModuleById(id: string): ModuleDefinition | undefined {
  return MODULES.find((module) => module.id === id);
}
