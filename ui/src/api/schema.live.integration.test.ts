import { describe, expect, it } from "vitest";
import {
  fetchLiveRegistryProfiles,
  fetchSchemaFields,
  fetchSchemaMeasurements,
  fetchSchemaTags,
} from "./client";
import { formatScope } from "./registry";
import { clearSessionToken, setSessionToken } from "./session";

/**
 * Authenticated real-backend smoke for the P3-003 schema client.
 *
 * This test runs only when the isolated harness supplies a live API URL, an
 * admin token and the persisted profile ID. The shell script that starts the
 * API exports those values; the token is read from the environment and is
 * never printed. In normal `npm test` runs the suite is skipped, so the default
 * test path stays hermetic.
 */
const env = (
  globalThis as unknown as {
    process?: { env?: Record<string, string | undefined> };
  }
).process?.env ?? {};

const liveBase = env.TTK_LIVE_API_URL ?? "";
const liveToken = env.TTK_LIVE_API_TOKEN ?? "";
const liveProfileId = env.TTK_LIVE_PROFILE_ID ?? "";
const describeLive =
  liveBase !== "" && liveToken !== "" && liveProfileId !== ""
    ? describe
    : describe.skip;

describeLive("authenticated live schema client smoke", () => {
  it("reads the real profile, measurements, field keys and tag keys", async () => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      const url =
        typeof input === "string" && input.startsWith("/")
          ? `${liveBase}${input}`
          : input;
      return originalFetch(url, init);
    }) as typeof fetch;

    setSessionToken(liveToken);
    try {
      const profiles = await fetchLiveRegistryProfiles();
      const profile = profiles.find(
        (candidate) => candidate.connectionId === liveProfileId,
      );
      expect(profile).toBeDefined();
      if (!profile) {
        throw new Error("the live profile was not returned by the API");
      }

      const measurements = await fetchSchemaMeasurements(liveProfileId);
      expect(measurements.connection_id).toBe(liveProfileId);
      expect(measurements.scope).toBe(formatScope(profile.scope));
      expect(measurements.outcome).toBe("supported");
      expect(measurements.mock).toBe(false);
      expect(measurements.measurements).toContain("environment");
      expect(measurements.measurements).toContain("sensor");

      const fields = await fetchSchemaFields(liveProfileId, "sensor");
      expect(fields.kind).toBe("fields");
      expect(fields.measurement).toBe("sensor");
      expect(fields.connection_id).toBe(liveProfileId);
      expect(fields.mock).toBe(false);
      expect(fields.fields.map((field) => field.name)).toContain("value");

      const tags = await fetchSchemaTags(liveProfileId, "sensor");
      expect(tags.kind).toBe("tags");
      expect(tags.measurement).toBe("sensor");
      expect(tags.connection_id).toBe(liveProfileId);
      expect(tags.mock).toBe(false);
      expect(tags.tags).toContain("host");
      expect(tags.tags).toContain("sensor_id");
    } finally {
      clearSessionToken();
      globalThis.fetch = originalFetch;
    }
  });
});
