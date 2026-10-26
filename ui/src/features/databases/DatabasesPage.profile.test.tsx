import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DatabasesPage } from "./DatabasesPage";
import { clearSessionToken, setSessionToken } from "../../api/session";
import type { ApiProfile } from "../../api/registry";

const LIVE_PROFILE: ApiProfile = {
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

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
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

function respondWith(body: unknown, status = 200) {
  return () => Promise.resolve(jsonResponse(body, status));
}

/** Answers the profile list with the given profiles and every mutation with 200. */
function respondWithList(profiles: ApiProfile[]) {
  return (input: RequestInfo | URL, init?: RequestInit) => {
    const method = init?.method ?? "GET";
    if (method === "POST" && String(input).endsWith("/registry/profiles")) {
      return Promise.resolve(jsonResponse(LIVE_PROFILE, 201));
    }
    if (method === "DELETE") {
      return Promise.resolve(new Response(null, { status: 204 }));
    }
    return Promise.resolve(jsonResponse(profiles));
  };
}

/** The live create form's own status region, so its wording is not confused with the mock draft's. */
function liveStatus() {
  const element = document.getElementById("live-create-status");
  if (!element) {
    throw new Error("the live create status region was not rendered");
  }
  return within(element);
}

beforeEach(() => {
  clearSessionToken();
});

afterEach(() => {
  vi.restoreAllMocks();
  clearSessionToken();
});

describe("DatabasesPage live profile management", () => {
  it("creates an unbound profile through the authorized API and refreshes the list", async () => {
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(respondWithList([]));
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: /live registry api/i }));
    setSessionToken("operator-token");

    await user.type(screen.getByLabelText(/^new connection id$/i), "live-influx-2x");
    await user.type(screen.getByLabelText(/^new display name$/i), "Live InfluxDB 2.x");
    await user.type(screen.getByLabelText(/^endpoint$/i), "http://127.0.0.1:18086");
    await user.type(screen.getByLabelText(/^organization$/i), "acme");
    await user.type(screen.getByLabelText(/^bucket$/i), "historian");
    await user.click(screen.getByRole("button", { name: /^create profile$/i }));

    await waitFor(() => {
      const create = fetchSpy.mock.calls.find(
        (call) =>
          (call[1] as RequestInit | undefined)?.method === "POST" &&
          String(call[0]).endsWith("/registry/profiles"),
      );
      expect(create).toBeDefined();
    });

    const create = fetchSpy.mock.calls.find(
      (call) =>
        (call[1] as RequestInit | undefined)?.method === "POST" &&
        String(call[0]).endsWith("/registry/profiles"),
    );
    const body = JSON.parse(String((create?.[1] as RequestInit).body));
    // The browser never sends a credential reference or value.
    expect(body.credential_ref_id).toBeUndefined();
    expect(body.scope).toEqual({
      kind: "organization-bucket",
      organization: "acme",
      bucket: "historian",
    });
    expect(await screen.findByText(/created profile live-influx-2x/i)).toBeInTheDocument();
  });

  it("refuses to send an incomplete draft and explains why", async () => {
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(respondWithList([LIVE_PROFILE]));
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: /live registry api/i }));
    await user.click(screen.getByRole("button", { name: /^create profile$/i }));

    expect(
      await liveStatus().findByText(/a connection id is required/i),
    ).toBeInTheDocument();
    const create = fetchSpy.mock.calls.find(
      (call) => (call[1] as RequestInit | undefined)?.method === "POST",
    );
    expect(create).toBeUndefined();
  });

  it("refuses an endpoint that carries embedded credentials", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(respondWithList([LIVE_PROFILE]));
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: /live registry api/i }));
    await user.type(screen.getByLabelText(/^new connection id$/i), "c1");
    await user.type(screen.getByLabelText(/^new display name$/i), "C1");
    await user.type(
      screen.getByLabelText(/^endpoint$/i),
      "http://admin:secret@127.0.0.1:18086",
    );
    await user.click(screen.getByRole("button", { name: /^create profile$/i }));

    expect(
      await screen.findByText(/absolute http or https url without embedded credentials/i),
    ).toBeInTheDocument();
  });

  it("shows the permission state when the API refuses the create", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation((input, init) => {
      if ((init as RequestInit | undefined)?.method === "POST") {
        return Promise.resolve(jsonResponse({ code: "permission-denied" }, 403));
      }
      return respondWithList([LIVE_PROFILE])(input, init);
    });
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: /live registry api/i }));
    await user.type(screen.getByLabelText(/^new connection id$/i), "c1");
    await user.type(screen.getByLabelText(/^new display name$/i), "C1");
    await user.type(screen.getByLabelText(/^endpoint$/i), "http://127.0.0.1:18086");
    await user.type(screen.getByLabelText(/^organization$/i), "acme");
    await user.type(screen.getByLabelText(/^bucket$/i), "historian");
    await user.click(screen.getByRole("button", { name: /^create profile$/i }));

    expect(
      await screen.findByText(/your role is not permitted/i),
    ).toBeInTheDocument();
  });

  it("reports a duplicate connection ID without inventing a success", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation((input, init) => {
      if ((init as RequestInit | undefined)?.method === "POST") {
        return Promise.resolve(jsonResponse({ code: "duplicate-profile" }, 409));
      }
      return respondWithList([LIVE_PROFILE])(input, init);
    });
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: /live registry api/i }));
    await user.type(screen.getByLabelText(/^new connection id$/i), "c1");
    await user.type(screen.getByLabelText(/^new display name$/i), "C1");
    await user.type(screen.getByLabelText(/^endpoint$/i), "http://127.0.0.1:18086");
    await user.type(screen.getByLabelText(/^organization$/i), "acme");
    await user.type(screen.getByLabelText(/^bucket$/i), "historian");
    await user.click(screen.getByRole("button", { name: /^create profile$/i }));

    expect(
      await screen.findByText(/already exists/i),
    ).toBeInTheDocument();
    expect(screen.queryByText(/created profile/i)).not.toBeInTheDocument();
  });

  it("requires a confirmation before a profile is deleted", async () => {
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(respondWithList([LIVE_PROFILE]));
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: /live registry api/i }));
    await user.click(await screen.findByRole("button", { name: /delete profile/i }));

    // The confirmation is shown and nothing has been deleted yet.
    expect(await screen.findByText(/delete live-influx-2x\?/i)).toBeInTheDocument();
    expect(
      fetchSpy.mock.calls.find(
        (call) => (call[1] as RequestInit | undefined)?.method === "DELETE",
      ),
    ).toBeUndefined();

    await user.click(screen.getByRole("button", { name: /confirm delete/i }));

    await waitFor(() => {
      expect(
        fetchSpy.mock.calls.find(
          (call) => (call[1] as RequestInit | undefined)?.method === "DELETE",
        ),
      ).toBeDefined();
    });
    expect(await screen.findByText(/deleted profile live-influx-2x/i)).toBeInTheDocument();
  });

  it("keeps the profile when the delete is refused", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation((input, init) => {
      if ((init as RequestInit | undefined)?.method === "DELETE") {
        return Promise.resolve(jsonResponse({ code: "permission-denied" }, 403));
      }
      return respondWithList([LIVE_PROFILE])(input, init);
    });
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: /live registry api/i }));
    await user.click(await screen.findByRole("button", { name: /delete profile/i }));
    await user.click(screen.getByRole("button", { name: /confirm delete/i }));

    expect(
      await screen.findByText(/your role is not permitted/i),
    ).toBeInTheDocument();
    expect(await screen.findByText("Live InfluxDB 2.x")).toBeInTheDocument();
  });

  it("clears a stale selection when the selected profile is deleted", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(respondWithList([LIVE_PROFILE]));
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: /live registry api/i }));
    await user.selectOptions(
      await screen.findByLabelText(/source database/i),
      "live-influx-2x",
    );
    await waitFor(() => {
      expect(
        (screen.getByLabelText(/source database/i) as HTMLSelectElement).value,
      ).toBe("live-influx-2x");
    });

    await user.click(screen.getByRole("button", { name: /delete profile/i }));
    await user.click(screen.getByRole("button", { name: /confirm delete/i }));

    await waitFor(() => {
      expect(
        (screen.getByLabelText(/source database/i) as HTMLSelectElement).value,
      ).toBe("");
    });
  });

  it("states that a profile without a binding resolves no credential", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(respondWithList([LIVE_PROFILE]));
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: /live registry api/i }));

    const profile = await screen.findByText("Live InfluxDB 2.x");
    const item = profile.closest("li") as HTMLElement;
    expect(
      within(item).getByText(/resolves no credential/i),
    ).toBeInTheDocument();
  });

  it("keeps the mock source separate: a live failure never falls back to fixtures", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(
      respondWith({ code: "authentication-required" }, 401),
    );
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: /live registry api/i }));

    expect(
      await screen.findByText(/this is not a mock fallback/i),
    ).toBeInTheDocument();
    expect(screen.queryByText("MOCK")).not.toBeInTheDocument();
  });
});
