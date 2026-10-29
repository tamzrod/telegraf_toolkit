import type { ApiProfile, ApiSchemaResult } from "../registry";

/**
 * Test-only fixtures for the Canonical Mapper schema browser.
 *
 * The payloads are shaped exactly like the authorized P3-002 responses so the
 * typed guards, stale checks and state rendering are exercised without a real
 * database. No value here is a credential; the reference IDs are opaque and are
 * deliberately absent from the page assertions.
 */

export const LIVE_REGISTRY_PROFILES: ApiProfile[] = [
  {
    connection_id: "live-influx-2x-a",
    display_name: "LIVE InfluxDB 2.x A",
    family: "influxdb",
    major_version: 2,
    endpoint: "https://influx-a.example:8086",
    scope: {
      kind: "organization-bucket",
      organization: "ttk-test-org",
      bucket: "ttk-test-bucket",
    },
    capabilities: {
      query_dialects: ["influxql"],
      write_protocols: ["line-protocol"],
      schema_discovery: true,
    },
    credential_ref_id: "ref-2x-a",
    mock: false,
    note: "Test-only live profile. The server resolves the credential.",
  },
  {
    connection_id: "live-influx-2x-b",
    display_name: "LIVE InfluxDB 2.x B",
    family: "influxdb",
    major_version: 2,
    endpoint: "https://influx-b.example:8086",
    scope: {
      kind: "organization-bucket",
      organization: "ttk-test-org",
      bucket: "other-bucket",
    },
    capabilities: {
      query_dialects: ["influxql"],
      write_protocols: ["line-protocol"],
      schema_discovery: true,
    },
    credential_ref_id: "ref-2x-b",
    mock: false,
    note: "Test-only live profile. The server resolves the credential.",
  },
  {
    connection_id: "live-influx-1x",
    display_name: "LIVE InfluxDB 1.x",
    family: "influxdb",
    major_version: 1,
    endpoint: "http://influx-1x.example:8086",
    scope: {
      kind: "database-retention-policy",
      database: "telemetry",
      retention_policy: "autogen",
    },
    capabilities: {
      query_dialects: ["influxql"],
      write_protocols: ["line-protocol"],
      schema_discovery: true,
    },
    credential_ref_id: "ref-1x",
    mock: false,
    note: "Test-only live profile. 1.x metadata remains unverified.",
  },
];

export function liveProfilesResponse(): Response {
  return new Response(JSON.stringify(LIVE_REGISTRY_PROFILES), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

const BASE_OBSERVATION = {
  check: "show-measurements",
  method: "GET",
  path: "/query?db=ttk-test-bucket&q=SHOW+MEASUREMENTS",
  status: 200,
  outcome: "supported" as const,
  edition: "influxdb-2.x-oss",
  detail: "Observed the deployment's measurement names.",
};

/** Builds one valid P3-002 schema payload with optional overrides. */
export function makeSchemaResult(
  overrides: Partial<ApiSchemaResult> = {},
): ApiSchemaResult {
  return {
    connection_id: "live-influx-2x-a",
    scope: "ttk-test-org/ttk-test-bucket",
    edition: "influxdb-2.x-oss",
    kind: "measurements",
    outcome: "supported",
    measurements: ["environment", "sensor"],
    fields: [],
    tags: [],
    observations: [BASE_OBSERVATION],
    truncated: false,
    requests: 2,
    read_at: "2026-10-26T00:00:00Z",
    mock: false,
    ...overrides,
  };
}

export function schemaResponse(body: unknown = makeSchemaResult()): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

export function fieldsSchemaResult(
  overrides: Partial<ApiSchemaResult> = {},
): ApiSchemaResult {
  return makeSchemaResult({
    kind: "fields",
    measurement: "sensor",
    measurements: [],
    fields: [
      { name: "temperature", type: "float" },
      { name: "value", type: "integer" },
    ],
    observations: [
      {
        check: "show-field-keys",
        method: "GET",
        path: '/query?db=ttk-test-bucket&q=SHOW+FIELD+KEYS+FROM+%22sensor%22',
        status: 200,
        outcome: "supported",
        edition: "influxdb-2.x-oss",
        detail: "Observed field keys after verifying measurement membership.",
      },
    ],
    ...overrides,
  });
}

export function tagsSchemaResult(
  overrides: Partial<ApiSchemaResult> = {},
): ApiSchemaResult {
  return makeSchemaResult({
    kind: "tags",
    measurement: "sensor",
    measurements: [],
    tags: ["host", "sensor_id"],
    observations: [
      {
        check: "show-tag-keys",
        method: "GET",
        path: '/query?db=ttk-test-bucket&q=SHOW+TAG+KEYS+FROM+%22sensor%22',
        status: 200,
        outcome: "supported",
        edition: "influxdb-2.x-oss",
        detail: "Observed tag keys after verifying measurement membership.",
      },
    ],
    ...overrides,
  });
}
