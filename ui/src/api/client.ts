import type { Health } from "../types/health";
import { isHealth } from "../types/health";
import type {
  ApiProbeResult,
  ApiProfile,
  RegistryProfile,
} from "./registry";
import {
  isApiProfileList,
  isApiProbeResult,
  isRegistryProfileList,
  toRegistryProfile,
} from "./registry";
import { getSessionToken } from "./session";

/**
 * Single typed boundary between the SPA and the Toolkit backend.
 *
 * The health endpoint is a scaffold. The Registry endpoints are real and
 * authorized: when a session token is present it is attached as a bearer
 * header. Every request goes through `requestJson`, which enforces the shared
 * conventions: a versioned base path, JSON parsing, cancellation via
 * `AbortSignal`, and a typed error that carries no response body. Credentials
 * are never sent from or stored in the browser.
 */

export const API_BASE = "/api/v1";

/**
 * Typed transport failure. It deliberately carries only a status code and a
 * short message: raw response bodies are never surfaced to the UI, so a
 * sanitized backend error cannot leak through this path.
 */
export class ApiError extends Error {
  readonly status: number;
  readonly url: string;

  constructor(message: string, options: { status: number; url: string }) {
    super(message);
    this.name = "ApiError";
    this.status = options.status;
    this.url = options.url;
  }
}

export type RequestOptions = {
  signal?: AbortSignal;
  method?: "GET" | "POST" | "DELETE";
  body?: unknown;
};

/**
 * Performs a request against the versioned API and validates the payload with a
 * caller-supplied guard. A guard failure is reported as an `ApiError` rather
 * than letting an unexpected shape reach feature code.
 *
 * When a session token is held it is attached as `Authorization: Bearer …`.
 * The token is read from the in-memory holder and is never logged.
 */
export async function requestJson<T>(
  path: string,
  guard: (value: unknown) => value is T,
  options: RequestOptions = {},
): Promise<T> {
  const url = `${API_BASE}${path}`;
  const headers: Record<string, string> = { Accept: "application/json" };
  const token = getSessionToken();
  if (token !== "") {
    headers.Authorization = `Bearer ${token}`;
  }
  if (options.body !== undefined) {
    headers["Content-Type"] = "application/json";
  }

  const response = await fetch(url, {
    method: options.method ?? "GET",
    headers,
    signal: options.signal,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  });

  if (!response.ok) {
    throw new ApiError(`request failed with status ${response.status}`, {
      status: response.status,
      url,
    });
  }

  // A 204 response (for example a delete) has no body to validate.
  if (response.status === 204) {
    return undefined as T;
  }

  const payload: unknown = await response.json();
  if (!guard(payload)) {
    throw new ApiError("response did not match the expected contract", {
      status: response.status,
      url,
    });
  }

  return payload;
}

export async function fetchHealth(signal?: AbortSignal): Promise<Health> {
  return requestJson("/health", isHealth, { signal });
}

/**
 * Lists registry connection profiles from the mock adapter. The payload is
 * validated against the Registry contract, which rejects any response that
 * would carry a credential value.
 */
export async function fetchRegistryProfiles(
  signal?: AbortSignal,
): Promise<RegistryProfile[]> {
  return requestJson("/registry/profiles", isRegistryProfileList, { signal });
}

/**
 * Lists registry connection profiles from the live, authorized API and converts
 * them to the browser view model. It requires a session token; without one the
 * backend responds 401 and this throws an `ApiError`.
 */
export async function fetchLiveRegistryProfiles(
  signal?: AbortSignal,
): Promise<RegistryProfile[]> {
  const api = await requestJson<ApiProfile[]>(
    "/registry/profiles",
    isApiProfileList,
    { signal },
  );
  return api.map(toRegistryProfile);
}

/**
 * Performs a read-only capability probe through the authorized API.
 *
 * The browser sends only a connection profile ID. The endpoint and credential
 * are resolved server-side from the stored profile, so the browser cannot direct
 * a stored secret to an origin of its choosing.
 */
export async function probeCapabilities(
  input: { connectionId: string; deadlineMs?: number },
  signal?: AbortSignal,
): Promise<ApiProbeResult> {
  return requestJson("/registry/probe", isApiProbeResult, {
    signal,
    method: "POST",
    body: {
      connection_id: input.connectionId,
      deadline_ms: input.deadlineMs ?? 0,
    },
  });
}
