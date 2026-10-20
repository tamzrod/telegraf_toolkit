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
    nonOverviewModules.map((module) => [module.label, module.path] as const),
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

  it("keeps Overview as the only scaffold-labeled module", () => {
    const scaffoldModules = MODULES.filter(
      (module) => module.status === "scaffold",
    );
    expect(scaffoldModules.map((module) => module.id)).toEqual(["overview"]);
  });
});
