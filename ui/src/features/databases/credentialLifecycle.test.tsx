import { render, screen, waitFor } from "@testing-library/react";
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

const CREDENTIAL_REF = {
  credential_ref_id: "cred-1",
  alias: "historian write",
  role: "write",
  fingerprint: "fp-1",
  mock: false,
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

beforeEach(() => {
  clearSessionToken();
  setSessionToken("operator-token");
});

afterEach(() => {
  vi.restoreAllMocks();
  clearSessionToken();
});

describe("credential lifecycle in the browser", () => {
  it("submits a write-only value once and clears it from the form", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockImplementation((_input, init) => {
      if ((init as RequestInit | undefined)?.method === "POST") {
        return Promise.resolve(jsonResponse(CREDENTIAL_REF, 201));
      }
      return Promise.resolve(jsonResponse([LIVE_PROFILE]));
    });
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: /live registry api/i }));
    await user.selectOptions(
      await screen.findByLabelText(/^connection profile$/i),
      "live-influx-2x",
    );
    const valueField = screen.getByLabelText(/credential value/i);
    await user.type(valueField, "super-secret-token");
    await user.click(screen.getByRole("button", { name: /^enroll credential$/i }));

    await waitFor(() => {
      const enroll = fetchSpy.mock.calls.find(
        (call) =>
          (call[1] as RequestInit | undefined)?.method === "POST" &&
          String(call[0]).endsWith("/registry/credentials"),
      );
      expect(enroll).toBeDefined();
    });

    const enroll = fetchSpy.mock.calls.find(
      (call) =>
        (call[1] as RequestInit | undefined)?.method === "POST" &&
        String(call[0]).endsWith("/registry/credentials"),
    );
    const body = JSON.parse(String((enroll?.[1] as RequestInit).body));
    expect(body.value).toBe("super-secret-token");
    expect(body.connection_id).toBe("live-influx-2x");

    // The value is cleared from the form and never echoed into the page.
    await waitFor(() => {
      expect(
        (screen.getByLabelText(/credential value/i) as HTMLInputElement).value,
      ).toBe("");
    });
    expect(screen.queryByDisplayValue("super-secret-token")).not.toBeInTheDocument();
    expect(
      await screen.findByText(/Enrolled credential cred-1/i),
    ).toBeInTheDocument();

    // The value is never written to browser storage.
    expect(window.localStorage.length).toBe(0);
    expect(window.sessionStorage.length).toBe(0);
  });

  it("shows the permission state when the role may not enroll a credential", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation((_input, init) => {
      if ((init as RequestInit | undefined)?.method === "POST") {
        return Promise.resolve(jsonResponse({ code: "permission-denied" }, 403));
      }
      return Promise.resolve(jsonResponse([LIVE_PROFILE]));
    });
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: /live registry api/i }));
    await user.selectOptions(
      await screen.findByLabelText(/^connection profile$/i),
      "live-influx-2x",
    );
    await user.type(screen.getByLabelText(/credential value/i), "secret");
    await user.click(screen.getByRole("button", { name: /^enroll credential$/i }));

    expect(
      await screen.findByText(/role is not permitted/i),
    ).toBeInTheDocument();
  });

  it("reports a refusal to revoke a credential that is still approved", async () => {
    const boundProfile: ApiProfile = {
      ...LIVE_PROFILE,
      credential_ref_id: "cred-1",
    };
    vi.spyOn(globalThis, "fetch").mockImplementation((_input, init) => {
      if ((init as RequestInit | undefined)?.method === "DELETE") {
        return Promise.resolve(jsonResponse({ code: "credential-in-use" }, 409));
      }
      return Promise.resolve(jsonResponse([boundProfile]));
    });
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: /live registry api/i }));
    await user.click(await screen.findByRole("button", { name: /revoke credential/i }));
    await user.click(screen.getByRole("button", { name: /confirm revoke/i }));

    expect(
      await screen.findByText(/already exists|role is not permitted|could not complete/i),
    ).toBeInTheDocument();
  });

  it("never sends the credential value in a URL", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockImplementation((_input, init) => {
      if ((init as RequestInit | undefined)?.method === "POST") {
        return Promise.resolve(jsonResponse(CREDENTIAL_REF, 201));
      }
      return Promise.resolve(jsonResponse([LIVE_PROFILE]));
    });
    const user = userEvent.setup();
    renderPage();

    await user.click(screen.getByRole("button", { name: /live registry api/i }));
    await user.selectOptions(
      await screen.findByLabelText(/^connection profile$/i),
      "live-influx-2x",
    );
    await user.type(screen.getByLabelText(/credential value/i), "super-secret-token");
    await user.click(screen.getByRole("button", { name: /^enroll credential$/i }));

    await waitFor(() => {
      expect(
        fetchSpy.mock.calls.some(
          (call) => String(call[0]).includes("super-secret-token"),
        ),
      ).toBe(false);
    });
    for (const call of fetchSpy.mock.calls) {
      expect(String(call[0])).not.toContain("super-secret-token");
    }
  });
});
