import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { healthResponse } from "../../api/mock/health";
import { setSessionToken, clearSessionToken } from "../../api/session";
import { App } from "../../app/App";

function renderPipelines() {
  vi.spyOn(globalThis, "fetch").mockResolvedValue(healthResponse());
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={["/pipelines"]}>
        <App />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

afterEach(() => {
  clearSessionToken();
  vi.restoreAllMocks();
});

function revisionPayload(overrides: Record<string, unknown> = {}) {
  return {
    pipeline_id: "p1",
    revision_id: "r1",
    revision: 1,
    state: "draft",
    mode: "graph",
    generation: 1,
    deployed: false,
    document: {},
    has_snapshot: false,
    export_toml: "[agent]\n",
    ...overrides,
  };
}

function headingLevels(container: HTMLElement): number[] {
  return Array.from(container.querySelectorAll("h1, h2, h3, h4, h5, h6")).map((node) =>
    Number(node.tagName.slice(1)),
  );
}

describe("pipeline designer", () => {
  it("preserves an edited field when selection changes and supports undo", async () => {
    const user = userEvent.setup();
    renderPipelines();

    await user.click(screen.getByRole("button", { name: "Add input cpu" }));
    const alias = screen.getByRole("textbox", { name: "cpu-1 Alias" });
    await user.type(alias, "edge-host");
    await user.click(screen.getByRole("button", { name: "Add input mem" }));
    await user.click(screen.getByRole("button", { name: "Select cpu-1" }));

    expect(screen.getByRole("textbox", { name: "cpu-1 Alias" })).toHaveValue("edge-host");
    expect(screen.queryByRole("button", { name: /deploy/i })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Undo" }));
    expect(screen.queryByRole("button", { name: "Select mem-1" })).not.toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "cpu-1 Alias" })).toHaveValue("edge-host");
    await user.click(screen.getByRole("button", { name: "Undo" }));
    expect(screen.getByRole("textbox", { name: "cpu-1 Alias" })).toHaveValue("edge-hos");
  });

  it("reorders processors, deletes an instance, and ignores a presentation nudge", async () => {
    const user = userEvent.setup();
    renderPipelines();

    await user.click(screen.getByRole("button", { name: "Add processor rename" }));
    await user.click(screen.getByRole("button", { name: "Add processor rename" }));
    const edges = () => screen.getByRole("list", { name: "Execution edges" });
    expect(within(edges()).getByText("then: rename-1 to rename-2")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Move rename-2 earlier" }));
    expect(within(edges()).getByText("then: rename-2 to rename-1")).toBeInTheDocument();
    expect(within(edges()).queryByText("then: rename-1 to rename-2")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Select rename-1" }));
    await user.click(screen.getByRole("button", { name: "Nudge rename-1 right" }));
    expect(screen.getByText(/Position 64, 150 is presentation only/)).toBeInTheDocument();
    expect(within(edges()).getByText("then: rename-2 to rename-1")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Delete rename-1" }));
    expect(screen.queryByRole("button", { name: "Select rename-1" })).not.toBeInTheDocument();
    expect(screen.getByText(/No execution edges yet/)).toBeInTheDocument();
  });

  it("searches only the supported catalog", async () => {
    const user = userEvent.setup();
    renderPipelines();

    await user.type(screen.getByRole("searchbox", { name: "Search supported plugins" }), "kafka");
    expect(screen.getByText("No supported plugin matches.")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /kafka/i })).not.toBeInTheDocument();
  });

  it("rejects credential material in the form and does not echo it", async () => {
    const user = userEvent.setup();
    renderPipelines();

    await user.click(screen.getByRole("button", { name: "Add output influxdb_v2" }));
    const token = screen.getByRole("textbox", { name: "influxdb-v2-1 Credential reference" });
    await user.click(token);
    await user.paste("user:placeholder-literal@host");

    expect(token).toHaveValue("");
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Embedded credentials were rejected and were not stored.",
    );
    expect(document.body.textContent).not.toContain("placeholder-literal");

    const urls = screen.getByRole("textbox", { name: "influxdb-v2-1 URLs" });
    expect(urls).toHaveValue("http://127.0.0.1:8086");
    await user.click(urls);
    await user.paste("\nhttp://user:placeholder-literal@127.0.0.1:8086");
    expect(urls).toHaveValue("http://127.0.0.1:8086");
    expect(document.body.textContent).not.toContain("placeholder-literal");
    expect(screen.getByText("Organization is required.")).toBeInTheDocument();
  });

  it("shows fan-out without chaining aggregators or linking two inputs", async () => {
    const user = userEvent.setup();
    renderPipelines();

    expect(
      screen.getByText("No aggregators are configured, so the second processor pass does not run."),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Add input cpu" }));
    await user.click(screen.getByRole("button", { name: "Add input mem" }));
    expect(screen.getByText(/No execution edges yet/)).toBeInTheDocument();
    expect(screen.queryByText(/cpu-1 to mem-1/)).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Add aggregator basicstats" }));
    await user.click(screen.getByRole("button", { name: "Add aggregator basicstats" }));
    const edges = screen.getByRole("list", { name: "Execution edges" });
    expect(within(edges).getByText("fan-out, keeps original: cpu-1 to basicstats-1")).toBeInTheDocument();
    expect(within(edges).getByText("fan-out, keeps original: mem-1 to basicstats-1")).toBeInTheDocument();
    expect(within(edges).getByText("fan-out, keeps original: cpu-1 to basicstats-2")).toBeInTheDocument();
    expect(within(edges).queryByText(/basicstats-1 to basicstats-2/)).not.toBeInTheDocument();
    expect(
      screen.queryByText("No aggregators are configured, so the second processor pass does not run."),
    ).not.toBeInTheDocument();
  });

  it("blocks skipping both processor passes", async () => {
    const user = userEvent.setup();
    renderPipelines();

    const before = screen.getByRole("checkbox", { name: "Skip processors before aggregators" });
    const after = screen.getByRole("checkbox", { name: "Skip processors after aggregators" });
    await user.click(before);
    expect(before).toBeChecked();
    await user.click(after);
    expect(after).not.toBeChecked();
    expect(before).toBeChecked();
    expect(screen.getByRole("alert")).toHaveTextContent("Both processor passes cannot be skipped.");
  });

  it("undoes the last graph edit from the keyboard", async () => {
    const user = userEvent.setup();
    renderPipelines();

    await user.click(screen.getByRole("button", { name: "Add input cpu" }));
    expect(screen.getByRole("button", { name: "Select cpu-1" })).toBeInTheDocument();
    await user.keyboard("{Control>}z{/Control}");
    expect(screen.queryByRole("button", { name: "Select cpu-1" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Undo" })).toBeDisabled();
  });

  it("shows a read-only TOML preview that follows processor order", async () => {
    const user = userEvent.setup();
    renderPipelines();

    const preview = () => screen.getByRole("textbox", { name: "Graph TOML preview" }) as HTMLTextAreaElement;
    expect(preview()).toHaveAttribute("readonly");
    expect(preview().value).toContain("[agent]");
    expect(preview().value).not.toContain("[[inputs.cpu]]");

    await user.click(screen.getByRole("button", { name: "Add processor rename" }));
    await user.click(screen.getByRole("button", { name: "Add processor rename" }));
    const before = preview().value;
    expect(before).toContain('toolkit.instance = "rename-1"');
    expect(before.indexOf('toolkit.instance = "rename-1"')).toBeLessThan(
      before.indexOf('toolkit.instance = "rename-2"'),
    );

    await user.click(screen.getByRole("button", { name: "Move rename-2 earlier" }));
    const after = preview().value;
    expect(after.indexOf('toolkit.instance = "rename-2"')).toBeLessThan(
      after.indexOf('toolkit.instance = "rename-1"'),
    );
    expect(screen.queryByRole("button", { name: /deploy/i })).not.toBeInTheDocument();
  });

  it("keeps one set of landmarks, labelled controls, and a right-side settings column", () => {
    const { container } = renderPipelines();

    expect(screen.getAllByRole("main")).toHaveLength(1);
    expect(screen.getAllByRole("banner")).toHaveLength(1);
    expect(screen.getAllByRole("navigation")).toHaveLength(1);
    expect(screen.getAllByRole("heading", { level: 1 })).toHaveLength(1);
    expect(screen.getByRole("heading", { level: 1, name: "Pipelines" })).toBeInTheDocument();
    const levels = headingLevels(container);
    for (let index = 1; index < levels.length; index += 1) {
      expect(levels[index] - levels[index - 1]).toBeLessThanOrEqual(1);
    }
    const ids = Array.from(container.querySelectorAll("[id]")).map((node) => node.id);
    expect(new Set(ids).size).toBe(ids.length);
    expect(screen.getAllByRole("button", { name: /menu/i })).toHaveLength(1);

    const layout = container.querySelector("[data-layout='canvas-with-settings']");
    expect(layout).toHaveClass("flex-col", "lg:flex-row");
    const settings = screen.getByRole("heading", { level: 2, name: "Node settings" }).closest("aside");
    expect(settings).not.toBeNull();
    expect(settings).toHaveClass("lg:w-80");
    expect(screen.getByLabelText("Search supported plugins")).toBeInTheDocument();
    expect(screen.getByText(/Select an instance to edit its settings/)).toBeInTheDocument();
    expect(screen.getByText(/does not deploy a runtime/i)).toBeInTheDocument();
  });

  it("reviews a manual switch and refuses an edited reverse conversion", async () => {
    const user = userEvent.setup();
    renderPipelines();

    expect(screen.getByRole("button", { name: "Save draft" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Approve revision" })).toBeDisabled();
    expect(screen.getByText(/A session is required to save a draft. Nothing is deployed./)).toBeInTheDocument();
    expect(screen.getByText(/They do not deploy a runtime./)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Review switch to manual" }));
    expect(screen.getByLabelText("Difference review")).toHaveTextContent("Current");
    expect(screen.getByLabelText("Difference review")).toHaveTextContent("[agent]");
    await user.click(screen.getByRole("button", { name: "Confirm mode switch" }));

    const manual = screen.getByRole("textbox", { name: "Manual TOML" }) as HTMLTextAreaElement;
    expect(manual.value).toContain("[agent]");
    expect(screen.queryByRole("textbox", { name: "Graph TOML preview" })).not.toBeInTheDocument();
    await user.type(manual, "# edited");
    await user.click(screen.getByRole("button", { name: "Review switch to graph" }));
    expect(screen.getByRole("alert")).toHaveTextContent(/cannot be converted back into the graph/);

    await user.click(screen.getByRole("button", { name: "Review restore snapshot" }));
    expect(screen.getByLabelText("Difference review")).toHaveTextContent("Proposed");
    await user.click(screen.getByRole("button", { name: "Confirm mode switch" }));
    expect(screen.getByRole("textbox", { name: "Graph TOML preview" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /deploy/i })).not.toBeInTheDocument();
  });

  it("saves a draft revision and reports that nothing was deployed", async () => {
    const user = userEvent.setup();
    setSessionToken("session-token");
    const saved = revisionPayload({ generation: 2, export_toml: "[agent]\n" });
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = String(input);
      if (url.endsWith("/health")) {
        return healthResponse();
      }
      if (url.endsWith("/api/v1/pipelines") && init?.method === "POST") {
        return jsonResponse(revisionPayload());
      }
      if (url.endsWith("/api/v1/pipelines/p1/revisions/1") && init?.method === "PUT") {
        const body = JSON.parse(String(init.body));
        expect(body.document.mode).toBe("graph");
        expect(JSON.stringify(body)).not.toContain("deploy");
        return jsonResponse(saved);
      }
      throw new Error(`unexpected ${init?.method ?? "GET"} ${url}`);
    });
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={["/pipelines"]}>
          <App />
        </MemoryRouter>
      </QueryClientProvider>,
    );

    await user.click(screen.getByRole("button", { name: "Save draft" }));
    expect(await screen.findByText("Draft revision 1 saved. Nothing was deployed.")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /deploy/i })).not.toBeInTheDocument();
  });
});

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}
