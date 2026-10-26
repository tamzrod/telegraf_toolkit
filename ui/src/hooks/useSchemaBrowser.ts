import { useQuery } from "@tanstack/react-query";
import type { UseQueryResult } from "@tanstack/react-query";
import {
  fetchSchemaFields,
  fetchSchemaMeasurements,
  fetchSchemaTags,
} from "../api/client";
import { queryKeys } from "../api/queryKeys";
import { formatScope } from "../api/registry";
import type { ApiSchemaResult, RegistryProfile } from "../api/registry";

/**
 * Read-only schema queries for the Canonical Mapper (P3-003).
 *
 * Every query is keyed by the selected profile's *identity and canonical
 * scope*, so a selection or scope change creates a new cache entry and the
 * previous answer cannot be reused. The query function forwards TanStack
 * Query's `AbortSignal`, so a request is cancelled when its observer goes away
 * (for example when the operator switches a side to another profile).
 *
 * `enabled` is controlled by the page: mock mode never issues a schema read,
 * and the detail reads are not issued until a measurement has been observed
 * and selected. The server remains the authority; these hooks add no fallback
 * and never synthesize a supported result.
 */
export function useSchemaMeasurements(
  profile: RegistryProfile | null,
  enabled: boolean,
): UseQueryResult<ApiSchemaResult, Error> {
  const connectionId = profile?.connectionId ?? "";
  const scope = profile ? formatScope(profile.scope) : "";
  return useQuery<ApiSchemaResult, Error>({
    queryKey: queryKeys.schemaMeasurements(connectionId, scope),
    queryFn: ({ signal }: { signal: AbortSignal }) => {
      if (!profile) {
        throw new Error("no connection profile is selected");
      }
      return fetchSchemaMeasurements(profile.connectionId, signal);
    },
    enabled: enabled && profile !== null,
    retry: false,
  });
}

export function useSchemaFields(
  profile: RegistryProfile | null,
  measurement: string,
  enabled: boolean,
): UseQueryResult<ApiSchemaResult, Error> {
  const connectionId = profile?.connectionId ?? "";
  const scope = profile ? formatScope(profile.scope) : "";
  return useQuery<ApiSchemaResult, Error>({
    queryKey: queryKeys.schemaFields(connectionId, scope, measurement),
    queryFn: ({ signal }: { signal: AbortSignal }) => {
      if (!profile) {
        throw new Error("no connection profile is selected");
      }
      return fetchSchemaFields(profile.connectionId, measurement, signal);
    },
    enabled: enabled && profile !== null && measurement !== "",
    retry: false,
  });
}

export function useSchemaTags(
  profile: RegistryProfile | null,
  measurement: string,
  enabled: boolean,
): UseQueryResult<ApiSchemaResult, Error> {
  const connectionId = profile?.connectionId ?? "";
  const scope = profile ? formatScope(profile.scope) : "";
  return useQuery<ApiSchemaResult, Error>({
    queryKey: queryKeys.schemaTags(connectionId, scope, measurement),
    queryFn: ({ signal }: { signal: AbortSignal }) => {
      if (!profile) {
        throw new Error("no connection profile is selected");
      }
      return fetchSchemaTags(profile.connectionId, measurement, signal);
    },
    enabled: enabled && profile !== null && measurement !== "",
    retry: false,
  });
}
