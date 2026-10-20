import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, API_BASE, fetchHealth, requestJson } from "./client";
import { MOCK_HEALTH, healthResponse } from "./mock/health";
import { isHealth } from "../types/health";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("typed API client", () => {
  it("requests the versioned health path and returns a typed payload", async () => {
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(healthResponse());

    const health = await fetchHealth();

    expect(health).toEqual(MOCK_HEALTH);
    expect(fetchSpy).toHaveBeenCalledTimes(1);
    const [url, init] = fetchSpy.mock.calls[0];
    expect(String(url)).toBe(`${API_BASE}/health`);
    expect(init?.method).toBe("GET");
  });

  it("forwards the abort signal to fetch for cancellation", async () => {
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(healthResponse());
    const controller = new AbortController();

    await fetchHealth(controller.signal);

    const [, init] = fetchSpy.mock.calls[0];
    expect(init?.signal).toBe(controller.signal);
  });

  it("rejects with a typed ApiError carrying the status and no body", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("internal detail that must not surface", { status: 503 }),
    );

    const error = await fetchHealth().catch((cause: unknown) => cause);

    expect(error).toBeInstanceOf(ApiError);
    const apiError = error as ApiError;
    expect(apiError.status).toBe(503);
    expect(apiError.url).toBe(`${API_BASE}/health`);
    expect(apiError.message).not.toContain("internal detail");
  });

  it("rejects a malformed payload instead of coercing it", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ status: "ok" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const error = await fetchHealth().catch((cause: unknown) => cause);

    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).message).toMatch(/did not match/i);
  });

  it("propagates an abort error unchanged", async () => {
    const abortError = new DOMException("aborted", "AbortError");
    vi.spyOn(globalThis, "fetch").mockRejectedValue(abortError);

    const error = await fetchHealth().catch((cause: unknown) => cause);

    expect(error).toBe(abortError);
  });

  it("only exposes the versioned base path to callers", () => {
    expect(API_BASE).toBe("/api/v1");
  });
});

describe("requestJson guard handling", () => {
  const isNumber = (value: unknown): value is number => typeof value === "number";

  it("returns the payload when the guard accepts it", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify(42), { status: 200 }),
    );

    await expect(requestJson("/number", isNumber)).resolves.toBe(42);
  });

  it("throws a typed error when the guard rejects the payload", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify("not a number"), { status: 200 }),
    );

    await expect(requestJson("/number", isNumber)).rejects.toBeInstanceOf(
      ApiError,
    );
  });
});

describe("isHealth guard", () => {
  it("accepts a complete health payload", () => {
    expect(isHealth(MOCK_HEALTH)).toBe(true);
  });

  it.each([
    ["null", null],
    ["a string", "ok"],
    ["an array", []],
    ["a partial object", { status: "ok", connected: false }],
    ["a wrong-typed field", { ...MOCK_HEALTH, connected: "false" }],
  ])("rejects %s", (_label, value) => {
    expect(isHealth(value)).toBe(false);
  });
});
