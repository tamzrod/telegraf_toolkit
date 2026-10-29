import type { Health } from "../types/health";
import { isHealth } from "../types/health";
import type {
  ApiCredentialInput,
  ApiCredentialRef,
  ApiCredentialReplaceInput,
  ApiProbeResult,
  ApiProfile,
  ApiProfileInput,
  ApiSchemaResult,
  RegistryProfile,
} from "./registry";
import {
  isApiCredentialInput,
  isApiCredentialRef,
  isApiProfile,
  isApiProfileInput,
  isApiProfileList,
  isApiProbeResult,
  isApiSchemaResult,
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
  method?: "GET" | "POST" | "PUT" | "DELETE";
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

  // A body that is not JSON is a transport failure, not a raw parse error: the
  // raw exception would carry the response text into the UI.
  let payload: unknown;
  try {
    payload = await response.json();
  } catch {
    throw new ApiError("response was not valid JSON", {
      status: response.status,
      url,
    });
  }
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

/**
 * Reads the observed measurement names for one stored connection profile.
 *
 * The browser sends only the connection ID: the endpoint, canonical scope and
 * credential are resolved server-side from the stored profile. The request is a
 * GET and the server executes a fixed `SHOW MEASUREMENTS` statement; there is no
 * caller-supplied query in this path.
 */
export async function fetchSchemaMeasurements(
  connectionId: string,
  signal?: AbortSignal,
): Promise<ApiSchemaResult> {
  const query = new URLSearchParams({ connection_id: connectionId });
  return requestJson(
    `/registry/schema/measurements?${query.toString()}`,
    isApiSchemaResult,
    { signal },
  );
}

/**
 * Reads the observed field keys for one measurement of a stored profile.
 *
 * The measurement must be one the server already observed; the server verifies
 * membership before it executes `SHOW FIELD KEYS`. The browser supplies no
 * query, endpoint or credential.
 */
export async function fetchSchemaFields(
  connectionId: string,
  measurement: string,
  signal?: AbortSignal,
): Promise<ApiSchemaResult> {
  const query = new URLSearchParams({ connection_id: connectionId, measurement });
  return requestJson(
    `/registry/schema/fields?${query.toString()}`,
    isApiSchemaResult,
    { signal },
  );
}

/**
 * Reads the observed tag keys for one measurement of a stored profile.
 *
 * This is the tag-key counterpart of `fetchSchemaFields`; it shares the same
 * membership check and fixed `SHOW TAG KEYS` statement.
 */
export async function fetchSchemaTags(
  connectionId: string,
  measurement: string,
  signal?: AbortSignal,
): Promise<ApiSchemaResult> {
  const query = new URLSearchParams({ connection_id: connectionId, measurement });
  return requestJson(
    `/registry/schema/tags?${query.toString()}`,
    isApiSchemaResult,
    { signal },
  );
}

/**
 * Creates an **unbound** connection profile through the authorized API.
 *
 * The body carries no credential reference and no credential value: attaching a
 * stored credential is an administrator-only operation that stays gated until
 * the credential lifecycle is delivered (P2C-005). The server validates the
 * body again and is authoritative; a rejection is reported as an `ApiError`
 * with the status only, so a sanitized backend message never reaches the DOM.
 */
export async function createLiveProfile(
  input: ApiProfileInput,
  signal?: AbortSignal,
): Promise<ApiProfile> {
  if (!isApiProfileInput(input)) {
    throw new ApiError("the profile body did not match the expected contract", {
      status: 0,
      url: `${API_BASE}/registry/profiles`,
    });
  }
  return requestJson("/registry/profiles", isApiProfile, {
    signal,
    method: "POST",
    body: input,
  });
}

/**
 * Reads one connection profile from the authorized API.
 */
export async function fetchLiveRegistryProfile(
  connectionId: string,
  signal?: AbortSignal,
): Promise<ApiProfile> {
  return requestJson(
    `/registry/profiles/${encodeURIComponent(connectionId)}`,
    isApiProfile,
    { signal },
  );
}

/**
 * Deletes one connection profile through the authorized API.
 *
 * The server removes the profile and its approved credential bindings in one
 * transaction (P2C-002). A rejection is reported as an `ApiError` with the
 * status only.
 */
export async function deleteLiveProfile(
  connectionId: string,
  signal?: AbortSignal,
): Promise<void> {
  // A delete answers 204 with no body; the guard only has to accept a body if
  // the server ever sends one.
  await requestJson<unknown>(
    `/registry/profiles/${encodeURIComponent(connectionId)}`,
    acceptsAnyBody,
    { signal, method: "DELETE" },
  );
}

/** Accepts any body. Used for a delete, whose success answer carries none. */
function acceptsAnyBody(value: unknown): value is unknown {
  void value;
  return true;
}

/**
 * Enrolls a credential for a profile through the authorized API.
 *
 * The value is **write-only**: it is submitted once and never returned. The
 * response carries the reference only, so the browser never holds the value
 * after the request. The enrollment requires the administrator-only
 * `credential.write` and `credential.attach` permissions.
 */
export async function enrollCredential(
  input: ApiCredentialInput,
  signal?: AbortSignal,
): Promise<ApiCredentialRef> {
  if (!isApiCredentialInput(input)) {
    throw new ApiError("the credential body did not match the expected contract", {
      status: 0,
      url: `${API_BASE}/registry/credentials`,
    });
  }
  return requestJson("/registry/credentials", isApiCredentialRef, {
    signal,
    method: "POST",
    body: input,
  });
}

/**
 * Replaces the credential a profile is bound to.
 *
 * The server stages the new value, validates it against the profile's
 * approved endpoint and only then swaps it in; an inconclusive validation
 * leaves the credential that was in place usable.
 */
export async function replaceCredential(
  input: ApiCredentialReplaceInput,
  signal?: AbortSignal,
): Promise<ApiCredentialRef> {
  if (!isApiCredentialInput(input)) {
    throw new ApiError("the credential body did not match the expected contract", {
      status: 0,
      url: `${API_BASE}/registry/credentials/replace`,
    });
  }
  return requestJson("/registry/credentials/replace", isApiCredentialRef, {
    signal,
    method: "POST",
    body: input,
  });
}

/**
 * Revokes a stored credential.
 *
 * The server refuses the revocation while an approved binding still requires
 * the credential.
 */
export async function revokeCredential(
  credentialRefId: string,
  signal?: AbortSignal,
): Promise<void> {
  await requestJson<unknown>(
    `/registry/credentials/${encodeURIComponent(credentialRefId)}`,
    acceptsAnyBody,
    { signal, method: "DELETE" },
  );
}

/** A stored pipeline revision. `deployed` is false because these routes never deploy. */
export type PipelineRevisionPayload = {
  pipeline_id: string;
  revision_id: string;
  revision: number;
  state: "draft" | "approved";
  mode: "graph" | "manual";
  generation: number;
  deployed: false;
  export_toml?: string;
};

export type PipelineTransitionReview = {
  from_mode: "graph" | "manual";
  to_mode: "graph" | "manual";
  current_text: string;
  proposed_text: string;
  digest: string;
  deployed: false;
};

function isPipelineRevisionPayload(value: unknown): value is PipelineRevisionPayload {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const row = value as Record<string, unknown>;
  return (
    row.deployed === false &&
    typeof row.pipeline_id === "string" &&
    typeof row.revision_id === "string" &&
    typeof row.revision === "number" &&
    (row.state === "draft" || row.state === "approved") &&
    (row.mode === "graph" || row.mode === "manual") &&
    typeof row.generation === "number"
  );
}

function isPipelineTransitionReview(value: unknown): value is PipelineTransitionReview {
  if (typeof value !== "object" || value === null) {
    return false;
  }
  const row = value as Record<string, unknown>;
  return (
    row.deployed === false &&
    (row.from_mode === "graph" || row.from_mode === "manual") &&
    (row.to_mode === "graph" || row.to_mode === "manual") &&
    typeof row.current_text === "string" &&
    typeof row.proposed_text === "string" &&
    typeof row.digest === "string"
  );
}

/** Creates an empty graph draft. The draft is not deployed. */
export function createPipelineRevision(signal?: AbortSignal): Promise<PipelineRevisionPayload> {
  return requestJson("/pipelines", isPipelineRevisionPayload, { method: "POST", signal });
}

/** Replaces a draft document without changing its mode. */
export function savePipelineRevision(
  pipelineId: string,
  revision: number,
  expectedGeneration: number,
  document: unknown,
  signal?: AbortSignal,
): Promise<PipelineRevisionPayload> {
  return requestJson(
    `/pipelines/${encodeURIComponent(pipelineId)}/revisions/${revision}`,
    isPipelineRevisionPayload,
    {
      method: "PUT",
      signal,
      body: { expected_generation: expectedGeneration, document },
    },
  );
}

/** Returns the difference a mode change would write. */
export function previewPipelineTransition(
  pipelineId: string,
  revision: number,
  targetMode: "graph" | "manual" | "snapshot",
  signal?: AbortSignal,
): Promise<PipelineTransitionReview> {
  return requestJson(
    `/pipelines/${encodeURIComponent(pipelineId)}/revisions/${revision}/transition-preview`,
    isPipelineTransitionReview,
    { method: "POST", signal, body: { target_mode: targetMode } },
  );
}

/** Writes a reviewed mode change. */
export function commitPipelineTransition(
  pipelineId: string,
  revision: number,
  expectedGeneration: number,
  targetMode: "graph" | "manual",
  digest: string,
  signal?: AbortSignal,
): Promise<PipelineRevisionPayload> {
  return requestJson(
    `/pipelines/${encodeURIComponent(pipelineId)}/revisions/${revision}/transition`,
    isPipelineRevisionPayload,
    {
      method: "POST",
      signal,
      body: { expected_generation: expectedGeneration, target_mode: targetMode, digest },
    },
  );
}

/** Approves a draft. Approval does not deploy the revision. */
export function approvePipelineRevision(
  pipelineId: string,
  revision: number,
  expectedGeneration: number,
  signal?: AbortSignal,
): Promise<PipelineRevisionPayload> {
  return requestJson(
    `/pipelines/${encodeURIComponent(pipelineId)}/revisions/${revision}/approve`,
    isPipelineRevisionPayload,
    { method: "POST", signal, body: { expected_generation: expectedGeneration } },
  );
}
