import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { App } from "../../app/App";
import type { Health } from "../../types/health";

const health: Health = {
  status: "ok",
  service: "telegraf-toolkit-api",
  version: "0.0.0-scaffold",
  scaffold: true,
  connected: false,
  description: "scaffold",
};

function renderApp() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <App />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("App scaffold", () => {
  it("labels itself as a scaffold and never claims a live connection", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify(health), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    renderApp();

    expect(screen.getByText(/Scaffold notice/i)).toBeInTheDocument();
    expect(await screen.findByText(/connected: false/i)).toBeInTheDocument();
    expect(screen.getByText(/scaffold: true/i)).toBeInTheDocument();
  });

  it("reports an unreachable backend without pretending to be connected", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("network down"));

    renderApp();

    expect(
      await screen.findByText(/Backend unreachable/i),
    ).toBeInTheDocument();
  });
});
