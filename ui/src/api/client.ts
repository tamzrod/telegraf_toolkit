import type { Health } from "../types/health";

/**
 * Typed API client for the Toolkit backend.
 *
 * Phase 0 scaffold: only the health endpoint exists. The client is the single
 * boundary through which the SPA talks to the backend; no database, Docker or
 * Telegraf runtime access is performed from the browser.
 */

export const API_BASE = "/api/v1";

export async function fetchHealth(signal?: AbortSignal): Promise<Health> {
  const response = await fetch(`${API_BASE}/health`, { signal });
  if (!response.ok) {
    throw new Error(`health request failed with status ${response.status}`);
  }
  return (await response.json()) as Health;
}
