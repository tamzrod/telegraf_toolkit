import type { Health } from "../../types/health";

/**
 * Test-only mock backend adapter.
 *
 * This module exists so tests never reach the network. It is deliberately
 * isolated under `api/mock/` and must not be imported by application code:
 * feature modules depend on the typed client in `api/client.ts`, and the mock
 * is swapped in by stubbing `fetch` in tests.
 */

export const MOCK_HEALTH: Health = {
  status: "ok",
  service: "telegraf-toolkit-api",
  version: "0.0.0-scaffold",
  scaffold: true,
  connected: false,
  description:
    "Phase 0 scaffold. Not connected to databases, Telegraf runtimes or Docker.",
};

/** Returns a resolved JSON `Response` shaped like the real health endpoint. */
export function healthResponse(body: Health = MOCK_HEALTH): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}
