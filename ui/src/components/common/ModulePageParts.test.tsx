import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import {
  DisabledAction,
  DisabledActionGroup,
  NotImplementedNotice,
  PageHeader,
  StatusBadge,
} from "./ModulePageParts";

describe("ModulePageParts", () => {
  it("renders a status badge with a human-readable label", () => {
    render(<StatusBadge status="not implemented" />);
    expect(screen.getByText("Not implemented")).toBeInTheDocument();
  });

  it("renders exactly one level-1 heading through PageHeader", () => {
    render(
      <PageHeader
        id="demo-heading"
        title="Demo"
        scope="Scope text."
        status="scaffold"
      />,
    );
    expect(screen.getAllByRole("heading", { level: 1 })).toHaveLength(1);
    expect(screen.getByText("Scope text.")).toBeInTheDocument();
    expect(screen.getByText("Scaffold")).toBeInTheDocument();
  });

  it("lists the operations that remain unavailable", () => {
    render(<NotImplementedNotice operations={["Do a thing", "Do another"]} />);
    expect(screen.getByRole("status")).toBeInTheDocument();
    expect(screen.getByText("Do a thing")).toBeInTheDocument();
    expect(screen.getByText("Do another")).toBeInTheDocument();
  });

  it("renders disabled actions that cannot be activated", () => {
    render(<DisabledAction label="Deploy" reason="Not authorized yet" />);
    const button = screen.getByRole("button", { name: "Deploy" });
    expect(button).toBeDisabled();
    expect(button).toHaveAttribute("aria-disabled", "true");
    expect(button).toHaveAttribute("title", "Not authorized yet");
  });

  it("renders a labelled group of disabled actions", () => {
    render(
      <DisabledActionGroup
        heading="demo"
        reason="Not authorized yet"
        actions={["One", "Two"]}
      />,
    );
    expect(
      screen.getByRole("heading", { level: 2, name: /planned actions/i }),
    ).toBeInTheDocument();
    for (const name of ["One", "Two"]) {
      expect(screen.getByRole("button", { name })).toBeDisabled();
    }
  });
});
