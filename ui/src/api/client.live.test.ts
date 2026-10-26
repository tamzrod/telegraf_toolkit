import { describe, expect, it, vi, afterEach } from "vitest";
import {
  ApiError,
  createLiveProfile,
  deleteLiveProfile,
  fetchLiveRegistryProfile,
} from "./client";
import type { ApiProfile, ApiProfileInput } from "./registry";

const PROFILE: ApiProfile = {
  connection_id: "live-influx-2x",
  display_name: "Live InfluxDB 2.x",
  family: "influxdb",
  major_version: 2,
  endpoint: "http://127.0.0.1:18086",
  scope: { kind: "organization-bucket", organization: "acme", bucket: "historian" },
  capabilities: {
    query_dialects: ["influxql"],
    write_protocols: [],
    schema_discovery: false,
  },
  credential_ref_id: "",
  mock: false,
  note: "",
};

const INPUT: ApiProfileInput = {
  connection_id: "live-influx-2x",
  display_name: "Live InfluxDB 2.x",
  family: "influxdb",
  major_version: 2,
  endpoint: "http://127.0.0.1:18086",
  scope: { kind: "organization-bucket", organization: "acme", bucket: "historian" },
  capabilities: {
    query_dialects: ["influxql"],
    write_protocols: [],
    schema_discovery: false,
  },
  note: "",
};

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("live profile client", () => {
  it("creates an unbound profile and returns the stored profile", async () => {
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse(PROFILE));

    const created = await createLiveProfile(INPUT);

    expect(created.connection_id).toBe("live-influx-2x");
    const [url, init] = fetchSpy.mock.calls[0];
    expect(String(url)).toBe("/api/v1/registry/profiles");
    expect((init as RequestInit).method).toBe("POST");
    const body = JSON.parse(String((init as RequestInit).body));
    // The create body carries no credential reference and no credential value.
    expect(body.credential_ref_id).toBeUndefined();
    expect(Object.keys(body)).not.toContain("credential_ref_id");
  });

  it("refuses to send a body that carries a credential field", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch");
    const smuggled = { ...INPUT, credential_ref_id: "ref-1" } as unknown as ApiProfileInput;

    await expect(createLiveProfile(smuggled)).rejects.toBeInstanceOf(ApiError);
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  it("reports the status of a rejected create without echoing the body", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      jsonResponse({ code: "duplicate-profile", message: "exists" }, 409),
    );

    const error = await createLiveProfile(INPUT).catch((e: unknown) => e);
    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).status).toBe(409);
    expect((error as ApiError).message).not.toContain("exists");
  });

  it("reports a 401 and a 403 with their status", async () => {
    for (const status of [401, 403]) {
      vi.spyOn(globalThis, "fetch").mockResolvedValue(
        jsonResponse({ code: "denied", message: "no" }, status),
      );
      const error = await createLiveProfile(INPUT).catch((e: unknown) => e);
      expect((error as ApiError).status).toBe(status);
    }
  });

  it("reports a non-JSON error body as a transport failure, not as a success", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("<html>not json</html>", {
        status: 200,
        headers: { "Content-Type": "text/html" },
      }),
    );

    await expect(createLiveProfile(INPUT)).rejects.toBeInstanceOf(ApiError);
  });

  it("reads one profile by connection ID", async () => {
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse(PROFILE));

    const profile = await fetchLiveRegistryProfile("live-influx-2x");

    expect(profile.connection_id).toBe("live-influx-2x");
    expect(String(fetchSpy.mock.calls[0][0])).toBe(
      "/api/v1/registry/profiles/live-influx-2x",
    );
  });

  it("deletes a profile with a DELETE request", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(null, { status: 204 }),
    );

    await deleteLiveProfile("live-influx-2x");

    const [url, init] = fetchSpy.mock.calls[0];
    expect(String(url)).toBe("/api/v1/registry/profiles/live-influx-2x");
    expect((init as RequestInit).method).toBe("DELETE");
  });

  it("reports a refused delete with its status", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      jsonResponse({ code: "permission-denied", message: "no" }, 403),
    );

    const error = await deleteLiveProfile("live-influx-2x").catch((e: unknown) => e);
    expect((error as ApiError).status).toBe(403);
  });
});
