import { describe, expect, it } from "vitest";
import {
  hasDialect,
  profile1x,
  profile2x,
  profile3x,
  profiles,
  scopeLabel,
  validateSelection,
} from "./profiles";

describe("mock connection profiles", () => {
  it("labels every profile as mock and test-only", () => {
    for (const profile of profiles) {
      expect(profile.mock).toBe(true);
      expect(profile.displayName.startsWith("MOCK")).toBe(true);
      expect(profile.note).toMatch(/test-only/i);
    }
  });

  it("contains no secret-like material in fixtures", () => {
    const blob = JSON.stringify(profiles).toLowerCase();
    for (const forbidden of ["password", "token", "apikey", "api_key", "bearer "]) {
      expect(blob).not.toContain(forbidden);
    }
  });

  it("exposes distinct logical scopes per family", () => {
    expect(scopeLabel(profile1x.scope)).toBe("telemetry/autogen");
    expect(scopeLabel(profile2x.scope)).toBe("acme/historian");
    expect(scopeLabel(profile3x.scope)).toBe("telemetry_v3");
  });

  it("reports capabilities per connection, not per version", () => {
    expect(hasDialect(profile1x, "influxql")).toBe(true);
    expect(hasDialect(profile3x, "influxql")).toBe(true);
    expect(profile2x.capabilities.schemaDiscovery).toBe(true);
    expect(profile3x.capabilities.schemaDiscovery).toBe(false);
  });
});

describe("source/destination selection", () => {
  it("requires both sides to be selected", () => {
    expect(validateSelection(null, profile2x).ok).toBe(false);
    expect(validateSelection(profile1x, null).ok).toBe(false);
  });

  it("rejects a source and destination that are not distinct", () => {
    const result = validateSelection(profile1x, profile1x);
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.reason).toMatch(/distinct/i);
    }
  });

  it("accepts distinct source and destination", () => {
    expect(validateSelection(profile1x, profile2x).ok).toBe(true);
    expect(validateSelection(profile1x, profile3x).ok).toBe(true);
  });
});
