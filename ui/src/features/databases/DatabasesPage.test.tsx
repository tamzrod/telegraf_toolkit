import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { DatabasesPage } from "./DatabasesPage";
import {
  MOCK_REGISTRY_PROFILES,
  registryProfilesResponse,
} from "../../api/mock/registry";

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

afterEach(() => {
  vi.restoreAllMocks();
});

describe("DatabasesPage registry workflow", () => {
  it("loads mock profiles and shows a mock badge and notice", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(registryProfilesResponse());
    renderPage();

    expect(screen.getByText(/Loading registry profiles/i)).toBeInTheDocument();

    const notice = await screen.findByText(/Sample data only/i);
    expect(notice).toBeInTheDocument();
    expect(screen.getAllByText("MOCK").length).toBe(MOCK_REGISTRY_PROFILES.length);
    expect(screen.getByText(/No connection is probed/i)).toBeInTheDocument();
  });

  it("shows three-valued capability statuses without implying support for unknown", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(registryProfilesResponse());
    renderPage();

    await screen.findByText(/Sample data only/i);

    expect(screen.getAllByText("Supported").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Unsupported").length).toBeGreaterThan(0);
    // Each profile reports its capabilities, including schema discovery; the
    // 3.x fixture reports it unsupported (never inferred from version).
    expect(screen.getAllByText(/Schema discovery/).length).toBe(
      MOCK_REGISTRY_PROFILES.length,
    );
    const list = screen.getByRole("heading", { name: /connection profiles/i })
      .parentElement as HTMLElement;
    expect(within(list).getAllByText("Unsupported").length).toBeGreaterThan(0);
  });

  it("renders an explicit absent-credential state for the profile without a reference", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(registryProfilesResponse());
    renderPage();

    await screen.findByText(/Sample data only/i);

    expect(screen.getByText(/No credential reference attached/i)).toBeInTheDocument();
    // References are shown, never values.
    expect(screen.getByText(/read reference/i)).toBeInTheDocument();
    expect(screen.queryByText(/value/i)).not.toBeInTheDocument();
  });

  it("requires an explicit, distinct source and destination selection", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(registryProfilesResponse());
    const user = userEvent.setup();
    renderPage();

    await screen.findByText(/Sample data only/i);

    // Nothing selected yet.
    expect(screen.getByRole("alert")).toHaveTextContent(/both a source and a destination/i);

    await user.selectOptions(screen.getByLabelText(/source database/i), "mock-influx-1x");
    await user.selectOptions(
      screen.getByLabelText(/destination database/i),
      "mock-influx-1x",
    );
    expect(screen.getByRole("alert")).toHaveTextContent(/must be distinct/i);

    await user.selectOptions(
      screen.getByLabelText(/destination database/i),
      "mock-influx-2x",
    );
    // The distinct-pair confirmation replaces the validation alert.
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.getByText(/are distinct and selected/i)).toBeInTheDocument();
  });

  it("validates the unsaved local draft and labels it as unsaved", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(registryProfilesResponse());
    const user = userEvent.setup();
    renderPage();

    await screen.findByText(/Sample data only/i);

    expect(screen.getByText(/A connection ID is required/i)).toBeInTheDocument();

    await user.type(screen.getByLabelText(/connection id/i), "draft-1");
    expect(screen.getByText(/A display name is required/i)).toBeInTheDocument();

    await user.type(screen.getByLabelText(/display name/i), "Draft profile");
    expect(
      screen.getByText(/Draft is locally complete\. It is still unsaved/i),
    ).toBeInTheDocument();
  });

  it("shows an error state with retry when the registry endpoint fails", async () => {
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockRejectedValueOnce(new Error("network down"))
      .mockResolvedValue(registryProfilesResponse());

    const user = userEvent.setup();
    renderPage();

    expect(
      await screen.findByText(/Registry profiles unavailable/i),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /retry/i }));
    await waitFor(() => expect(fetchSpy).toHaveBeenCalledTimes(2));
    expect(await screen.findByText(/Sample data only/i)).toBeInTheDocument();
  });

  it("keeps every mutation control disabled", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(registryProfilesResponse());
    renderPage();

    await screen.findByText(/Sample data only/i);

    const planned = screen
      .getByRole("heading", { level: 2, name: /planned actions/i })
      .closest("section") as HTMLElement;
    for (const button of within(planned).getAllByRole("button")) {
      expect(button).toBeDisabled();
      expect(button).toHaveAttribute("aria-disabled", "true");
    }
  });

  it("requests only the registry endpoint and never the health endpoint", async () => {
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(registryProfilesResponse());
    renderPage();

    await screen.findByText(/Sample data only/i);

    expect(fetchSpy).toHaveBeenCalledTimes(1);
    expect(String(fetchSpy.mock.calls[0][0])).toBe("/api/v1/registry/profiles");
  });
});
