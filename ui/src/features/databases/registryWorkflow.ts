import type {
  ApiProbeObservation,
  ApiProbeResult,
  CapabilityOutcome,
  RegistryProfile,
  RegistryScope,
} from "../../api/registry";
import { formatScope, outcomeOf } from "../../api/registry";

/**
 * Pure presentation logic for the Database Registry mock workflow.
 *
 * Kept free of React and of any network access so the validation and capability
 * rules can be tested directly. Nothing here resolves a credential or probes a
 * real endpoint.
 */

export type SelectionResult =
  | { ok: true }
  | { ok: false; reason: string };

/**
 * Enforces the contract rule that a source and destination are explicit and
 * distinct before any schema browsing or mapping.
 */
export function validateSelection(
  source: RegistryProfile | null,
  destination: RegistryProfile | null,
): SelectionResult {
  if (!source || !destination) {
    return { ok: false, reason: "Select both a source and a destination database." };
  }
  if (
    source.connectionId === destination.connectionId &&
    formatScope(source.scope) === formatScope(destination.scope)
  ) {
    return { ok: false, reason: "Source and destination must be distinct." };
  }
  return { ok: true };
}

const OUTCOME_LABELS: Record<CapabilityOutcome, string> = {
  supported: "Supported",
  unsupported: "Unsupported",
  unknown: "Unknown (not probed)",
};

export function outcomeLabel(outcome: CapabilityOutcome): string {
  return OUTCOME_LABELS[outcome];
}

/**
 * True only when a capability is positively confirmed. `unsupported` and
 * `unknown` are both non-supporting, and `unknown` is never rendered as support.
 */
export function isConfirmed(outcome: CapabilityOutcome): boolean {
  return outcome === "supported";
}

export type CapabilityRow = {
  label: string;
  outcome: CapabilityOutcome;
  /** False for `unsupported` and `unknown`, so a badge cannot imply support. */
  confirmed: boolean;
};

/** Flattens a profile's capability maps into labelled, ordered rows. */
export function capabilityRows(profile: RegistryProfile): CapabilityRow[] {
  const rows: CapabilityRow[] = [];
  for (const dialect of ["influxql", "flux", "sql"]) {
    const outcome = outcomeOf(profile.capabilities.queryDialects, dialect);
    rows.push({
      label: `Query: ${dialect}`,
      outcome,
      confirmed: isConfirmed(outcome),
    });
  }
  for (const protocol of ["line-protocol"]) {
    const outcome = outcomeOf(profile.capabilities.writeProtocols, protocol);
    rows.push({
      label: `Write: ${protocol}`,
      outcome,
      confirmed: isConfirmed(outcome),
    });
  }
  rows.push({
    label: "Schema discovery",
    outcome: profile.capabilities.schemaDiscovery,
    confirmed: isConfirmed(profile.capabilities.schemaDiscovery),
  });
  return rows;
}

/**
 * Flattens a live probe result into labelled, ordered rows.
 *
 * The three states are carried through unchanged: `unknown` stays `unknown`
 * and is never collapsed into `supported` or `unsupported`. A capability is
 * only confirmed when the probe observed it (P2C-003).
 */
export function probeCapabilityRows(result: ApiProbeResult): CapabilityRow[] {
  const rows: CapabilityRow[] = [];
  for (const dialect of ["influxql", "flux", "sql"]) {
    const outcome = outcomeOf(result.query_dialects, dialect);
    rows.push({
      label: `Query: ${dialect}`,
      outcome,
      confirmed: isConfirmed(outcome),
    });
  }
  for (const protocol of ["line-protocol"]) {
    const outcome = outcomeOf(result.write_protocols, protocol);
    rows.push({
      label: `Write: ${protocol}`,
      outcome,
      confirmed: isConfirmed(outcome),
    });
  }
  rows.push({
    label: "Schema discovery",
    outcome: result.schema_discovery,
    confirmed: isConfirmed(result.schema_discovery),
  });
  return rows;
}

/**
 * The observations that back one capability of a live probe result, in the
 * order the probe recorded them.
 */
export function observationsFor(
  result: ApiProbeResult,
  capability: string,
): ApiProbeObservation[] {
  return result.observations.filter(
    (observation) => observation.capability === capability,
  );
}

export type CredentialState =
  | { kind: "reference"; alias: string; role: string; credentialRefId: string }
  | { kind: "absent" };

/** Describes whether a profile has a credential reference attached. */
export function credentialState(profile: RegistryProfile): CredentialState {
  const ref = profile.credentialRef;
  if (!ref) {
    return { kind: "absent" };
  }
  return {
    kind: "reference",
    alias: ref.alias,
    role: ref.role,
    credentialRefId: ref.credentialRefId,
  };
}

/**
 * Validates an unsaved, local draft profile form. This is client-side input
 * validation only; it is not authorization and does not persist anything.
 */
export function validateDraft(input: {
  connectionId: string;
  displayName: string;
  scope: RegistryScope;
}): SelectionResult {
  if (input.connectionId.trim() === "") {
    return { ok: false, reason: "A connection ID is required." };
  }
  if (input.displayName.trim() === "") {
    return { ok: false, reason: "A display name is required." };
  }
  if (input.scope.kind === "database-retention-policy") {
    if (!input.scope.database || !input.scope.retentionPolicy) {
      return { ok: false, reason: "A 1.x scope requires a database and retention policy." };
    }
  } else if (input.scope.kind === "organization-bucket") {
    if (!input.scope.organization || !input.scope.bucket) {
      return { ok: false, reason: "A 2.x scope requires an organization and bucket." };
    }
  } else if (!input.scope.database) {
    return { ok: false, reason: "A 3.x scope requires a database." };
  }
  return { ok: true };
}
