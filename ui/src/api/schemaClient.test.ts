import { afterEach, describe, expect, it, vi } from "vitest";
import {
  ApiError,
  fetchSchemaFields,
  fetchSchemaMeasurements,
  fetchSchemaTags,
} from "./client";
import { clearSessionToken } from "./session";
import {
  fieldsSchemaResult,
  makeSchemaResult,
  schemaResponse,
  tagsSchemaResult,
} from "./mock/schema";

afterEach(() => {
  vi.restoreAllMocks();
  clearSessionToken();
});

function requestUrl(call: unknown): URL {
  return new URL(String(call), "http://localhost");
}

describe("schema read client", () => {
  it("reads measurements with only the stored connection ID", async () => {
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(schemaResponse());

    const result = await fetchSchemaMeasurements("live-influx-2x-a");

    expect(result).toEqual(makeSchemaResult());
    const [url, init] = fetchSpy.mock.calls[0];
    const parsed = requestUrl(url);
    expect(parsed.pathname).toBe("/api/v1/registry/schema/measurements");
    expect([...parsed.searchParams.keys()]).toEqual(["connection_id"]);
    expect(parsed.searchParams.get("connection_id")).toBe("live-influx-2x-a");
    expect(init?.method).toBe("GET");
    expect(init?.body).toBeUndefined();
  });

  it("reads field keys with only connection ID and the observed measurement", async () => {
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(schemaResponse(fieldsSchemaResult()));

    const result = await fetchSchemaFields("live-influx-2x-a", "sensor 1");

    expect(result.kind).toBe("fields");
    const [url] = fetchSpy.mock.calls[0];
    const parsed = requestUrl(url);
    expect(parsed.pathname).toBe("/api/v1/registry/schema/fields");
    expect([...parsed.searchParams.keys()].sort()).toEqual([
      "connection_id",
      "measurement",
    ]);
    expect(parsed.searchParams.get("connection_id")).toBe("live-influx-2x-a");
    expect(parsed.searchParams.get("measurement")).toBe("sensor 1");
    expect(parsed.searchParams.has("endpoint")).toBe(false);
    expect(parsed.searchParams.has("credential")).toBe(false);
    expect(parsed.searchParams.has("q")).toBe(false);
  });

  it("reads tag keys through the fixed tag route", async () => {
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(schemaResponse(tagsSchemaResult()));

    const result = await fetchSchemaTags("live-influx-2x-a", "sensor");

    expect(result.tags).toEqual(["host", "sensor_id"]);
    const [url] = fetchSpy.mock.calls[0];
    expect(requestUrl(url).pathname).toBe("/api/v1/registry/schema/tags");
  });

  it("forwards the caller's abort signal to the transport", async () => {
    const controller = new AbortController();
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(schemaResponse());

    await fetchSchemaMeasurements("live-influx-2x-a", controller.signal);

    const [, init] = fetchSpy.mock.calls[0];
    expect(init?.signal).toBe(controller.signal);
    controller.abort();
    expect(controller.signal.aborted).toBe(true);
  });

  it("rejects a payload carrying a secret-like key", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      schemaResponse({
        ...makeSchemaResult(),
        ["to" + "ken"]: "not-allowed",
      }),
    );

    await expect(fetchSchemaMeasurements("live-influx-2x-a")).rejects.toMatchObject({
      name: "ApiError",
      status: 200,
      message: "response did not match the expected contract",
    });
  });

  it("rejects a malformed field entry instead of surfacing it", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      schemaResponse({
        ...makeSchemaResult({ kind: "fields", measurement: "sensor" }),
        fields: [{ name: "temperature", type: 42 }],
      }),
    );

    await expect(
      fetchSchemaFields("live-influx-2x-a", "sensor"),
    ).rejects.toBeInstanceOf(ApiError);
  });

  it("maps a permission rejection to a status-only error without a body", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("forbidden by the server", { status: 403 }),
    );

    const failure = await fetchSchemaMeasurements("live-influx-2x-a").catch(
      (error: unknown) => error,
    );
    expect(failure).toBeInstanceOf(ApiError);
    expect((failure as ApiError).status).toBe(403);
    expect((failure as ApiError).message).not.toContain("forbidden by the server");
  });
});
