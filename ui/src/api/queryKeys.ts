/**
 * Central TanStack Query key factory.
 *
 * Keys are built from one place so features cannot drift into ad-hoc string
 * arrays and so invalidation targets stay greppable.
 */
export const queryKeys = {
  health: () => ["health"] as const,
  registryProfiles: () => ["registry", "profiles"] as const,
  liveRegistryProfiles: () => ["registry", "live", "profiles"] as const,
  // Prefix for every schema read, used when a selection, token or mode change
  // must drop and cancel the previous schema answers.
  schema: () => ["registry", "schema"] as const,
  // Schema keys include the selected profile identity *and* its canonical
  // scope. A profile whose scope changes therefore gets a different cache
  // entry, and the old answer can never be rendered for the new selection.
  schemaMeasurements: (connectionId: string, scope: string) =>
    ["registry", "schema", "measurements", connectionId, scope] as const,
  schemaFields: (connectionId: string, scope: string, measurement: string) =>
    ["registry", "schema", "fields", connectionId, scope, measurement] as const,
  schemaTags: (connectionId: string, scope: string, measurement: string) =>
    ["registry", "schema", "tags", connectionId, scope, measurement] as const,
};

export type QueryKeys = typeof queryKeys;
