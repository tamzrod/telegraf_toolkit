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
  mock: boolean;
  note: string;
};

const OUTCOMES: CapabilityOutcome[] = ["supported", "unsupported", "unknown"];

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
