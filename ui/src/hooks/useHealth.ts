import { useQuery } from "@tanstack/react-query";
import type { UseQueryResult } from "@tanstack/react-query";
import { fetchHealth } from "../api/client";
import { queryKeys } from "../api/queryKeys";
import type { Health } from "../types/health";

/**
 * Shared query options for the scaffold health endpoint.
 *
 * `retry: false` keeps the scaffold's error state deterministic for tests and
 * for a developer running without the API; TanStack Query still cancels the
 * in-flight request when the component unmounts, because the query function
 * forwards the provided `AbortSignal`.
 */
export const healthQueryOptions = {
  queryKey: queryKeys.health(),
  queryFn: ({ signal }: { signal: AbortSignal }) => fetchHealth(signal),
  retry: false,
} as const;

export function useHealth(): UseQueryResult<Health, Error> {
  return useQuery<Health, Error>(healthQueryOptions);
}
