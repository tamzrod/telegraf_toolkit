import { describe, expect, it } from "vitest";
import {
  capabilityRows,
  credentialState,
  isConfirmed,
  outcomeLabel,
  validateDraft,
  validateSelection,
} from "./registryWorkflow";
import type { RegistryProfile } from "../../api/registry";

const baseProfile: RegistryProfile = {
  connectionId: "mock-influx-1x",
  displayName: "MOCK InfluxDB 1.x",
  family: "influxdb",
  majorVersion: 1,
  scope: {
    kind: "database-retention-policy",
    database: "telemetry",
    retentionPolicy: "autogen",
  },
  capabilities: {
    queryDialects: { influxql: "supported", flux: "unsupported", sql: "unsupported" },
    writeProtocols: { "line-protocol": "supported" },
    schemaDiscovery: "supported",
  },
  credentialRef: {
    credentialRefId: "mock-cred-readonly-1x",
    alias: "MOCK read-only 1.x",
    role: "read",
    mock: true,
  },
  mock: true,
  note: "Test-only mock.",
};

describe("validateSelection", () => {
  it("requires both sides", () => {
    expect(validateSelection(null, baseProfile).ok).toBe(false);
    expect(validateSelection(baseProfile, null).ok).toBe(false);
  });

  it("rejects a non-distinct source and destination", () => {
    const result = validateSelection(baseProfile, { ...baseProfile });
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.reason).toMatch(/distinct/i);
    }
  });

  it("accepts a distinct pair", () => {
    const other: RegistryProfile = {
      ...baseProfile,
      connectionId: "mock-influx-2x",
      scope: { kind: "organization-bucket", organization: "acme", bucket: "historian" },
    };
    expect(validateSelection(baseProfile, other).ok).toBe(true);
  });
});

describe("capability presentation", () => {
  it("treats only a confirmed outcome as support", () => {
    expect(isConfirmed("supported")).toBe(true);
    expect(isConfirmed("unsupported")).toBe(false);
    expect(isConfirmed("unknown")).toBe(false);
  });

  it("labels unknown distinctly from unsupported", () => {
    expect(outcomeLabel("unknown")).not.toBe(outcomeLabel("unsupported"));
    expect(outcomeLabel("unknown")).toMatch(/not probed/i);
  });

  it("marks unsupported and unknown capabilities as unconfirmed rows", () => {
    const profile: RegistryProfile = {
      ...baseProfile,
      capabilities: {
        queryDialects: { influxql: "unknown", flux: "unsupported", sql: "supported" },
        writeProtocols: { "line-protocol": "supported" },
        schemaDiscovery: "unsupported",
      },
    };
    const rows = capabilityRows(profile);
    const influxql = rows.find((row) => row.label === "Query: influxql");
    const schema = rows.find((row) => row.label === "Schema discovery");
    expect(influxql?.confirmed).toBe(false);
    expect(influxql?.outcome).toBe("unknown");
    expect(schema?.confirmed).toBe(false);
  });
});

describe("credentialState", () => {
  it("reports an attached reference without a value", () => {
    const state = credentialState(baseProfile);
    expect(state.kind).toBe("reference");
    if (state.kind === "reference") {
      expect(state.credentialRefId).toBe("mock-cred-readonly-1x");
      expect(state).not.toHaveProperty("value");
    }
  });

  it("reports an absent reference explicitly", () => {
    const state = credentialState({ ...baseProfile, credentialRef: null });
    expect(state.kind).toBe("absent");
  });
});

describe("validateDraft", () => {
  it("requires a connection id and display name", () => {
    expect(
      validateDraft({
        connectionId: "  ",
        displayName: "x",
        scope: { kind: "database", database: "d" },
      }).ok,
    ).toBe(false);
    expect(
      validateDraft({
        connectionId: "id",
        displayName: " ",
        scope: { kind: "database", database: "d" },
      }).ok,
    ).toBe(false);
  });

  it("requires a kind-complete scope", () => {
    expect(
      validateDraft({
        connectionId: "id",
        displayName: "name",
        scope: { kind: "organization-bucket", organization: "acme" },
      }).ok,
    ).toBe(false);
    expect(
      validateDraft({
        connectionId: "id",
        displayName: "name",
        scope: { kind: "database-retention-policy", database: "db" },
      }).ok,
    ).toBe(false);
  });

  it("accepts a complete draft", () => {
    expect(
      validateDraft({
        connectionId: "id",
        displayName: "name",
        scope: { kind: "database", database: "db" },
      }).ok,
    ).toBe(true);
  });
});
