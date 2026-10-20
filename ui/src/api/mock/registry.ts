import type { RegistryProfile } from "../registry";

/**
 * Test-only mock Registry adapter.
 *
 * This module exists so tests never reach the network and so the demo UI has a
 * clearly labeled, typed source. It is deliberately isolated under
 * `api/mock/` and must not be imported by application code (enforced by
 * `src/api/boundary.test.ts`); feature code depends on the typed client in
 * `api/client.ts` and the mock is swapped in by stubbing `fetch` in tests.
 *
 * Every value is synthetic. Credential fields are references only, and the
 * capability maps use the three-valued outcome vocabulary so an unsupported or
 * unknown capability can never be rendered as support.
 */

export const MOCK_REGISTRY_PROFILES: RegistryProfile[] = [
  {
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
    note: "Test-only mock. No live endpoint.",
  },
  {
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
      queryDialects: { influxql: "supported", flux: "supported", sql: "unsupported" },
      writeProtocols: { "line-protocol": "supported" },
      schemaDiscovery: "supported",
    },
    credentialRef: {
      credentialRefId: "mock-cred-write-2x",
      alias: "MOCK write 2.x",
      role: "write",
      mock: true,
    },
    mock: true,
    note: "Test-only mock. No live endpoint.",
  },
  {
    connectionId: "mock-influx-3x",
    displayName: "MOCK InfluxDB 3.x (plant-c)",
    family: "influxdb",
    majorVersion: 3,
    scope: { kind: "database", database: "telemetry_v3" },
    capabilities: {
      queryDialects: { influxql: "supported", flux: "unsupported", sql: "supported" },
      writeProtocols: { "line-protocol": "supported" },
      schemaDiscovery: "unsupported",
    },
    // Deliberately no credential reference: the UI must show an explicit
    // "no credential reference" state rather than inventing one.
    credentialRef: null,
    mock: true,
    note: "Test-only mock. Schema discovery unsupported (probed). No credential attached.",
  },
];

/** Returns a resolved JSON `Response` shaped like a registry list endpoint. */
export function registryProfilesResponse(
  body: unknown = MOCK_REGISTRY_PROFILES,
): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}
