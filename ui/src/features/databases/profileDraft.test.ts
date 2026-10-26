import { describe, expect, it } from "vitest";
import {
  describeApiFailure,
  isCompleteScope,
  isValidEndpoint,
  parseList,
  profileInputFromDraft,
  validateProfileDraft,
} from "./registryWorkflow";
import type { ProfileDraft } from "./registryWorkflow";

const BASE: ProfileDraft = {
  connectionId: "live-influx-2x",
  displayName: "Live InfluxDB 2.x",
  family: "influxdb",
  majorVersion: "2",
  endpoint: "http://127.0.0.1:18086",
  scopeKind: "organization-bucket",
  organization: "acme",
  bucket: "historian",
  database: "",
  retentionPolicy: "",
  queryDialects: "influxql",
  writeProtocols: "",
  schemaDiscovery: false,
  note: "",
};

describe("live profile draft validation", () => {
  it("accepts a complete draft", () => {
    const result = validateProfileDraft(BASE);
    expect(result.ok).toBe(true);
    expect(result.invalid).toEqual([]);
  });

  it("marks the missing identity fields", () => {
    const result = validateProfileDraft({
      ...BASE,
      connectionId: "  ",
      displayName: "",
    });
    expect(result.ok).toBe(false);
    expect(result.invalid).toContain("connectionId");
    expect(result.invalid).toContain("displayName");
    expect(result.reason).toMatch(/connection id is required/i);
  });

  it("rejects a major version that is not a whole number", () => {
    const result = validateProfileDraft({ ...BASE, majorVersion: "2.5" });
    expect(result.ok).toBe(false);
    expect(result.invalid).toContain("majorVersion");
  });

  it("rejects an endpoint that is not an absolute http or https URL", () => {
    for (const endpoint of [
      "127.0.0.1:18086",
      "ftp://127.0.0.1",
      "http://admin:secret@127.0.0.1:18086",
      "",
    ]) {
      expect(isValidEndpoint(endpoint)).toBe(false);
      const result = validateProfileDraft({ ...BASE, endpoint });
      expect(result.invalid).toContain("endpoint");
    }
  });

  it("requires the fields the selected scope kind needs", () => {
    expect(
      validateProfileDraft({
        ...BASE,
        scopeKind: "organization-bucket",
        organization: "",
      }).invalid,
    ).toContain("scope");
    expect(
      validateProfileDraft({
        ...BASE,
        scopeKind: "database",
        database: "",
      }).invalid,
    ).toContain("scope");
    expect(
      validateProfileDraft({
        ...BASE,
        scopeKind: "database-retention-policy",
        database: "",
        retentionPolicy: "",
      }).invalid,
    ).toContain("scope");
  });

  it("accepts a complete scope for every kind", () => {
    expect(
      isCompleteScope({ ...BASE, scopeKind: "organization-bucket" }),
    ).toBe(true);
    expect(isCompleteScope({ ...BASE, scopeKind: "database", database: "telemetry" })).toBe(
      true,
    );
    expect(
      isCompleteScope({
        ...BASE,
        scopeKind: "database-retention-policy",
        database: "telemetry",
        retentionPolicy: "autogen",
      }),
    ).toBe(true);
  });

  it("requires at least one query dialect", () => {
    const result = validateProfileDraft({ ...BASE, queryDialects: " , " });
    expect(result.invalid).toContain("capabilities");
  });

  it("builds a request body that carries no credential", () => {
    const input = profileInputFromDraft({
      ...BASE,
      queryDialects: "influxql, flux",
      writeProtocols: "line-protocol",
      note: " created in the browser ",
    });
    expect(input.connection_id).toBe("live-influx-2x");
    expect(input.scope).toEqual({
      kind: "organization-bucket",
      organization: "acme",
      bucket: "historian",
    });
    expect(input.capabilities.query_dialects).toEqual(["influxql", "flux"]);
    expect(input.capabilities.write_protocols).toEqual(["line-protocol"]);
    expect(input.note).toBe("created in the browser");
    expect(Object.keys(input)).not.toContain("credential_ref_id");
  });

  it("splits a comma separated list and drops empty entries", () => {
    expect(parseList(" influxql , flux ,, ")).toEqual(["influxql", "flux"]);
    expect(parseList("   ")).toEqual([]);
  });
});

describe("API failure descriptions", () => {
  it("maps the status to an actionable, credential-free message", () => {
    expect(describeApiFailure({ status: 401 })).toMatch(/session token/i);
    expect(describeApiFailure({ status: 403 })).toMatch(/role is not permitted/i);
    expect(describeApiFailure({ status: 404 })).toMatch(/no longer present/i);
    expect(describeApiFailure({ status: 409 })).toMatch(/already exists/i);
    expect(describeApiFailure({ status: 0 })).toMatch(/could not be reached/i);
    expect(describeApiFailure({ status: 500 })).toMatch(/could not complete/i);
    expect(describeApiFailure(new Error("boom"))).toMatch(/could not be reached/i);
  });

  it("never repeats a backend body", () => {
    const message = describeApiFailure({
      status: 400,
      message: "token=secret-value",
    });
    expect(message).not.toContain("secret-value");
  });
});
