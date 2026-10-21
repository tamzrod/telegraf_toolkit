import { describe, expect, it } from "vitest";
import type { ApiProbeResult } from "../../api/registry";
import {
  observationsFor,
  outcomeLabel,
  probeCapabilityRows,
} from "./registryWorkflow";

const PROBE_RESULT: ApiProbeResult = {
  connection_id: "live-influx-2x",
  edition: "influxdb-2.x-oss",
  query_dialects: { influxql: "unknown", flux: "unknown", sql: "unknown" },
  write_protocols: { "line-protocol": "unknown" },
  schema_discovery: "supported",
  observations: [
    {
      capability: "influxql",
      check: "influxql-v2-query",
      method: "GET",
      path: "/api/v2/query?q=SELECT%201",
      status: 405,
      outcome: "unknown",
      detail: "the endpoint does not serve this method",
    },
    {
      capability: "flux",
      check: "flux-query",
      method: "GET",
      path: "",
      status: 0,
      outcome: "unknown",
      detail: "not attempted: this check needs a request body",
    },
    {
      capability: "schema_discovery",
      check: "schema-v2-buckets",
      method: "GET",
      path: "/api/v2/buckets",
      status: 200,
      outcome: "supported",
      detail: "the endpoint returned the expected response",
    },
    {
      capability: "line-protocol",
      check: "line-protocol-write",
      method: "GET",
      path: "",
      status: 0,
      outcome: "unknown",
      detail: "not attempted: a read-only probe never writes to the target",
    },
  ],
  probed_at: "2026-10-21T03:00:00Z",
  mock: false,
};

describe("live probe result presentation", () => {
  it("keeps the three states: unknown is never shown as supported or unsupported", () => {
    const rows = probeCapabilityRows(PROBE_RESULT);
    const byLabel = new Map(rows.map((row) => [row.label, row]));

    expect(byLabel.get("Query: influxql")?.outcome).toBe("unknown");
    expect(byLabel.get("Query: flux")?.outcome).toBe("unknown");
    expect(byLabel.get("Query: sql")?.outcome).toBe("unknown");
    expect(byLabel.get("Write: line-protocol")?.outcome).toBe("unknown");
    expect(byLabel.get("Schema discovery")?.outcome).toBe("supported");

    // Only a confirmed capability may be marked as confirmed.
    for (const row of rows) {
      expect(row.confirmed).toBe(row.outcome === "supported");
    }
  });

  it("labels an unknown outcome as not probed", () => {
    expect(outcomeLabel("unknown")).toBe("Unknown (not probed)");
    expect(outcomeLabel("supported")).toBe("Supported");
    expect(outcomeLabel("unsupported")).toBe("Unsupported");
  });

  it("exposes the observations that back a capability", () => {
    const observations = observationsFor(PROBE_RESULT, "schema_discovery");
    expect(observations).toHaveLength(1);
    expect(observations[0].check).toBe("schema-v2-buckets");
    expect(observations[0].status).toBe(200);

    const write = observationsFor(PROBE_RESULT, "line-protocol");
    expect(write).toHaveLength(1);
    expect(write[0].detail).toContain("never writes");
  });
});
