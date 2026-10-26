import { render, screen, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { App } from "../app/App";
import { MODULES } from "../app/modules";
import type { Health } from "../types/health";

const health: Health = {
  status: "ok",
  service: "telegraf-toolkit-api",
  version: "0.0.0-scaffold",
  scaffold: true,
  connected: false,
  description: "scaffold",
};

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

afterEach(() => {
  vi.restoreAllMocks();
});

vi.spyOn(globalThis, "fetch").mockResolvedValue(
  new Response(JSON.stringify(health), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  }),
);

describe("feature module entry points", () => {
  const nonOverviewModules = MODULES.filter((module) => module.path !== "/");
  // Databases gained an authorized, mock-only Phase 2 registry workflow and
  // Canonical Mapper gained the read-only P3-003 schema browser, so neither is
  // an inert shell. Their safety boundaries are asserted separately.
  const inertModules = nonOverviewModules.filter(
    (module) => module.id !== "databases" && module.id !== "canonical-mapper",
  );

  it("registers exactly the eight primary areas", () => {
    expect(MODULES).toHaveLength(8);
    expect(nonOverviewModules).toHaveLength(7);
  });

  it.each(
    MODULES.map((module) => [module.label, module.path] as const),
  )("%s has a reachable distinct page at %s", async (label, path) => {
    renderAt(path);

    const headings = await screen.findAllByRole("heading", { level: 1 });
    expect(headings).toHaveLength(1);
    expect(headings[0]).toHaveTextContent(label);
  });

  it.each(
    inertModules.map((module) => [module.label, module.path] as const),
  )(
    "%s is visibly non-operational with disabled actions",
    async (_label, path) => {
      renderAt(path);

      const notice = await screen.findByRole("status");
      expect(notice).toHaveTextContent(/Not implemented/i);

      const plannedActions = screen
        .getByRole("heading", { level: 2, name: /planned actions/i })
        .closest("section");
      expect(plannedActions).not.toBeNull();

      const buttons = within(plannedActions as HTMLElement).getAllByRole(
        "button",
      );
      expect(buttons.length).toBeGreaterThan(0);
      for (const button of buttons) {
        expect(button).toBeDisabled();
        expect(button).toHaveAttribute("aria-disabled", "true");
      }
    },
  );

  it("keeps Databases labeled as partially implemented with disabled mutation controls", async () => {
    renderAt("/databases");

    const notice = await screen.findByText(/Mock workflow/i);
    expect(notice).toBeInTheDocument();

    const plannedActions = screen
      .getByRole("heading", { level: 2, name: /planned actions/i })
      .closest("section") as HTMLElement;
    const buttons = within(plannedActions).getAllByRole("button");
    expect(buttons.length).toBeGreaterThan(0);
    for (const button of buttons) {
      expect(button).toBeDisabled();
      expect(button).toHaveAttribute("aria-disabled", "true");
    }
  });

  it("keeps Canonical Mapper labeled as a partial, read-only schema browser", async () => {
    renderAt("/canonical-mapper");

    expect(await screen.findByText(/Mock workflow/i)).toBeInTheDocument();
    expect(screen.getByText(/Read-only bounds/i)).toBeInTheDocument();

    const plannedActions = screen
      .getByRole("heading", { level: 2, name: /planned actions/i })
      .closest("section") as HTMLElement;
    const buttons = within(plannedActions).getAllByRole("button");
    expect(buttons.length).toBeGreaterThan(0);
    for (const button of buttons) {
      expect(button).toBeDisabled();
      expect(button).toHaveAttribute("aria-disabled", "true");
    }
  });

  it("keeps Overview as the only scaffold-labeled module", () => {
    const scaffoldModules = MODULES.filter(
      (module) => module.status === "scaffold",
    );
    expect(scaffoldModules.map((module) => module.id)).toEqual(["overview"]);
  });

  it("labels Databases and Canonical Mapper as partially implemented, not live", () => {
    const databases = MODULES.find((module) => module.id === "databases");
    expect(databases?.status).toBe("partial");
    const canonical = MODULES.find((module) => module.id === "canonical-mapper");
    expect(canonical?.status).toBe("partial");
  });
});
