import { useQuery } from "@tanstack/react-query";
import type { UseQueryResult } from "@tanstack/react-query";
import { fetchRegistryProfiles } from "../api/client";
import { queryKeys } from "../api/queryKeys";
import type { RegistryProfile } from "../api/registry";

/**
 * Shared query options for the registry profile list.
 *
 * `retry: false` keeps the error state deterministic for tests and for a
 * developer running without the API; TanStack Query still cancels the in-flight
 * request on unmount because the query function forwards the `AbortSignal`.
 */
export const registryProfilesQueryOptions = {
  queryKey: queryKeys.registryProfiles(),
  queryFn: ({ signal }: { signal: AbortSignal }) =>
    fetchRegistryProfiles(signal),
  retry: false,
} as const;

export function useRegistryProfiles(): UseQueryResult<RegistryProfile[], Error> {
  return useQuery<RegistryProfile[], Error>(registryProfilesQueryOptions);
}
