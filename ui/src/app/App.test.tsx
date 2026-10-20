import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";
import { MODULES } from "./modules";
import type { Health } from "../types/health";

const health: Health = {
  status: "ok",
  service: "telegraf-toolkit-api",
  version: "0.0.0-scaffold",
  scaffold: true,
  connected: false,
  description: "scaffold",
};

function renderApp(initialPath = "/") {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialPath]}>
        <App />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function mockHealthyBackend() {
  vi.spyOn(globalThis, "fetch").mockResolvedValue(
    new Response(JSON.stringify(health), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("application shell", () => {
  it("renders the primary navigation with every registered module", () => {
    mockHealthyBackend();
    renderApp();

    const nav = screen.getByRole("navigation", { name: /primary/i });
    for (const module of MODULES) {
      expect(
        within(nav).getByRole("link", { name: module.label }),
      ).toBeInTheDocument();
    }
  });

  it("navigates directly to a module route without a click", () => {
    mockHealthyBackend();
    renderApp("/synchronizer");

    expect(
      screen.getByRole("heading", { level: 1, name: "Synchronizer" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent(/Not implemented/i);
  });

  it("navigates between modules through the sidebar links", async () => {
    mockHealthyBackend();
    const user = userEvent.setup();
    renderApp();

    const nav = screen.getByRole("navigation", { name: /primary/i });
    await user.click(within(nav).getByRole("link", { name: "Canonical Mapper" }));

    expect(
      screen.getByRole("heading", { level: 1, name: "Canonical Mapper" }),
    ).toBeInTheDocument();
    expect(
      within(nav).getByRole("link", { name: "Canonical Mapper" }),
    ).toHaveAttribute("aria-current", "page");
  });

  it("shows an explicit not-found page for unknown paths", () => {
    mockHealthyBackend();
    renderApp("/does-not-exist");

    expect(
      screen.getByRole("heading", { level: 1, name: /page not found/i }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Not implemented/i)).not.toBeInTheDocument();
  });

  it("supports back and forward navigation between modules", async () => {
    mockHealthyBackend();
    const user = userEvent.setup();
    window.history.pushState({}, "", "/");
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <App />
        </BrowserRouter>
      </QueryClientProvider>,
    );

    const nav = screen.getByRole("navigation", { name: /primary/i });
    await user.click(within(nav).getByRole("link", { name: "Databases" }));
    expect(
      screen.getByRole("heading", { level: 1, name: "Databases" }),
    ).toBeInTheDocument();

    await user.click(within(nav).getByRole("link", { name: "Runtimes" }));
    expect(
      screen.getByRole("heading", { level: 1, name: "Runtimes" }),
    ).toBeInTheDocument();

    window.history.back();
    await waitFor(() =>
      expect(
        screen.getByRole("heading", { level: 1, name: "Databases" }),
      ).toBeInTheDocument(),
    );

    window.history.forward();
    await waitFor(() =>
      expect(
        screen.getByRole("heading", { level: 1, name: "Runtimes" }),
      ).toBeInTheDocument(),
    );
  });

  it("toggles the mobile navigation menu for narrow viewports", async () => {
    mockHealthyBackend();
    const user = userEvent.setup();
    renderApp();

    const toggle = screen.getByRole("button", { name: /menu/i });
    expect(toggle).toHaveAttribute("aria-expanded", "false");

    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "true");

    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "false");
  });

  it("exposes a skip link and a labelled main region for keyboard users", () => {
    mockHealthyBackend();
    renderApp();

    expect(
      screen.getByRole("link", { name: /skip to main content/i }),
    ).toHaveAttribute("href", "#main-content");
    expect(screen.getByRole("main")).toHaveAttribute("id", "main-content");
  });

  it("makes no network call other than the approved scaffold health endpoint", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify(health), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    // Databases has an authorized mock registry workflow that calls the typed
    // registry endpoint, so it is exercised separately (see DatabasesPage.test).
    for (const module of MODULES.filter(
      (entry) => entry.path !== "/" && entry.id !== "databases",
    )) {
      renderApp(module.path);
      await screen.findByRole("heading", { level: 1, name: module.label });
    }

    for (const [url] of fetchSpy.mock.calls) {
      expect(String(url)).toBe("/api/v1/health");
    }
  });
});
