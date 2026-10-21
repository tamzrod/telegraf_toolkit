/**
 * Typed Registry contract for the browser.
 *
 * This mirrors the backend interface contract in
 * `backend/internal/registrycontract` and `docs/contracts/registry.md`:
 * capabilities are three-valued and probed per connection, a profile carries a
 * credential *reference* (never a value), and every payload is explicitly
 * labeled mock until a real backend implements it.
 *
 * Nothing here resolves a credential, probes a real endpoint or performs a
 * write.
 */

/** Three-valued capability outcome. `unknown` is never treated as support. */
export type CapabilityOutcome = "supported" | "unsupported" | "unknown";

export type RegistryScopeKind =
  | "database-retention-policy"
  | "organization-bucket"
  | "database";

export type RegistryScope = {
  kind: RegistryScopeKind;
  database?: string;
  retentionPolicy?: string;
  organization?: string;
  bucket?: string;
};

export type RegistryCapabilities = {
  queryDialects: Record<string, CapabilityOutcome>;
  writeProtocols: Record<string, CapabilityOutcome>;
  schemaDiscovery: CapabilityOutcome;
};

/** A credential reference. It deliberately has no value-bearing field. */
export type RegistryCredentialRef = {
  credentialRefId: string;
  alias: string;
  role: "read" | "write" | "admin";
  mock: boolean;
};

export type RegistryProfile = {
  connectionId: string;
  displayName: string;
  family: string;
  majorVersion: number;
  scope: RegistryScope;
  capabilities: RegistryCapabilities;
  /** Absent or null when no credential reference is attached. */
  credentialRef?: RegistryCredentialRef | null;
  /**
   * Opaque credential reference ID reported by the live API. It is present when
   * a profile carries a reference but the API did not return full reference
   * metadata. It is never a credential value.
   */
  credentialRefId?: string | null;
  mock: boolean;
  note: string;
};

const OUTCOMES: CapabilityOutcome[] = ["supported", "unsupported", "unknown"];

/**
 * Wire shape returned by the live authorized API (`GET /api/v1/registry/profiles`).
 *
 * The live backend emits snake_case keys and a credential reference ID, never a
 * credential value. The browser-side view model (`RegistryProfile`) is derived
 * from this shape by `toRegistryProfile`.
 */
export type ApiScope = {
  kind: RegistryScopeKind;
  database?: string;
  retention_policy?: string;
  organization?: string;
  bucket?: string;
};

export type ApiCapabilities = {
  query_dialects: string[];
  write_protocols: string[];
  schema_discovery: boolean;
};

export type ApiCredentialRef = {
  credential_ref_id: string;
  alias: string;
  role: "read" | "write" | "admin";
  fingerprint?: string;
  last_rotated?: string;
  expires_at?: string;
  mock: boolean;
};

export type ApiProfile = {
  connection_id: string;
  display_name: string;
  family: string;
  major_version: number;
  endpoint: string;
  scope: ApiScope;
  capabilities: ApiCapabilities;
  credential_ref_id: string;
  mock: boolean;
  note: string;
};

export type ApiCapabilityOutcomeMap = Record<string, CapabilityOutcome>;

/**
 * The body the browser may send to create a connection profile.
 *
 * It carries **no** credential field: a browser-created profile is unbound.
 * Attaching a stored credential is an administrator-only operation that is
 * gated until the credential lifecycle is delivered (P2C-005), so the browser
 * cannot smuggle a reference or a value into the create request.
 */
export type ApiProfileInput = {
  connection_id: string;
  display_name: string;
  family: string;
  major_version: number;
  endpoint: string;
  scope: ApiScope;
  capabilities: ApiCapabilities;
  note: string;
};

/**
 * Validates a create body before it is sent.
 *
 * The check is a client-side guard only: the server validates again and is
 * authoritative. It refuses a body that carries a secret-like key, so a
 * credential value can never leave the browser in a registry request.
 */
export function isApiProfileInput(value: unknown): value is ApiProfileInput {
  if (!isRecord(value) || hasSecretLikeKey(value)) {
    return false;
  }
  if (
    typeof value.connection_id !== "string" ||
    typeof value.display_name !== "string" ||
    typeof value.family !== "string" ||
    typeof value.major_version !== "number" ||
    typeof value.endpoint !== "string" ||
    typeof value.note !== "string"
  ) {
    return false;
  }
  if (!isApiScope(value.scope) || !isApiCapabilities(value.capabilities)) {
    return false;
  }
  // The create body must not carry a credential reference or a credential
  // value: a browser-created profile is unbound.
  return !("credential_ref_id" in value) && !("credentialRefId" in value);
}

/**
 * One bounded, read-only capability check that was actually performed.
 *
 * The observation is the evidence behind a capability outcome: the check that
 * was attempted, the answer the endpoint gave and the reason for the outcome.
 * It never carries a credential value.
 */
export type ApiProbeObservation = {
  capability: string;
  check: string;
  method: string;
  path: string;
  status: number;
  outcome: CapabilityOutcome;
  detail: string;
};

export type ApiProbeResult = {
  connection_id: string;
  edition: string;
  query_dialects: ApiCapabilityOutcomeMap;
  write_protocols: ApiCapabilityOutcomeMap;
  schema_discovery: CapabilityOutcome;
  observations: ApiProbeObservation[];
  probed_at: string;
  mock: boolean;
};

const SECRET_LIKE_KEYS = [
  "value",
  "password",
  "passwd",
  "token",
  "apikey",
  "api_key",
  "secret",
];

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isOutcome(value: unknown): value is CapabilityOutcome {
  return typeof value === "string" && (OUTCOMES as string[]).includes(value);
}

function isOutcomeMap(value: unknown): value is Record<string, CapabilityOutcome> {
  if (!isRecord(value)) {
    return false;
  }
  return Object.values(value).every(isOutcome);
}

/**
 * Rejects any payload that carries a secret-like key anywhere in its shape.
 * A registry response must never transport a credential value, so a malformed
 * or malicious payload is refused rather than rendered.
 */
export function hasSecretLikeKey(value: unknown): boolean {
  if (Array.isArray(value)) {
    return value.some(hasSecretLikeKey);
  }
  if (!isRecord(value)) {
    return false;
  }
  return Object.entries(value).some(([key, nested]) => {
    const normalized = key.toLowerCase().replace(/-/g, "_");
    if (SECRET_LIKE_KEYS.includes(normalized)) {
      return true;
    }
    return hasSecretLikeKey(nested);
  });
}

export function isRegistryScope(value: unknown): value is RegistryScope {
  if (!isRecord(value)) {
    return false;
  }
  const kind = value.kind;
  if (
    kind !== "database-retention-policy" &&
    kind !== "organization-bucket" &&
    kind !== "database"
  ) {
    return false;
  }
  if (kind === "database-retention-policy") {
    return typeof value.database === "string" && typeof value.retentionPolicy === "string";
  }
  if (kind === "organization-bucket") {
    return typeof value.organization === "string" && typeof value.bucket === "string";
  }
  return typeof value.database === "string";
}

export function isRegistryCredentialRef(
  value: unknown,
): value is RegistryCredentialRef {
  if (value === null || value === undefined) {
    return true;
  }
  if (!isRecord(value)) {
    return false;
  }
  const role = value.role;
  return (
    typeof value.credentialRefId === "string" &&
    typeof value.alias === "string" &&
    (role === "read" || role === "write" || role === "admin") &&
    typeof value.mock === "boolean"
  );
}

export function isRegistryProfile(value: unknown): value is RegistryProfile {
  if (!isRecord(value) || hasSecretLikeKey(value)) {
    return false;
  }
  const capabilities = value.capabilities;
  if (!isRecord(capabilities)) {
    return false;
  }
  return (
    typeof value.connectionId === "string" &&
    typeof value.displayName === "string" &&
    typeof value.family === "string" &&
    typeof value.majorVersion === "number" &&
    isRegistryScope(value.scope) &&
    isOutcomeMap(capabilities.queryDialects) &&
    isOutcomeMap(capabilities.writeProtocols) &&
    isOutcome(capabilities.schemaDiscovery) &&
    isRegistryCredentialRef(value.credentialRef) &&
    typeof value.mock === "boolean" &&
    typeof value.note === "string"
  );
}

export function isRegistryProfileList(value: unknown): value is RegistryProfile[] {
  return Array.isArray(value) && value.every(isRegistryProfile);
}

function isApiScope(value: unknown): value is ApiScope {
  if (!isRecord(value)) {
    return false;
  }
  const kind = value.kind;
  if (
    kind !== "database-retention-policy" &&
    kind !== "organization-bucket" &&
    kind !== "database"
  ) {
    return false;
  }
  if (kind === "database-retention-policy") {
    return typeof value.database === "string" && typeof value.retention_policy === "string";
  }
  if (kind === "organization-bucket") {
    return typeof value.organization === "string" && typeof value.bucket === "string";
  }
  return typeof value.database === "string";
}

function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((item) => typeof item === "string");
}

/**
 * Runtime guard for the capabilities block of a live API payload.
 */
export function isApiCapabilities(value: unknown): value is ApiCapabilities {
  if (!isRecord(value)) {
    return false;
  }
  return (
    isStringArray(value.query_dialects) &&
    isStringArray(value.write_protocols) &&
    typeof value.schema_discovery === "boolean"
  );
}

/**
 * Runtime guard for a live API profile. It rejects any payload carrying a
 * secret-like key, so a credential value can never reach a feature component
 * through this path.
 */
export function isApiProfile(value: unknown): value is ApiProfile {
  if (!isRecord(value) || hasSecretLikeKey(value)) {
    return false;
  }
  const capabilities = value.capabilities;
  if (!isRecord(capabilities)) {
    return false;
  }
  return (
    typeof value.connection_id === "string" &&
    typeof value.display_name === "string" &&
    typeof value.family === "string" &&
    typeof value.major_version === "number" &&
    typeof value.endpoint === "string" &&
    isApiScope(value.scope) &&
    isStringArray(capabilities.query_dialects) &&
    isStringArray(capabilities.write_protocols) &&
    typeof capabilities.schema_discovery === "boolean" &&
    typeof value.credential_ref_id === "string" &&
    typeof value.mock === "boolean" &&
    typeof value.note === "string"
  );
}

export function isApiProfileList(value: unknown): value is ApiProfile[] {
  return Array.isArray(value) && value.every(isApiProfile);
}

function isApiObservation(value: unknown): value is ApiProbeObservation {
  if (!isRecord(value) || hasSecretLikeKey(value)) {
    return false;
  }
  return (
    typeof value.capability === "string" &&
    typeof value.check === "string" &&
    typeof value.method === "string" &&
    typeof value.path === "string" &&
    typeof value.status === "number" &&
    isOutcome(value.outcome) &&
    typeof value.detail === "string"
  );
}

export function isApiProbeResult(value: unknown): value is ApiProbeResult {
  if (!isRecord(value) || hasSecretLikeKey(value)) {
    return false;
  }
  return (
    typeof value.connection_id === "string" &&
    typeof value.edition === "string" &&
    isOutcomeMap(value.query_dialects) &&
    isOutcomeMap(value.write_protocols) &&
    isOutcome(value.schema_discovery) &&
    Array.isArray(value.observations) &&
    value.observations.every(isApiObservation) &&
    typeof value.probed_at === "string" &&
    typeof value.mock === "boolean"
  );
}

/**
 * Converts a live API profile into the browser view model.
 *
 * The live API reports positively-supported capabilities as lists; a capability
 * absent from the list is `unsupported`, which is exactly what the backend
 * observed. `unknown` is not produced here: the live prober only records
 * confirmed outcomes, and `unknown` remains reserved for a probe that could not
 * complete.
 */
export function toRegistryProfile(api: ApiProfile): RegistryProfile {
  const dialects: Record<string, CapabilityOutcome> = {};
  for (const dialect of ["influxql", "flux", "sql"]) {
    dialects[dialect] = api.capabilities.query_dialects.includes(dialect)
      ? "supported"
      : "unsupported";
  }
  const protocols: Record<string, CapabilityOutcome> = {};
  for (const protocol of ["line-protocol"]) {
    protocols[protocol] = api.capabilities.write_protocols.includes(protocol)
      ? "supported"
      : "unsupported";
  }
  const scope: RegistryScope =
    api.scope.kind === "database-retention-policy"
      ? {
          kind: "database-retention-policy",
          database: api.scope.database,
          retentionPolicy: api.scope.retention_policy,
        }
      : api.scope.kind === "organization-bucket"
        ? {
            kind: "organization-bucket",
            organization: api.scope.organization,
            bucket: api.scope.bucket,
          }
        : { kind: "database", database: api.scope.database };

  return {
    connectionId: api.connection_id,
    displayName: api.display_name,
    family: api.family,
    majorVersion: api.major_version,
    scope,
    capabilities: {
      queryDialects: dialects,
      writeProtocols: protocols,
      schemaDiscovery: api.capabilities.schema_discovery ? "supported" : "unsupported",
    },
    credentialRef: null,
    credentialRefId: api.credential_ref_id === "" ? null : api.credential_ref_id,
    mock: api.mock,
    note: api.note,
  };
}

/** Stable, human-readable scope identity. */
export function formatScope(scope: RegistryScope): string {
  switch (scope.kind) {
    case "database-retention-policy":
      return `${scope.database ?? ""}/${scope.retentionPolicy ?? ""}`;
    case "organization-bucket":
      return `${scope.organization ?? ""}/${scope.bucket ?? ""}`;
    case "database":
      return scope.database ?? "";
  }
}

/** Reports whether a capability map positively confirms a capability. */
export function outcomeOf(
  map: Record<string, CapabilityOutcome>,
  key: string,
): CapabilityOutcome {
  return map[key] ?? "unknown";
}
