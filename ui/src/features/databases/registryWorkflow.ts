import type {
  ApiProbeObservation,
  ApiProbeResult,
  ApiProfileInput,
  ApiScope,
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

/**
 * A profile draft as the browser collects it. It holds no credential: a
 * browser-created profile is unbound, and credential attachment stays gated
 * until the credential lifecycle is delivered (P2C-005).
 */
export type ProfileDraft = {
  connectionId: string;
  displayName: string;
  family: string;
  majorVersion: string;
  endpoint: string;
  scopeKind: "organization-bucket" | "database" | "database-retention-policy";
  organization: string;
  bucket: string;
  database: string;
  retentionPolicy: string;
  queryDialects: string;
  writeProtocols: string;
  schemaDiscovery: boolean;
  note: string;
};

/** The fields of a draft that can be invalid. */
export type ProfileDraftField =
  | "connectionId"
  | "displayName"
  | "family"
  | "majorVersion"
  | "endpoint"
  | "scope"
  | "capabilities";

export type ProfileDraftValidation = {
  ok: boolean;
  /** The invalid fields, so the form can mark them individually. */
  invalid: ProfileDraftField[];
  /** A short, human-readable summary of what is missing or wrong. */
  reason: string;
};

/**
 * Validates a profile draft before it is sent to the server.
 *
 * This is a client-side guard only: the server validates again and is
 * authoritative. The rules mirror the server contract so the user is told what
 * is wrong before a request is made, and a rejected request still never turns
 * into a local success.
 */
export function validateProfileDraft(draft: ProfileDraft): ProfileDraftValidation {
  const invalid: ProfileDraftField[] = [];

  if (draft.connectionId.trim() === "") {
    invalid.push("connectionId");
  }
  if (draft.displayName.trim() === "") {
    invalid.push("displayName");
  }
  if (draft.family.trim() === "") {
    invalid.push("family");
  }
  const majorVersion = Number(draft.majorVersion.trim());
  if (!Number.isInteger(majorVersion) || majorVersion < 1) {
    invalid.push("majorVersion");
  }
  if (!isValidEndpoint(draft.endpoint)) {
    invalid.push("endpoint");
  }
  if (!isCompleteScope(draft)) {
    invalid.push("scope");
  }
  if (parseList(draft.queryDialects).length === 0) {
    invalid.push("capabilities");
  }

  if (invalid.length === 0) {
    return { ok: true, invalid, reason: "The profile is ready to be created." };
  }
  return { ok: false, invalid, reason: draftProblem(draft, invalid) };
}

/**
 * Reports whether an endpoint is an absolute http(s) URL without embedded
 * credentials. A credential-bearing URL is refused, exactly as the server
 * refuses it.
 */
export function isValidEndpoint(raw: string): boolean {
  const trimmed = raw.trim();
  if (trimmed === "") {
    return false;
  }
  let url: URL;
  try {
    url = new URL(trimmed);
  } catch {
    return false;
  }
  if (url.protocol !== "http:" && url.protocol !== "https:") {
    return false;
  }
  if (url.username !== "" || url.password !== "") {
    return false;
  }
  return url.host !== "";
}

/** Reports whether the draft's scope carries the fields its kind requires. */
export function isCompleteScope(draft: ProfileDraft): boolean {
  switch (draft.scopeKind) {
    case "organization-bucket":
      return draft.organization.trim() !== "" && draft.bucket.trim() !== "";
    case "database":
      return draft.database.trim() !== "";
    case "database-retention-policy":
      return draft.database.trim() !== "" && draft.retentionPolicy.trim() !== "";
  }
}

/** Splits a comma-separated list into trimmed, non-empty entries. */
export function parseList(raw: string): string[] {
  return raw
    .split(",")
    .map((item) => item.trim())
    .filter((item) => item !== "");
}

/** Builds the request body for a validated draft. */
export function profileInputFromDraft(draft: ProfileDraft): ApiProfileInput {
  const scope: ApiScope =
    draft.scopeKind === "organization-bucket"
      ? {
          kind: "organization-bucket",
          organization: draft.organization.trim(),
          bucket: draft.bucket.trim(),
        }
      : draft.scopeKind === "database-retention-policy"
        ? {
            kind: "database-retention-policy",
            database: draft.database.trim(),
            retention_policy: draft.retentionPolicy.trim(),
          }
        : { kind: "database", database: draft.database.trim() };

  return {
    connection_id: draft.connectionId.trim(),
    display_name: draft.displayName.trim(),
    family: draft.family.trim(),
    major_version: Number(draft.majorVersion.trim()),
    endpoint: draft.endpoint.trim(),
    scope,
    capabilities: {
      query_dialects: parseList(draft.queryDialects),
      write_protocols: parseList(draft.writeProtocols),
      schema_discovery: draft.schemaDiscovery,
    },
    note: draft.note.trim(),
  };
}

function draftProblem(draft: ProfileDraft, invalid: ProfileDraftField[]): string {
  if (invalid.includes("connectionId")) {
    return "A connection ID is required.";
  }
  if (invalid.includes("displayName")) {
    return "A display name is required.";
  }
  if (invalid.includes("majorVersion")) {
    return "The major version must be a whole number of at least 1.";
  }
  if (invalid.includes("endpoint")) {
    return "The endpoint must be an absolute http or https URL without embedded credentials.";
  }
  if (invalid.includes("scope")) {
    switch (draft.scopeKind) {
      case "organization-bucket":
        return "An organization and a bucket are required for this scope.";
      case "database-retention-policy":
        return "A database and a retention policy are required for this scope.";
      default:
        return "A database is required for this scope.";
    }
  }
  if (invalid.includes("capabilities")) {
    return "At least one query dialect is required.";
  }
  return "The profile is incomplete.";
}

/**
 * Maps a failed API call to a message the user can act on.
 *
 * The backend's own error body is deliberately not surfaced: the typed client
 * keeps only the status, so this mapping is the single place where a status
 * becomes wording. A 401 means the session token is missing or invalid, a 403
 * means the role is not permitted, and a 409 means the identity already
 * exists.
 */
export function describeApiFailure(error: unknown): string {
  const status =
    typeof error === "object" && error !== null && "status" in error
      ? Number((error as { status: unknown }).status)
      : 0;
  switch (status) {
    case 401:
      return "The live API rejected the session token. Apply a valid token and try again.";
    case 403:
      return "Your role is not permitted to perform this action in the Registry.";
    case 404:
      return "The profile is no longer present in the Registry.";
    case 409:
      return "A profile with that connection ID already exists.";
    case 422:
      return "The Registry refused the profile as it was sent.";
    case 400:
      return "The Registry rejected the profile as invalid.";
    case 0:
      return "The live Registry API could not be reached.";
    default:
      return "The live Registry API could not complete the request.";
  }
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
