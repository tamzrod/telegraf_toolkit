import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DatabasesPage } from "./DatabasesPage";
import { clearSessionToken } from "../../api/session";
import type { ApiProfile } from "../../api/registry";

const LIVE_PROFILE: ApiProfile = {
  connection_id: "live-influx-2x",
  display_name: "Live InfluxDB 2.x",
  family: "influxdb",
  major_version: 2,
  endpoint: "http://127.0.0.1:18086",
  scope: { kind: "organization-bucket", organization: "acme", bucket: "historian" },
  capabilities: {
    query_dialects: ["flux", "influxql"],
    write_protocols: ["line-protocol"],
    schema_discovery: true,
  },
  credential_ref_id: "ref-live-2x",
  mock: false,
  note: "persisted via API",
};

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <DatabasesPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

/** Returns a fresh Response per call, since a body can only be read once. */
function respondWith(body: unknown, status = 200) {
  return () => Promise.resolve(jsonResponse(body, status));
}

beforeEach(() => {
  clearSessionToken();
});

afterEach(() => {
  vi.restoreAllMocks();
  clearSessionToken();
});

describe("DatabasesPage live integration", () => {
  it("lists live profiles from the authorized API and labels them LIVE, not MOCK", async () => {
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(respondWith([LIVE_PROFILE]));
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: /live registry api/i }));

    expect(await screen.findByText("Live InfluxDB 2.x")).toBeInTheDocument();
    expect(screen.getAllByText("LIVE").length).toBeGreaterThan(0);
    expect(screen.queryByText("MOCK")).not.toBeInTheDocument();

    // The live request went to the versioned registry path.
    const liveCall = fetchSpy.mock.calls.find((call) =>
      String(call[0]).includes("/registry/profiles"),
    );
    expect(liveCall).toBeDefined();
    expect(String(liveCall?.[0])).toBe("/api/v1/registry/profiles");
  });

  it("never writes the session token to browser storage", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(respondWith([LIVE_PROFILE]));
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: /live registry api/i }));
    await user.type(screen.getByLabelText(/session token/i), "operator-secret-token");
    await user.click(screen.getByRole("button", { name: /use token/i }));

    expect(window.localStorage.length).toBe(0);
    expect(window.sessionStorage.length).toBe(0);
    // The token is not echoed back into the DOM.
    expect(screen.queryByDisplayValue("operator-secret-token")).not.toBeInTheDocument();
    expect(
      screen.getByText(/A session token is held for this session/i),
    ).toBeInTheDocument();
  });

  it("attaches the bearer token to the live request", async () => {
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(respondWith([LIVE_PROFILE]));
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: /live registry api/i }));
    await user.type(screen.getByLabelText(/session token/i), "operator-token");
    await user.click(screen.getByRole("button", { name: /use token/i }));

    await waitFor(() => {
      const authorized = fetchSpy.mock.calls.find((call) => {
        const init = call[1] as RequestInit | undefined;
        const headers = (init?.headers ?? {}) as Record<string, string>;
        return headers.Authorization === "Bearer operator-token";
      });
      expect(authorized).toBeDefined();
    });
  });

  it("shows an explicit error, not a mock fallback, when the live API rejects the request", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(
      respondWith({ code: "authentication-required", message: "A bearer token is required." }, 401),
    );
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: /live registry api/i }));

    expect(
      await screen.findByText(/Registry profiles unavailable/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/this is not a mock fallback/i),
    ).toBeInTheDocument();
    // No mock profile leaked into the live view.
    expect(screen.queryByText("MOCK InfluxDB 1.x (plant-a)")).not.toBeInTheDocument();
  });
});
