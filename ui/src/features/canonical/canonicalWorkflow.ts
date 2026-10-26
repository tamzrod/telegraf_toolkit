import { formatScope } from "../../api/registry";
import type {
  ApiSchemaKind,
  ApiSchemaResult,
  CapabilityOutcome,
  RegistryProfile,
} from "../../api/registry";

/**
 * Pure presentation logic for the Canonical Mapper schema browser.
 *
 * It performs no network access and holds no credential. The live read itself
 * is the authorized P3-002 GET route; this module only decides whether a
 * profile pair is a valid selection, whether an answer is stale for that
 * selection, and how a sanitized failure should be worded.
 */

export type MapperSelection =
  | { ok: true }
  | { ok: false; reason: string };

/**
 * The mapper requires an explicit, distinct source and destination. Selecting
 * the same profile and scope on both sides is refused before any schema read is
 * issued, matching the entry contract.
 */
export function validateMapperSelection(
  source: RegistryProfile | null,
  destination: RegistryProfile | null,
): MapperSelection {
  if (!source || !destination) {
    return {
      ok: false,
      reason: "Select both a source and a destination database profile.",
    };
  }
  if (
    source.connectionId === destination.connectionId &&
    formatScope(source.scope) === formatScope(destination.scope)
  ) {
    return { ok: false, reason: "Source and destination must be distinct." };
  }
  return { ok: true };
}

/**
 * Stable identity for a selected side. It includes the canonical scope because
 * a profile's scope can change server-side while its connection ID stays the
 * same; a changed identity must reset the browser and its query cache.
 */
export function profileIdentity(profile: RegistryProfile): string {
  return `${profile.connectionId}::${formatScope(profile.scope)}`;
}

/**
 * Explains why an answer cannot be shown for the current selection.
 *
 * A response that names a different connection, scope, metadata kind or
 * measurement is stale, not authoritative for what the operator is looking at.
 * The caller renders an explicit stale state and never substitutes the data.
 */
export function schemaResultProblem(
  profile: RegistryProfile,
  kind: ApiSchemaKind,
  measurement: string,
  result: ApiSchemaResult,
): string | null {
  if (result.connection_id !== profile.connectionId) {
    return "The response is for a different connection profile.";
  }
  if (result.scope !== formatScope(profile.scope)) {
    return "The response is for a different database scope.";
  }
  if (result.kind !== kind) {
    return "The response is for a different metadata read.";
  }
  if (
    (kind === "fields" || kind === "tags") &&
    result.measurement !== measurement
  ) {
    return "The response is for a different measurement.";
  }
  if (result.mock) {
    return "The response is marked as mock data and is not a live observation.";
  }
  return null;
}

const SCHEMA_OUTCOME_LABELS: Record<CapabilityOutcome, string> = {
  supported: "Supported",
  unsupported: "Unsupported",
  unknown: "Unknown",
};

/** The honest three-valued label for a schema observation. */
export function schemaOutcomeLabel(outcome: CapabilityOutcome): string {
  return SCHEMA_OUTCOME_LABELS[outcome];
}

/** True only for a positively observed schema; unknown is never support. */
export function isSchemaSupported(outcome: CapabilityOutcome): boolean {
  return outcome === "supported";
}

/**
 * Maps a failed schema request to user-facing wording.
 *
 * The typed client keeps only the HTTP status and never surfaces the backend's
 * error body. A 404 means the profile or the selected measurement is no longer
 * present; a 403 means the role lacks `schema.read`. No message implies that a
 * failed read is an empty or unsupported schema.
 */
export function describeSchemaFailure(error: unknown): string {
  const status =
    typeof error === "object" && error !== null && "status" in error
      ? Number((error as { status: unknown }).status)
      : 0;
  switch (status) {
    case 401:
      return "A valid session token with schema.read is required; the live API rejected the request.";
    case 403:
      return "Your role is not permitted to read schema metadata (schema.read).";
    case 404:
      return "The Registry no longer has that connection profile or measurement.";
    case 400:
      return "The schema request was rejected as invalid. This page sends only a connection ID and an observed measurement.";
    case 0:
      return "The live Registry API could not be reached. No schema data is shown.";
    default:
      return "The Registry could not complete the schema read. No supported result is claimed.";
  }
}
