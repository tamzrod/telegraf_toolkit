import { describe, expect, it } from "vitest";
import {
  hasSecretLikeKey,
  isApiProfile,
  isApiProfileList,
  isApiProbeResult,
  toRegistryProfile,
} from "./registry";
import type { ApiProfile } from "./registry";

const API_PROFILE: ApiProfile = {
  connection_id: "live-influx-3x",
  display_name: "Live InfluxDB 3.x",
  family: "influxdb",
  major_version: 3,
  endpoint: "http://127.0.0.1:18181",
  scope: { kind: "database", database: "telemetry_v3" },
  capabilities: {
    query_dialects: ["sql", "influxql"],
    write_protocols: ["line-protocol"],
    schema_discovery: false,
  },
  credential_ref_id: "ref-live-3x",
  mock: false,
  note: "live",
};

describe("live API registry contract", () => {
  it("accepts a valid live profile and rejects one carrying secret-like keys", () => {
    expect(isApiProfile(API_PROFILE)).toBe(true);
    expect(isApiProfileList([API_PROFILE])).toBe(true);

    const leaky = { ...API_PROFILE, password: "leaked" };
    expect(isApiProfile(leaky)).toBe(false);
    expect(hasSecretLikeKey(leaky)).toBe(true);
  });

  it("rejects a malformed profile instead of coercing it", () => {
    expect(isApiProfile({ ...API_PROFILE, major_version: "3" })).toBe(false);
    expect(
      isApiProfile({
        ...API_PROFILE,
        scope: { kind: "organization-bucket", organization: "a" },
      }),
    ).toBe(false);
  });

  it("converts a live profile to the view model with three-valued capabilities", () => {
    const profile = toRegistryProfile(API_PROFILE);
    expect(profile.connectionId).toBe("live-influx-3x");
    expect(profile.mock).toBe(false);
    expect(profile.scope).toEqual({ kind: "database", database: "telemetry_v3" });
    // Supported dialects are confirmed; the rest are reported unsupported.
    expect(profile.capabilities.queryDialects.sql).toBe("supported");
    expect(profile.capabilities.queryDialects.influxql).toBe("supported");
    expect(profile.capabilities.queryDialects.flux).toBe("unsupported");
    expect(profile.capabilities.schemaDiscovery).toBe("unsupported");
    // The credential is a reference ID, never a value.
    expect(profile.credentialRefId).toBe("ref-live-3x");
    expect(profile.credentialRef).toBeNull();
  });

  it("reports an absent credential reference as null", () => {
    const profile = toRegistryProfile({ ...API_PROFILE, credential_ref_id: "" });
    expect(profile.credentialRefId).toBeNull();
  });

  it("accepts a valid probe result and rejects a secret-bearing one", () => {
    const result = {
      connection_id: "live-influx-2x",
      edition: "influxdb-2.x-oss",
      query_dialects: { influxql: "unknown", flux: "unknown", sql: "unknown" },
      write_protocols: { "line-protocol": "unknown" },
      schema_discovery: "supported",
      observations: [
        {
          capability: "schema_discovery",
          check: "schema-v2-buckets",
          method: "GET",
          path: "/api/v2/buckets",
          status: 200,
          outcome: "supported",
          detail: "the endpoint returned the expected response",
        },
      ],
      probed_at: "2026-10-21T03:00:00Z",
      mock: false,
    };
    expect(isApiProbeResult(result)).toBe(true);
    expect(isApiProbeResult({ ...result, token: "leaked" })).toBe(false);
    expect(isApiProbeResult({ ...result, schema_discovery: "maybe" })).toBe(false);
    expect(
      isApiProbeResult({
        ...result,
        observations: [{ ...result.observations[0], outcome: "maybe" }],
      }),
    ).toBe(false);
  });
});
