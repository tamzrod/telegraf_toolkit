import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CanonicalMapperPage } from "./CanonicalMapperPage";
import { registryProfilesResponse } from "../../api/mock/registry";
import {
  LIVE_REGISTRY_PROFILES,
  fieldsSchemaResult,
  liveProfilesResponse,
  makeSchemaResult,
  schemaResponse,
  tagsSchemaResult,
} from "../../api/mock/schema";
import { clearSessionToken } from "../../api/session";
import { formatScope, toRegistryProfile } from "../../api/registry";

type UserEvent = ReturnType<typeof userEvent.setup>;

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <CanonicalMapperPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

afterEach(() => {
  vi.restoreAllMocks();
  clearSessionToken();
});

function panelFor(name: RegExp): HTMLElement {
  const heading = screen.getByRole("heading", { level: 3, name });
  const panel = heading.closest("article");
  if (!panel) {
    throw new Error(`no schema panel for ${String(name)}`);
  }
  return panel as HTMLElement;
}

function defaultSchemaResponse(url: URL): Response {
  const connectionId = url.searchParams.get("connection_id") ?? "";
  const profile = LIVE_REGISTRY_PROFILES.find(
    (candidate) => candidate.connection_id === connectionId,
  );
  if (!profile) {
    return new Response("not found", { status: 404 });
  }
  const scope = formatScope(toRegistryProfile(profile).scope);

  if (url.pathname.endsWith("/measurements")) {
    if (profile.major_version === 1) {
      return schemaResponse(
        makeSchemaResult({
          connection_id: connectionId,
          scope,
          outcome: "unknown",
          measurements: [],
          observations: [
            {
              check: "schema-1x-unverified",
              method: "none",
              path: "",
              status: 0,
              outcome: "unknown",
              edition: "influxdb-1.x-unverified",
              detail:
                "InfluxDB 1.x schema metadata remains unverified; no request was made.",
            },
          ],
        }),
      );
    }
    return schemaResponse(
      makeSchemaResult({ connection_id: connectionId, scope }),
    );
  }

  const measurement = url.searchParams.get("measurement") ?? "";
  if (url.pathname.endsWith("/fields")) {
    return schemaResponse(
      fieldsSchemaResult({
        connection_id: connectionId,
        scope,
        measurement,
      }),
    );
  }
  if (url.pathname.endsWith("/tags")) {
    return schemaResponse(
      tagsSchemaResult({
        connection_id: connectionId,
        scope,
        measurement,
      }),
    );
  }
  return new Response("not found", { status: 404 });
}

type LiveSchemaHandler = (
  url: URL,
  init?: RequestInit,
) => Response | Promise<Response>;

function installLiveBackend(
  overrides: { schema?: LiveSchemaHandler } = {},
) {
  return vi.spyOn(globalThis, "fetch").mockImplementation(
    async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = new URL(String(input), "http://localhost");
      const auth = new Headers(init?.headers).get("Authorization");
      if (url.pathname === "/api/v1/registry/profiles") {
        return auth === "Bearer test-session"
          ? liveProfilesResponse()
          : registryProfilesResponse();
      }
      if (auth !== "Bearer test-session") {
        return new Response("unauthorized", { status: 401 });
      }
      if (overrides.schema) {
        return overrides.schema(url, init);
      }
      return defaultSchemaResponse(url);
    },
  );
}

async function activateLive(user: UserEvent) {
  await user.click(screen.getByRole("button", { name: "Live Registry API" }));
  const tokenInput = screen.getByLabelText(/session token value/i);
  await user.type(tokenInput, "test-session");
  await user.click(screen.getByRole("button", { name: "Use token" }));
  await screen.findByText(/3 connection profiles available/i);
}

async function chooseProfiles(
  user: UserEvent,
  source = "live-influx-2x-a",
  destination = "live-influx-2x-b",
) {
  await user.selectOptions(screen.getByLabelText("Source profile"), source);
  await user.selectOptions(
    screen.getByLabelText("Destination profile"),
    destination,
  );
}

describe("CanonicalMapperPage mock mode", () => {
  it("defaults to a labeled mock source and renders typed synthetic groups", async () => {
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(registryProfilesResponse());
    const user = userEvent.setup();
    renderPage();

    expect(await screen.findByText(/Mock workflow/i)).toBeInTheDocument();
    expect(await screen.findByText(/Sample data only/i)).toBeInTheDocument();

    await user.selectOptions(
      screen.getByLabelText("Source profile"),
      "mock-influx-2x",
    );
    await user.selectOptions(
      screen.getByLabelText("Destination profile"),
      "mock-influx-3x",
    );

    const sourcePanel = await screen.findByRole("heading", {
      level: 3,
      name: /Source schema/i,
    });
    const source = sourcePanel.closest("article") as HTMLElement;
    const destination = panelFor(/Destination schema/i);

    expect(within(source).getByText("MOCK")).toBeInTheDocument();
    expect(within(source).getByText(/Mock preview/i)).toBeInTheDocument();
    await within(source).findByRole("button", { name: "environment" });
    await within(source).findByRole("button", { name: "sensor" });
    await within(source).findByRole("button", { name: "sensor" }).then((button) =>
      user.click(button),
    );

    expect(await within(source).findByText("temperature")).toBeInTheDocument();
    expect(within(source).getByText("host")).toBeInTheDocument();
    expect(within(source).getAllByText("float").length).toBeGreaterThan(0);

    // The 3.x mock profile reports schema discovery as unsupported, so the
    // preview must not invent measurements for it.
    expect(await within(destination).findByText("Unsupported")).toBeInTheDocument();
    expect(
      within(destination).queryByRole("button", { name: "sensor" }),
    ).toBeNull();

    for (const [url] of fetchSpy.mock.calls) {
      expect(String(url)).toBe("/api/v1/registry/profiles");
    }
  });

  it("refuses the same profile and clears the refusal when the pair is distinct", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      registryProfilesResponse(),
    );
    const user = userEvent.setup();
    renderPage();
    await screen.findByText(/Sample data only/i);

    await user.selectOptions(
      screen.getByLabelText("Source profile"),
      "mock-influx-2x",
    );
    await user.selectOptions(
      screen.getByLabelText("Destination profile"),
      "mock-influx-2x",
    );

    expect(
      screen.getByText("Source and destination must be distinct."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { level: 2, name: "Schema browsers" }),
    ).toBeNull();

    await user.selectOptions(
      screen.getByLabelText("Destination profile"),
      "mock-influx-3x",
    );
    expect(
      await screen.findByRole("heading", { level: 2, name: "Schema browsers" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Source and destination must be distinct."),
    ).toBeNull();
  });

  it("shows an unverified 1.x mock preview as unknown, never as supported", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      registryProfilesResponse(),
    );
    const user = userEvent.setup();
    renderPage();
    await screen.findByText(/Sample data only/i);

    await user.selectOptions(
      screen.getByLabelText("Source profile"),
      "mock-influx-1x",
    );
    await user.selectOptions(
      screen.getByLabelText("Destination profile"),
      "mock-influx-2x",
    );

    const source = panelFor(/Source schema/i);
    expect(await within(source).findByText("Unknown")).toBeInTheDocument();
    expect(
      within(source).getByText(/Unknown is not a supported result/i),
    ).toBeInTheDocument();
    expect(
      within(source).queryByRole("button", { name: "environment" }),
    ).toBeNull();
  });

  it("supports keyboard selection and keeps heading order and IDs unique", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      registryProfilesResponse(),
    );
    const user = userEvent.setup();
    renderPage();
    await screen.findByText(/Sample data only/i);

    await user.selectOptions(
      screen.getByLabelText("Source profile"),
      "mock-influx-2x",
    );
    await user.selectOptions(
      screen.getByLabelText("Destination profile"),
      "mock-influx-3x",
    );

    const source = panelFor(/Source schema/i);
    const sensor = await within(source).findByRole("button", { name: "sensor" });
    expect(sensor).toHaveAttribute("aria-pressed", "false");
    sensor.focus();
    expect(sensor).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(sensor).toHaveAttribute("aria-pressed", "true");
    expect(await within(source).findByText("temperature")).toBeInTheDocument();

    const headings = screen.getAllByRole("heading");
    expect(headings[0].tagName).toBe("H1");
    let previous = 0;
    for (const heading of headings) {
      const level = Number(heading.tagName.slice(1));
      expect(level).toBeLessThanOrEqual(previous + 1);
      previous = level;
    }
    expect(screen.queryByRole("heading", { level: 5 })).toBeNull();

    const ids = Array.from(document.querySelectorAll("[id]")).map(
      (element) => element.id,
    );
    expect(new Set(ids).size).toBe(ids.length);

    expect(screen.queryByText(/mock-cred-write-2x/)).toBeNull();
    expect(screen.queryByText(/MOCK write 2.x/)).toBeNull();
  });
});

describe("CanonicalMapperPage live mode", () => {
  it("reads measurements, field keys and tag keys for distinct live profiles", async () => {
    const fetchSpy = installLiveBackend();
    const user = userEvent.setup();
    renderPage();
    await activateLive(user);
    expect(screen.getByLabelText(/session token value/i)).toHaveValue("");
    expect(screen.queryByText("test-session")).toBeNull();

    await chooseProfiles(user);
    const source = panelFor(/Source schema/i);
    const destination = panelFor(/Destination schema/i);

    await within(source).findByRole("button", { name: "sensor" });
    await within(destination).findByRole("button", { name: "sensor" });
    await user.click(within(source).getByRole("button", { name: "sensor" }));

    expect(await within(source).findByText("temperature")).toBeInTheDocument();
    expect(within(source).getByText("host")).toBeInTheDocument();
    expect(within(source).getAllByText("float").length).toBeGreaterThan(0);

    const schemaUrls = fetchSpy.mock.calls
      .map(([input]) => new URL(String(input), "http://localhost"))
      .filter((url) => url.pathname.includes("/schema/"));
    expect(schemaUrls.length).toBeGreaterThanOrEqual(4);
    for (const url of schemaUrls) {
      expect([...url.searchParams.keys()].every((key) =>
        ["connection_id", "measurement"].includes(key),
      )).toBe(true);
      expect(url.searchParams.has("q")).toBe(false);
      expect(url.searchParams.has("endpoint")).toBe(false);
      expect(url.searchParams.has("credential")).toBe(false);
    }

    for (const [, init] of fetchSpy.mock.calls) {
      expect(init?.method ?? "GET").toBe("GET");
      expect(init?.body).toBeUndefined();
    }

    // Safe identity only: no endpoint, credential reference or token value is
    // rendered by the mapper.
    expect(screen.queryByText(/ref-2x-a/)).toBeNull();
    expect(screen.queryByText(/ref-2x-b/)).toBeNull();
    expect(screen.queryByText(/influx-a\.example/)).toBeNull();
    expect(screen.queryByText(/influx-b\.example/)).toBeNull();
    expect(screen.queryByText(/Bearer/)).toBeNull();
  });

  it("shows a permission state without falling back to mock data", async () => {
    installLiveBackend({
      schema: () => new Response("denied by policy", { status: 403 }),
    });
    const user = userEvent.setup();
    renderPage();
    await activateLive(user);
    await chooseProfiles(user);

    const source = panelFor(/Source schema/i);
    expect(
      await within(source).findByText(/not permitted.*schema\.read/i),
    ).toBeInTheDocument();
    expect(within(source).queryByRole("button", { name: "sensor" })).toBeNull();
    expect(screen.queryByText(/Mock preview/i)).toBeNull();
    expect(screen.queryByText(/Sample data only/i)).toBeNull();
  });

  it("renders an unknown observation as unknown and claims no data", async () => {
    installLiveBackend({
      schema: (url) => {
        if (url.pathname.endsWith("/measurements")) {
          const connectionId = url.searchParams.get("connection_id") ?? "";
          return schemaResponse(
            makeSchemaResult({
              connection_id: connectionId,
              scope: "ttk-test-org/ttk-test-bucket",
              outcome: "unknown",
              measurements: [],
              observations: [
                {
                  check: "show-measurements",
                  method: "GET",
                  path: "/query?q=SHOW+MEASUREMENTS",
                  status: 401,
                  outcome: "unknown",
                  detail: "The configured credential was rejected.",
                },
              ],
            }),
          );
        }
        return defaultSchemaResponse(url);
      },
    });
    const user = userEvent.setup();
    renderPage();
    await activateLive(user);
    await chooseProfiles(user);

    const source = panelFor(/Source schema/i);
    expect(await within(source).findByText("Unknown")).toBeInTheDocument();
    expect(
      within(source).getByText(/Unknown is not a supported result/i),
    ).toBeInTheDocument();
    expect(within(source).queryByRole("button", { name: "sensor" })).toBeNull();
  });

  it("renders an unsupported observation as unsupported", async () => {
    installLiveBackend({
      schema: (url) => {
        if (url.pathname.endsWith("/measurements")) {
          const connectionId = url.searchParams.get("connection_id") ?? "";
          return schemaResponse(
            makeSchemaResult({
              connection_id: connectionId,
              scope: "ttk-test-org/ttk-test-bucket",
              outcome: "unsupported",
              measurements: [],
              observations: [
                {
                  check: "show-measurements",
                  method: "GET",
                  path: "/query?q=SHOW+MEASUREMENTS",
                  status: 405,
                  outcome: "unsupported",
                  detail: "The deployment refused the fixed statement.",
                },
              ],
            }),
          );
        }
        return defaultSchemaResponse(url);
      },
    });
    const user = userEvent.setup();
    renderPage();
    await activateLive(user);
    await chooseProfiles(user);

    const source = panelFor(/Source schema/i);
    expect(await within(source).findByText("Unsupported")).toBeInTheDocument();
    expect(
      within(source).getByText(/did not support this metadata read/i),
    ).toBeInTheDocument();
    expect(within(source).queryByRole("button", { name: "sensor" })).toBeNull();
  });

  it("distinguishes an observed empty schema from a failed read", async () => {
    installLiveBackend({
      schema: (url) => {
        if (url.pathname.endsWith("/measurements")) {
          const connectionId = url.searchParams.get("connection_id") ?? "";
          return schemaResponse(
            makeSchemaResult({
              connection_id: connectionId,
              scope: "ttk-test-org/ttk-test-bucket",
              outcome: "supported",
              measurements: [],
            }),
          );
        }
        return defaultSchemaResponse(url);
      },
    });
    const user = userEvent.setup();
    renderPage();
    await activateLive(user);
    await chooseProfiles(user);

    const source = panelFor(/Source schema/i);
    expect(
      await within(source).findByText("No measurements observed"),
    ).toBeInTheDocument();
    expect(within(source).queryByRole("button", { name: "sensor" })).toBeNull();
  });

  it("reports a truncated measurement list explicitly", async () => {
    installLiveBackend({
      schema: (url) => {
        if (url.pathname.endsWith("/measurements")) {
          const connectionId = url.searchParams.get("connection_id") ?? "";
          return schemaResponse(
            makeSchemaResult({
              connection_id: connectionId,
              scope: "ttk-test-org/ttk-test-bucket",
              measurements: ["environment"],
              truncated: true,
            }),
          );
        }
        return defaultSchemaResponse(url);
      },
    });
    const user = userEvent.setup();
    renderPage();
    await activateLive(user);
    await chooseProfiles(user);

    const source = panelFor(/Source schema/i);
    expect(
      await within(source).findByText(/truncated the measurement list/i),
    ).toBeInTheDocument();
    expect(
      within(source).getByRole("button", { name: "environment" }),
    ).toBeInTheDocument();
  });

  it("shows a stale answer instead of data from another scope", async () => {
    installLiveBackend({
      schema: (url) => {
        if (url.pathname.endsWith("/fields")) {
          return schemaResponse(
            fieldsSchemaResult({
              connection_id: "live-influx-2x-a",
              scope: "ttk-test-org/other-bucket",
              measurement: "sensor",
            }),
          );
        }
        return defaultSchemaResponse(url);
      },
    });
    const user = userEvent.setup();
    renderPage();
    await activateLive(user);
    await chooseProfiles(user);

    const source = panelFor(/Source schema/i);
    await within(source).findByRole("button", { name: "sensor" });
    await user.click(within(source).getByRole("button", { name: "sensor" }));

    expect(
      await within(source).findByText("Stale schema answer"),
    ).toBeInTheDocument();
    expect(
      within(source).getByText(/different database scope/i),
    ).toBeInTheDocument();
    expect(within(source).queryByText("temperature")).toBeNull();
    // The tags read is independent and still valid.
    expect(await within(source).findByText("host")).toBeInTheDocument();
  });

  it("announces loading and never substitutes the mock preview", async () => {
    let resolveMeasurements: ((response: Response) => void) | null = null;
    installLiveBackend({
      schema: (url) => {
        if (
          url.pathname.endsWith("/measurements") &&
          url.searchParams.get("connection_id") === "live-influx-2x-a"
        ) {
          return new Promise<Response>((resolve) => {
            resolveMeasurements = resolve;
          });
        }
        return defaultSchemaResponse(url);
      },
    });
    const user = userEvent.setup();
    renderPage();
    await activateLive(user);
    await chooseProfiles(user);

    const source = panelFor(/Source schema/i);
    expect(
      await within(source).findByText("Reading measurements…"),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Mock preview/i)).toBeNull();

    const resolve = resolveMeasurements as
      | ((response: Response) => void)
      | null;
    expect(resolve).not.toBeNull();
    resolve?.(
      schemaResponse(
        makeSchemaResult({
          connection_id: "live-influx-2x-a",
          scope: "ttk-test-org/ttk-test-bucket",
        }),
      ),
    );
    expect(
      await within(source).findByRole("button", { name: "sensor" }),
    ).toBeInTheDocument();
  });

  it("cancels and clears a dependent schema read when the selection changes", async () => {
    let fieldsSignal: AbortSignal | null = null;
    let resolveFields: ((response: Response) => void) | null = null;
    const fetchSpy = installLiveBackend({
      schema: (url, init) => {
        if (
          url.pathname.endsWith("/fields") &&
          url.searchParams.get("connection_id") === "live-influx-2x-a"
        ) {
          fieldsSignal = init?.signal ?? null;
          return new Promise<Response>((resolve) => {
            resolveFields = resolve;
          });
        }
        return defaultSchemaResponse(url);
      },
    });
    const user = userEvent.setup();
    renderPage();
    await activateLive(user);
    await chooseProfiles(user);

    const source = panelFor(/Source schema/i);
    await within(source).findByRole("button", { name: "sensor" });
    await user.click(within(source).getByRole("button", { name: "sensor" }));
    expect(
      await within(source).findByText("Reading fields…"),
    ).toBeInTheDocument();

    await user.selectOptions(
      screen.getByLabelText("Source profile"),
      "live-influx-1x",
    );
    const nextSource = panelFor(/Source schema — LIVE InfluxDB 1\.x/i);
    expect(
      await within(nextSource).findByText("Unknown"),
    ).toBeInTheDocument();

    await waitFor(() => {
      expect(fieldsSignal?.aborted).toBe(true);
    });
    expect(within(nextSource).queryByText("temperature")).toBeNull();

    const resolve = resolveFields as
      | ((response: Response) => void)
      | null;
    resolve?.(
      schemaResponse(
        fieldsSchemaResult({
          connection_id: "live-influx-2x-a",
          scope: "ttk-test-org/ttk-test-bucket",
          measurement: "sensor",
        }),
      ),
    );

    const measurementUrls = fetchSpy.mock.calls
      .map(([input]) => new URL(String(input), "http://localhost"))
      .filter((url) => url.pathname.endsWith("/measurements"));
    expect(
      measurementUrls.some(
        (url) => url.searchParams.get("connection_id") === "live-influx-1x",
      ),
    ).toBe(true);
  });
});

describe("CanonicalMapperPage mapping draft (live, GET-only)", () => {
  it("validates a draft against observed live schema and issues only GET requests", async () => {
    const user = userEvent.setup();
    const fetchSpy = installLiveBackend();
    renderPage();
    await activateLive(user);
    await chooseProfiles(user);
    const section = await screen
      .findByRole("heading", { level: 2, name: "Mapping draft" })
      .then((heading) => heading.closest("section") as HTMLElement);

    await user.type(within(section).getByLabelText("Destination measurement"), "sensor");
    await user.selectOptions(within(section).getByLabelText("Source measurement"), "sensor");
    await user.click(await within(section).findByRole("button", { name: "Map field temperature" }));
    await user.type(
      within(section).getByLabelText("Destination name for field temperature"),
      "_copy",
    );

    await waitFor(() => {
      expect(within(section).getByText(/Plan ready for preview/)).toBeInTheDocument();
    });
    expect(within(section).queryByText(/MOCK draft/)).toBeNull();
    for (const [, init] of fetchSpy.mock.calls) {
      expect((init?.method ?? "GET").toUpperCase()).toBe("GET");
    }
  });
});
