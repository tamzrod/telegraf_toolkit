import { describe, expect, it } from "vitest";
import { MOCK_REGISTRY_PROFILES } from "../../api/mock/registry";
import { ApiError } from "../../api/client";
import { makeSchemaResult } from "../../api/mock/schema";
import {
  describeSchemaFailure,
  isSchemaSupported,
  profileIdentity,
  schemaOutcomeLabel,
  schemaResultProblem,
  validateMapperSelection,
} from "./canonicalWorkflow";
import { mockSchemaPreview } from "./mockSchemaPreview";

const source = MOCK_REGISTRY_PROFILES[1];
const destination = MOCK_REGISTRY_PROFILES[2];
const oneX = MOCK_REGISTRY_PROFILES[0];

describe("canonical mapper selection", () => {
  it("requires both sides before any browsing", () => {
    expect(validateMapperSelection(null, null)).toEqual({
      ok: false,
      reason: "Select both a source and a destination database profile.",
    });
    expect(validateMapperSelection(source, null)).toMatchObject({ ok: false });
  });

  it("refuses the same profile on both sides", () => {
    expect(validateMapperSelection(source, source)).toEqual({
      ok: false,
      reason: "Source and destination must be distinct.",
    });
  });

  it("accepts explicit distinct profiles", () => {
    expect(validateMapperSelection(source, destination)).toEqual({ ok: true });
  });

  it("includes the canonical scope in the profile identity", () => {
    expect(profileIdentity(source)).toBe("mock-influx-2x::acme/historian");
  });
});

describe("schema answer freshness", () => {
  const result = makeSchemaResult({
    connection_id: "mock-influx-2x",
    scope: "acme/historian",
    kind: "measurements",
    mock: false,
  });

  it("accepts an answer for the selected profile, scope and kind", () => {
    expect(schemaResultProblem(source, "measurements", "", result)).toBeNull();
  });

  it("rejects an answer for a different connection profile", () => {
    expect(
      schemaResultProblem(source, "measurements", "", {
        ...result,
        connection_id: "another-profile",
      }),
    ).toBe("The response is for a different connection profile.");
  });

  it("rejects an answer for a different scope", () => {
    expect(
      schemaResultProblem(source, "measurements", "", {
        ...result,
        scope: "acme/another-bucket",
      }),
    ).toBe("The response is for a different database scope.");
  });

  it("rejects an answer for a different metadata kind", () => {
    expect(
      schemaResultProblem(source, "fields", "sensor", {
        ...result,
        kind: "measurements",
      }),
    ).toBe("The response is for a different metadata read.");
  });

  it("rejects a detail answer for a different measurement", () => {
    const fields = makeSchemaResult({
      connection_id: "mock-influx-2x",
      scope: "acme/historian",
      kind: "fields",
      measurement: "environment",
      mock: false,
    });
    expect(schemaResultProblem(source, "fields", "sensor", fields)).toBe(
      "The response is for a different measurement.",
    );
  });

  it("rejects a response marked mock when a live answer is expected", () => {
    expect(
      schemaResultProblem(source, "measurements", "", {
        ...result,
        mock: true,
      }),
    ).toBe("The response is marked as mock data and is not a live observation.");
  });
});

describe("schema failure wording", () => {
  it("names authentication, permission and missing targets without a body", () => {
    expect(describeSchemaFailure(new ApiError("x", { status: 401, url: "/" }))).toMatch(
      /session token/i,
    );
    expect(describeSchemaFailure(new ApiError("x", { status: 403, url: "/" }))).toMatch(
      /not permitted.*schema\.read/i,
    );
    expect(describeSchemaFailure(new ApiError("x", { status: 404, url: "/" }))).toMatch(
      /no longer has/i,
    );
    expect(describeSchemaFailure(new ApiError("x", { status: 0, url: "/" }))).toMatch(
      /could not be reached/i,
    );
    expect(describeSchemaFailure(new ApiError("x", { status: 500, url: "/" }))).toMatch(
      /no supported result/i,
    );
  });

  it("never calls an unknown outcome supported", () => {
    expect(isSchemaSupported("unknown")).toBe(false);
    expect(isSchemaSupported("unsupported")).toBe(false);
    expect(isSchemaSupported("supported")).toBe(true);
    expect(schemaOutcomeLabel("unknown")).toBe("Unknown");
  });
});

describe("labeled mock schema preview", () => {
  it("marks InfluxDB 1.x as unknown and makes no supported claim", () => {
    const preview = mockSchemaPreview(oneX);
    expect(preview.mock).toBe(true);
    expect(preview.outcome).toBe("unknown");
    expect(preview.measurements).toEqual([]);
    expect(preview.observations[0].detail).toMatch(/unverified/i);
  });

  it("provides typed synthetic groups for a 2.x preview, clearly marked mock", () => {
    const preview = mockSchemaPreview(source);
    expect(preview.mock).toBe(true);
    expect(preview.outcome).toBe("supported");
    expect(preview.measurements).toEqual(["environment", "sensor"]);
    expect(preview.fields.map((field) => field.name)).toEqual([
      "humidity",
      "temperature",
      "value",
    ]);
    expect(preview.tags).toEqual(["host", "sensor_id"]);
    expect(preview.requests).toBe(0);
  });

  it("honors a mock profile that reports schema discovery as unsupported", () => {
    const preview = mockSchemaPreview(destination);
    expect(preview.outcome).toBe("unsupported");
    expect(preview.measurements).toEqual([]);
  });
});
