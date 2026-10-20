import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { StateDemo } from "./StateDemo";
import {
  EmptyState,
  ErrorState,
  LoadingState,
  MockDataNotice,
  PermissionDeniedState,
} from "./StatePanels";

describe("StatePanels", () => {
  it("labels mock data as sample-only and non-live", () => {
    render(<MockDataNotice subject="runtime list" />);
    const notice = screen.getByText(/Sample data only/i).closest("p");
    expect(notice).not.toBeNull();
    expect(notice).toHaveTextContent(/runtime list/);
    expect(notice).toHaveTextContent(/not read from any database/i);
    expect(notice).toHaveTextContent(/does not represent a live connection/i);
  });

  it("announces a loading state politely", () => {
    render(<LoadingState label="Loading things…" />);
    const status = screen.getByRole("status");
    expect(status).toHaveTextContent("Loading things…");
    expect(status).toHaveAttribute("aria-live", "polite");
  });

  it("renders an empty state", () => {
    render(<EmptyState title="Nothing here" description="Add one to begin." />);
    expect(screen.getByRole("status")).toHaveTextContent("Nothing here");
    expect(screen.getByText("Add one to begin.")).toBeInTheDocument();
  });

  it("renders an error state with a working retry control", async () => {
    const onRetry = vi.fn();
    const user = userEvent.setup();
    render(
      <ErrorState
        title="Failed"
        description="Try again."
        onRetry={onRetry}
      />,
    );

    expect(screen.getByRole("alert")).toHaveTextContent("Failed");
    await user.click(screen.getByRole("button", { name: /retry/i }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it("renders an error state without a retry control when none is given", () => {
    render(<ErrorState title="Failed" description="No retry available." />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("states that a permission denial is illustrative, not enforced", () => {
    render(
      <PermissionDeniedState role="Viewer" capability="manage runtimes" />,
    );
    const panel = screen.getByRole("status");
    expect(panel).toHaveTextContent(/Illustrative permission state/i);
    expect(panel).toHaveTextContent(/not enforced/i);
    expect(panel).toHaveTextContent(/no authentication or server-side authorization/i);
  });
});

describe("StateDemo", () => {
  it("defaults to the sample-data state", () => {
    render(<StateDemo subject="runtime list" />);
    expect(screen.getByText(/Sample data only/i)).toBeInTheDocument();
  });

  it("switches between labeled states without a network call", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch");
    const user = userEvent.setup();
    render(<StateDemo subject="runtime list" />);

    const group = screen.getByRole("group", { name: /demonstration state/i });

    await user.click(screen.getByRole("button", { name: "Loading" }));
    expect(screen.getByRole("status")).toHaveTextContent(/Loading runtime list/i);

    await user.click(screen.getByRole("button", { name: "Empty" }));
    expect(screen.getByRole("status")).toHaveTextContent(/No runtime list available/i);

    await user.click(screen.getByRole("button", { name: "Permission" }));
    expect(screen.getByRole("status")).toHaveTextContent(/not enforced/i);

    await user.click(screen.getByRole("button", { name: "Error" }));
    expect(screen.getByRole("alert")).toHaveTextContent(/Could not load runtime list/i);

    // The active state is reflected for assistive technology.
    expect(
      screen.getByRole("button", { name: "Error", pressed: true }),
    ).toBeInTheDocument();
    expect(group).toBeInTheDocument();

    expect(fetchSpy).not.toHaveBeenCalled();
    fetchSpy.mockRestore();
  });

  it("counts local retry selections without contacting a backend", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch");
    const user = userEvent.setup();
    render(<StateDemo subject="runtime list" />);

    await user.click(screen.getByRole("button", { name: "Error" }));
    await user.click(screen.getByRole("button", { name: /retry/i }));
    await user.click(screen.getByRole("button", { name: /retry/i }));

    expect(screen.getByText(/Retry selected 2 times/i)).toBeInTheDocument();
    expect(fetchSpy).not.toHaveBeenCalled();
    fetchSpy.mockRestore();
  });
});
