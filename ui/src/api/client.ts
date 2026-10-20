import type { Health } from "../types/health";
import { isHealth } from "../types/health";
import type { RegistryProfile } from "./registry";
import { isRegistryProfileList } from "./registry";

/**
 * Single typed boundary between the SPA and the Toolkit backend.
 *
 * Only `/api/v1/health` exists in Phase 1. Every request goes through
 * `requestJson`, which enforces the shared conventions: a versioned base path,
 * JSON parsing, cancellation via `AbortSignal`, and a typed error that carries
 * no response body. Credentials are never sent from or stored in the browser.
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
};

/**
 * Performs a GET request against the versioned API and validates the payload
 * with a caller-supplied guard. A guard failure is reported as an `ApiError`
 * rather than letting an unexpected shape reach feature code.
 */
export async function requestJson<T>(
  path: string,
  guard: (value: unknown) => value is T,
  options: RequestOptions = {},
): Promise<T> {
  const url = `${API_BASE}${path}`;

  const response = await fetch(url, {
    method: "GET",
    headers: { Accept: "application/json" },
    signal: options.signal,
  });

  if (!response.ok) {
    throw new ApiError(`request failed with status ${response.status}`, {
      status: response.status,
      url,
    });
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
 * Lists registry connection profiles. The payload is validated against the
 * Registry contract, which rejects any response that would carry a credential
 * value, so a secret can never reach a feature component through this path.
 */
export async function fetchRegistryProfiles(
  signal?: AbortSignal,
): Promise<RegistryProfile[]> {
  return requestJson("/registry/profiles", isRegistryProfileList, { signal });
}
