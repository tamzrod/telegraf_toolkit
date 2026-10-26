import { formatScope } from "../../api/registry";
import type { ApiSchemaResult, RegistryProfile } from "../../api/registry";

/**
 * Explicit, synthetic schema preview for the labeled Mock mode.
 *
 * This is **not** a fallback and is never imported by a live path. It exists so
 * the schema-browser layout can be exercised against the typed mock profile
 * list without contacting a database. Every result is marked `mock: true`, its
 * observations say that no request was made, and the page labels the whole
 * panel MOCK.
 *
 * An InfluxDB 1.x mock profile is deliberately `unknown`: live 1.x metadata is
 * unverified, and a synthetic preview must not imply otherwise.
 */
export function mockSchemaPreview(profile: RegistryProfile): ApiSchemaResult {
  const scope = formatScope(profile.scope);
  const base = {
    connection_id: profile.connectionId,
    scope,
    kind: "measurements" as const,
    measurements: [] as string[],
    fields: [] as { name: string; type: string }[],
    tags: [] as string[],
    truncated: false,
    requests: 0,
    read_at: "mock",
    mock: true,
  };

  if (profile.majorVersion === 1) {
    return {
      ...base,
      edition: "unknown",
      outcome: "unknown",
      observations: [
        {
          check: "mock-1x-unverified",
          method: "none",
          path: "",
          status: 0,
          outcome: "unknown",
          detail:
            "Synthetic preview: InfluxDB 1.x metadata is unverified and no request is made.",
        },
      ],
    };
  }

  if (profile.capabilities.schemaDiscovery !== "supported") {
    return {
      ...base,
      edition: "unknown",
      outcome: profile.capabilities.schemaDiscovery,
      observations: [
        {
          check: "mock-schema-capability",
          method: "none",
          path: "",
          status: 0,
          outcome: profile.capabilities.schemaDiscovery,
          detail: `Synthetic preview: the mock profile reports schema discovery as ${profile.capabilities.schemaDiscovery}; no data is shown.`,
        },
      ],
    };
  }

  return {
    ...base,
    edition: `mock-influxdb-${profile.majorVersion}.x`,
    outcome: "supported",
    measurements: ["environment", "sensor"],
    fields: [
      { name: "humidity", type: "float" },
      { name: "temperature", type: "float" },
      { name: "value", type: "integer" },
    ],
    tags: ["host", "sensor_id"],
    observations: [
      {
        check: "mock-schema-preview",
        method: "none",
        path: "",
        status: 0,
        outcome: "supported",
        edition: `mock-influxdb-${profile.majorVersion}.x`,
        detail: "Synthetic preview only; no request was made.",
      },
    ],
  };
}
