import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { UseMutationResult, UseQueryResult } from "@tanstack/react-query";
import {
  createLiveProfile,
  deleteLiveProfile,
  enrollCredential,
  fetchLiveRegistryProfiles,
  probeCapabilities,
  replaceCredential,
  revokeCredential,
} from "../api/client";
import { queryKeys } from "../api/queryKeys";
import type {
  ApiCredentialInput,
  ApiCredentialRef,
  ApiCredentialReplaceInput,
  ApiProbeResult,
  ApiProfile,
  ApiProfileInput,
  RegistryProfile,
} from "../api/registry";

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

/**
 * Mutation that creates an **unbound** connection profile through the
 * authorized API.
 *
 * The body carries no credential reference and no credential value. On success
 * the live profile list is invalidated so the new profile is fetched from the
 * server rather than assumed locally (P2C-004).
 */
export function useCreateLiveProfile(): UseMutationResult<
  ApiProfile,
  Error,
  ApiProfileInput
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: ApiProfileInput) => createLiveProfile(input),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.liveRegistryProfiles(),
      });
    },
  });
}

/**
 * Mutation that deletes a connection profile through the authorized API.
 *
 * The server removes the profile and its approved credential bindings in one
 * transaction. On success the live profile list is invalidated so the UI never
 * shows a profile the server no longer holds (P2C-004).
 */
export function useDeleteLiveProfile(): UseMutationResult<
  void,
  Error,
  { connectionId: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: { connectionId: string }) =>
      deleteLiveProfile(input.connectionId),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.liveRegistryProfiles(),
      });
    },
  });
}

/**
 * Mutation that enrolls a credential for a profile.
 *
 * The value is write-only: it is submitted once and never returned, so the
 * browser holds it only until the request is sent. The enrolment requires the
 * administrator-only permissions the server enforces (P2C-005).
 */
export function useEnrollCredential(): UseMutationResult<
  ApiCredentialRef,
  Error,
  ApiCredentialInput
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: ApiCredentialInput) => enrollCredential(input),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.liveRegistryProfiles(),
      });
    },
  });
}

/**
 * Mutation that replaces the credential a profile is bound to.
 */
export function useReplaceCredential(): UseMutationResult<
  ApiCredentialRef,
  Error,
  ApiCredentialReplaceInput
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: ApiCredentialReplaceInput) => replaceCredential(input),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.liveRegistryProfiles(),
      });
    },
  });
}

/**
 * Mutation that revokes a stored credential.
 *
 * The server refuses the revocation while an approved binding still requires
 * the credential, so the UI reports that as a refusal rather than as a
 * success.
 */
export function useRevokeCredential(): UseMutationResult<
  void,
  Error,
  { credentialRefId: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: { credentialRefId: string }) =>
      revokeCredential(input.credentialRefId),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: queryKeys.liveRegistryProfiles(),
      });
    },
  });
}
