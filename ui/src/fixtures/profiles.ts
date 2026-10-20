/**
 * Deterministic, test-only frontend fixtures.
 *
 * Every value is synthetic. Nothing here contacts a real database, Docker
 * daemon or Telegraf runtime. Credential fields are references only.
 */

export type ScopeKind =
  | "database-retention-policy"
  | "organization-bucket"
  | "database";

export type Scope = {
  kind: ScopeKind;
  database?: string;
  retentionPolicy?: string;
  organization?: string;
  bucket?: string;
};

export type Capabilities = {
  queryDialects: string[];
  writeProtocols: string[];
  schemaDiscovery: boolean;
};

export type ConnectionProfile = {
  connectionId: string;
  displayName: string;
  family: "influxdb";
  majorVersion: number;
  scope: Scope;
  capabilities: Capabilities;
  credentialRefId: string;
  mock: boolean;
  note: string;
};

export function scopeLabel(scope: Scope): string {
  switch (scope.kind) {
    case "database-retention-policy":
      return `${scope.database}/${scope.retentionPolicy}`;
    case "organization-bucket":
      return `${scope.organization}/${scope.bucket}`;
    case "database":
      return scope.database ?? "";
  }
}

export const profile1x: ConnectionProfile = {
  connectionId: "mock-influx-1x",
  displayName: "MOCK InfluxDB 1.x (plant-a)",
  family: "influxdb",
  majorVersion: 1,
  scope: {
    kind: "database-retention-policy",
    database: "telemetry",
    retentionPolicy: "autogen",
  },
  capabilities: {
    queryDialects: ["influxql"],
    writeProtocols: ["line-protocol"],
    schemaDiscovery: true,
  },
  credentialRefId: "mock-cred-readonly-1x",
  mock: true,
  note: "Test-only mock. No live endpoint.",
};

export const profile2x: ConnectionProfile = {
  connectionId: "mock-influx-2x",
  displayName: "MOCK InfluxDB 2.x (plant-b)",
  family: "influxdb",
  majorVersion: 2,
  scope: {
    kind: "organization-bucket",
    organization: "acme",
    bucket: "historian",
  },
  capabilities: {
    queryDialects: ["flux", "influxql"],
    writeProtocols: ["line-protocol"],
    schemaDiscovery: true,
  },
  credentialRefId: "mock-cred-write-2x",
  mock: true,
  note: "Test-only mock. No live endpoint.",
};

export const profile3x: ConnectionProfile = {
  connectionId: "mock-influx-3x",
  displayName: "MOCK InfluxDB 3.x (plant-c)",
  family: "influxdb",
  majorVersion: 3,
  scope: { kind: "database", database: "telemetry_v3" },
  capabilities: {
    queryDialects: ["sql", "influxql"],
    writeProtocols: ["line-protocol"],
    schemaDiscovery: false,
  },
  credentialRefId: "mock-cred-readonly-3x",
  mock: true,
  note: "Test-only mock. Schema discovery unsupported (probed).",
};

export const profiles: ConnectionProfile[] = [profile1x, profile2x, profile3x];

export function hasDialect(profile: ConnectionProfile, dialect: string): boolean {
  return profile.capabilities.queryDialects.includes(dialect);
}

/**
 * Enforce the contract rule that source and destination are explicit and
 * distinct before any schema browsing or mapping.
 */
export function validateSelection(
  source: ConnectionProfile | null,
  destination: ConnectionProfile | null,
): { ok: true } | { ok: false; reason: string } {
  if (!source || !destination) {
    return { ok: false, reason: "Select both a source and a destination database." };
  }
  if (
    source.connectionId === destination.connectionId &&
    scopeLabel(source.scope) === scopeLabel(destination.scope)
  ) {
    return {
      ok: false,
      reason: "Source and destination must be distinct.",
    };
  }
  return { ok: true };
}
