import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";
import { MODULES } from "./modules";
import { MOCK_HEALTH, healthResponse } from "../api/mock/health";

function renderAt(path: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[path]}>
        <App />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

const ALL_PATHS = MODULES.map((module) => module.path);

function headingLevels(container: HTMLElement): number[] {
  return Array.from(container.querySelectorAll("h1, h2, h3, h4, h5, h6")).map(
    (node) => Number(node.tagName.slice(1)),
  );
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("semantic landmarks and headings", () => {
  it.each(ALL_PATHS)(
    "%s exposes exactly one main landmark, one banner and one labelled nav",
    (path) => {
      vi.spyOn(globalThis, "fetch").mockResolvedValue(healthResponse());

      renderAt(path);

      expect(screen.getAllByRole("main")).toHaveLength(1);
      expect(screen.getAllByRole("banner")).toHaveLength(1);
      expect(screen.getAllByRole("navigation", { name: /primary/i })).toHaveLength(1);
    },
  );

  it.each(ALL_PATHS)("%s has exactly one level-1 heading", (path) => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(healthResponse());

    const { container } = renderAt(path);

    expect(container.querySelectorAll("h1")).toHaveLength(1);
  });

  it.each(ALL_PATHS)("%s does not skip heading levels", (path) => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(healthResponse());

    const { container } = renderAt(path);
    const levels = headingLevels(container);

    for (let index = 1; index < levels.length; index += 1) {
      expect(levels[index] - levels[index - 1]).toBeLessThanOrEqual(1);
    }
  });

  it.each(ALL_PATHS)("%s contains no duplicate element ids", (path) => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(healthResponse());

    const { container } = renderAt(path);
    const ids = Array.from(container.querySelectorAll("[id]")).map(
      (node) => node.id,
    );

    expect(new Set(ids).size).toBe(ids.length);
  });
});

describe("keyboard navigation", () => {
  it("makes the skip link the first tab stop and targets the main region", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(healthResponse());
    const user = userEvent.setup();

    renderAt("/");

    await user.tab();
    const firstStop = document.activeElement;
    expect(firstStop).toHaveAccessibleName(/skip to main content/i);
    expect(firstStop).toHaveAttribute("href", "#main-content");
  });

  it("reaches every primary navigation link by keyboard", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(healthResponse());
    const user = userEvent.setup();

    renderAt("/");

    const nav = screen.getByRole("navigation", { name: /primary/i });
    const expected = MODULES.map((module) => module.label);
    const reached: string[] = [];

    // The skip link is the first stop; the menu toggle follows on narrow layouts.
    for (let index = 0; index < MODULES.length + 4; index += 1) {
      await user.tab();
      const active = document.activeElement;
      if (active && nav.contains(active)) {
        reached.push(active.textContent?.trim() ?? "");
      }
    }

    for (const label of expected) {
      expect(reached).toContain(label);
    }
  });

  it("activates a navigation link with the keyboard and moves focus target", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(healthResponse());
    const user = userEvent.setup();

    renderAt("/");

    const nav = screen.getByRole("navigation", { name: /primary/i });
    const runtimesLink = within(nav).getByRole("link", { name: "Runtimes" });
    runtimesLink.focus();
    expect(runtimesLink).toHaveFocus();

    await user.keyboard("{Enter}");

    expect(
      screen.getByRole("heading", { level: 1, name: "Runtimes" }),
    ).toBeInTheDocument();
  });

  it("exposes the responsive menu toggle to keyboard users with state", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(healthResponse());
    const user = userEvent.setup();

    renderAt("/");

    const toggle = screen.getByRole("button", { name: /menu/i });
    expect(toggle).toHaveAttribute("aria-controls", "app-sidebar");
    expect(toggle).toHaveAttribute("aria-expanded", "false");

    toggle.focus();
    await user.keyboard("{Enter}");
    expect(toggle).toHaveAttribute("aria-expanded", "true");
  });
});

describe("route smoke", () => {
  it.each(ALL_PATHS)("%s renders without error and shows its title", (path) => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(healthResponse());

    const module = MODULES.find((entry) => entry.path === path);
    expect(module).toBeDefined();

    renderAt(path);

    expect(
      screen.getByRole("heading", { level: 1, name: module?.label }),
    ).toBeInTheDocument();
  });

  it("renders the not-found page for an unknown route", () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(healthResponse());

    renderAt("/unknown-route");

    expect(
      screen.getByRole("heading", { level: 1, name: /page not found/i }),
    ).toBeInTheDocument();
    expect(screen.getByRole("main")).toBeInTheDocument();
  });

  it("labels the scaffold health payload without claiming a live connection", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(healthResponse());

    renderAt("/");

    expect(await screen.findByText(/connected: false/i)).toBeInTheDocument();
    expect(screen.getByText(/scaffold: true/i)).toBeInTheDocument();
    expect(MOCK_HEALTH.connected).toBe(false);
  });

  it("announces interactive status regions for assistive technology", () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(healthResponse());

    renderAt("/runtimes");

    const statuses = screen.getAllByRole("status");
    expect(statuses.length).toBeGreaterThan(0);
  });
});
