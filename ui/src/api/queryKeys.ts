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
};

export type QueryKeys = typeof queryKeys;
