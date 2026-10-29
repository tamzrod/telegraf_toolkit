import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CanonicalMapperPage } from "./CanonicalMapperPage";
import { registryProfilesResponse } from "../../api/mock/registry";
import { clearSessionToken } from "../../api/session";

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

function draftSection(): HTMLElement {
  const heading = screen.getByRole("heading", { level: 2, name: "Mapping draft" });
  return heading.closest("section") as HTMLElement;
}

async function chooseMockPair(user: UserEvent) {
  vi.spyOn(globalThis, "fetch").mockResolvedValue(registryProfilesResponse());
  renderPage();
  await screen.findByText(/Mock workflow/i);
  await waitFor(() => {
    expect(screen.getByLabelText("Source profile")).toBeEnabled();
  });
  await user.selectOptions(screen.getByLabelText("Source profile"), "mock-influx-2x");
  await user.selectOptions(screen.getByLabelText("Destination profile"), "mock-influx-3x");
  await screen.findByRole("heading", { level: 2, name: "Mapping draft" });
}

function dataTransferWith(payload: string) {
  return {
    dropEffect: "none",
    effectAllowed: "all",
    getData: (type: string) => (type === "text/plain" ? payload : ""),
    setData: () => undefined,
  };
}

describe("Canonical Mapper mapping draft (mock)", () => {
  it("starts blocked with an explicit local-only, mock-labeled draft and no plan preview", async () => {
    const user = userEvent.setup();
    await chooseMockPair(user);
    const section = draftSection();
    expect(within(section).getByText(/MOCK draft/)).toBeInTheDocument();
    expect(within(section).getByText(/nothing is saved, exported, written or executed/i)).toBeInTheDocument();
    expect(within(section).getByText(/Plan blocked/)).toBeInTheDocument();
    expect(within(section).getByText(/No preview while the plan is blocked/)).toBeInTheDocument();
  });

  it("builds a valid plan through keyboard-operable buttons and shows it only as quoted text", async () => {
    const user = userEvent.setup();
    await chooseMockPair(user);
    const section = draftSection();

    await user.selectOptions(within(section).getByLabelText("Source measurement"), "sensor");
    await user.click(await within(section).findByRole("button", { name: "Map field temperature" }));

    const nameInput = within(section).getByLabelText("Destination name for field temperature");
    await user.clear(nameInput);
    await user.type(nameInput, "temp_c");
    await user.type(within(section).getByLabelText("Destination measurement"), "sensor_archive");

    await waitFor(() => {
      expect(within(section).getByText(/Plan ready for preview/)).toBeInTheDocument();
    });
    const plan = within(section).getByRole("list", { name: "Mapping plan lines" });
    expect(within(plan).getByText('Field "temperature" → "temp_c" (float); no transform (types must match)')).toBeInTheDocument();
  });

  it("refuses duplicate destination field names and blocks the preview", async () => {
    const user = userEvent.setup();
    await chooseMockPair(user);
    const section = draftSection();
    await user.selectOptions(within(section).getByLabelText("Source measurement"), "sensor");
    await user.click(await within(section).findByRole("button", { name: "Map field temperature" }));
    await user.click(within(section).getByRole("button", { name: "Map field humidity" }));
    await user.clear(within(section).getByLabelText("Destination name for field humidity"));
    await user.type(within(section).getByLabelText("Destination name for field humidity"), "temperature");
    await user.type(within(section).getByLabelText("Destination measurement"), "sensor_archive");

    const diagnostics = within(section).getByRole("list", { name: "Draft diagnostics" });
    expect(within(diagnostics).getAllByText("duplicate-destination-field").length).toBeGreaterThan(0);
    expect(within(section).getByText(/Plan blocked/)).toBeInTheDocument();
    expect(within(section).queryByRole("list", { name: "Mapping plan lines" })).toBeNull();
  });

  it("never coerces a type change and accepts only an explicit unit transform", async () => {
    const user = userEvent.setup();
    await chooseMockPair(user);
    const section = draftSection();
    await user.selectOptions(within(section).getByLabelText("Source measurement"), "sensor");
    await user.click(await within(section).findByRole("button", { name: "Map field value" }));
    await user.type(within(section).getByLabelText("Destination measurement"), "sensor_archive");
    await user.selectOptions(within(section).getByLabelText("Destination type for field value"), "float");

    expect(within(section).getByText("type-mismatch")).toBeInTheDocument();

    await user.selectOptions(within(section).getByLabelText("Transform for field value"), "unit");
    await user.type(within(section).getByLabelText("Conversion factor for field value"), "0.1");
    await user.type(within(section).getByLabelText("From unit for field value"), "raw");
    await user.type(within(section).getByLabelText("To unit for field value"), "celsius");

    await waitFor(() => {
      expect(within(section).queryByText("type-mismatch")).toBeNull();
    });
    expect(within(section).queryByText("transform-factor-invalid")).toBeNull();
  });

  it("maps a dragged source field and refuses a payload that is not observed", async () => {
    const user = userEvent.setup();
    await chooseMockPair(user);
    const section = draftSection();
    await user.selectOptions(within(section).getByLabelText("Source measurement"), "sensor");
    await within(section).findByRole("button", { name: "Map field humidity" });

    const zone = within(section).getByRole("group", { name: "Drop zone" });
    fireEvent.drop(zone, { dataTransfer: dataTransferWith("field:humidity") });
    expect(await within(section).findByLabelText("Destination name for field humidity")).toBeInTheDocument();

    fireEvent.drop(zone, { dataTransfer: dataTransferWith("field:ghost") });
    expect(
      within(section).getByText(/"ghost" is not observed on the source measurement/),
    ).toBeInTheDocument();
    expect(within(section).queryByLabelText("Destination name for field ghost")).toBeNull();
  });

  it("holds an injection-shaped name as invalid data and renders filter text as quoted literal", async () => {
    const user = userEvent.setup();
    await chooseMockPair(user);
    const section = draftSection();
    await user.selectOptions(within(section).getByLabelText("Source measurement"), "sensor");
    await user.click(await within(section).findByRole("button", { name: "Map field temperature" }));
    const dest = within(section).getByLabelText("Destination name for field temperature");
    await user.clear(dest);
    await user.type(dest, 'x"; DROP MEASUREMENT sensor; --');
    await user.type(within(section).getByLabelText("Destination measurement"), "sensor_archive");
    expect(within(section).getByText("destination-field-invalid")).toBeInTheDocument();
    expect(within(section).queryByRole("list", { name: "Mapping plan lines" })).toBeNull();

    await user.click(within(section).getByRole("button", { name: "Filter on tag host" }));
    const filterValue = within(section).getByLabelText("Filter value for tag host");
    await user.type(filterValue, `alpha' OR '1'='1`);
    await user.clear(dest);
    await user.type(dest, "temp_c");
    await waitFor(() => {
      expect(within(section).getByText(/Plan ready for preview/)).toBeInTheDocument();
    });
    const plan = within(section).getByRole("list", { name: "Mapping plan lines" });
    expect(
      within(plan).getByText(`Filter: tag "host" equals "alpha' OR '1'='1" (value is data)`),
    ).toBeInTheDocument();
  });

  it("refuses a filter with a control character and an empty filter value", async () => {
    const user = userEvent.setup();
    await chooseMockPair(user);
    const section = draftSection();
    await user.selectOptions(within(section).getByLabelText("Source measurement"), "sensor");
    await user.click(await within(section).findByRole("button", { name: "Filter on tag sensor_id" }));
    // A newly added filter has no value yet, which is itself refused.
    expect(within(section).getByText("filter-value-invalid")).toBeInTheDocument();

    fireEvent.change(within(section).getByLabelText("Filter value for tag sensor_id"), {
      target: { value: "a\u0007b" },
    });
    expect(within(section).getAllByText("filter-value-invalid").length).toBeGreaterThan(0);
    expect(within(section).queryByRole("list", { name: "Mapping plan lines" })).toBeNull();
  });

  it("warns that an unknown destination cannot be checked without blocking the plan", async () => {
    const user = userEvent.setup();
    await chooseMockPair(user);
    const section = draftSection();
    await user.selectOptions(within(section).getByLabelText("Source measurement"), "sensor");
    await user.click(await within(section).findByRole("button", { name: "Map field temperature" }));
    await user.clear(within(section).getByLabelText("Destination name for field temperature"));
    await user.type(within(section).getByLabelText("Destination name for field temperature"), "temp_c");
    await user.type(within(section).getByLabelText("Destination measurement"), "sensor_archive");

    expect(within(section).getByText("destination-schema-unknown")).toBeInTheDocument();
    expect(within(section).getByText(/Plan ready for preview/)).toBeInTheDocument();
  });

  it("discards the draft when either selected profile changes", async () => {
    const user = userEvent.setup();
    await chooseMockPair(user);
    let section = draftSection();
    await user.selectOptions(within(section).getByLabelText("Source measurement"), "sensor");
    await user.click(await within(section).findByRole("button", { name: "Map field temperature" }));
    expect(within(section).getByLabelText("Destination name for field temperature")).toBeInTheDocument();

    await user.selectOptions(screen.getByLabelText("Destination profile"), "mock-influx-2x");
    await user.selectOptions(screen.getByLabelText("Source profile"), "mock-influx-2x");
    await user.selectOptions(screen.getByLabelText("Destination profile"), "mock-influx-3x");
    section = draftSection();
    expect(within(section).queryByLabelText("Destination name for field temperature")).toBeNull();
    expect(within(section).getByText(/No field rules/)).toBeInTheDocument();
  });
});
