import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { UseMutationResult, UseQueryResult } from "@tanstack/react-query";
import {
  fetchLiveRegistryProfiles,
  probeCapabilities,
} from "../api/client";
import { queryKeys } from "../api/queryKeys";
import type { ApiProbeResult, RegistryProfile } from "../api/registry";

/**
 * Query and mutation hooks for the live, authorized Registry API.
 *
 * These call the real backend endpoints. Without a session token the backend
 * responds 401 and the query surfaces an error; the hook never falls back to
 * mock data, so the UI cannot mistake a mock for a live result.
 */

export const liveRegistryProfilesQueryOptions = {
  queryKey: queryKeys.liveRegistryProfiles(),
  queryFn: ({ signal }: { signal: AbortSignal }) =>
    fetchLiveRegistryProfiles(signal),
  retry: false,
} as const;

/**
 * Live Registry profile list. `enabled` gates the request so the live endpoint
 * is only called when the user has selected the live source, avoiding a
 * redundant unauthorized request in mock mode.
 */
export function useLiveRegistryProfiles(
  enabled = true,
): UseQueryResult<RegistryProfile[], Error> {
  return useQuery<RegistryProfile[], Error>({
    ...liveRegistryProfilesQueryOptions,
    enabled,
  });
}

/**
 * Mutation that performs a read-only capability probe through the authorized
 * API. The result is a live, three-valued capability report; it is not mock.
 *
 * Only the connection ID is sent: the endpoint and credential are resolved
 * server-side from the stored profile.
 */
export function useProbeCapabilities(): UseMutationResult<
  ApiProbeResult,
  Error,
  { connectionId: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: { connectionId: string }) => probeCapabilities(input),
    onSuccess: () => {
      // A probe does not change the profile list, but refresh it so any
      // server-side state stays consistent.
      void queryClient.invalidateQueries({
        queryKey: queryKeys.liveRegistryProfiles(),
      });
    },
  });
}
