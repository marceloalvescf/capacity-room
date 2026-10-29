import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { i18next } from "@/i18n/i18n";
import { withTooltips } from "../../tests/render";
import { NamespacePicker } from "./namespace-picker";

const names = ["checkout", "payments", "payments-batch"];

function Harness({
  list = names,
  initial = [],
  error,
  onChange,
}: {
  list?: readonly string[] | null;
  initial?: string[];
  error?: string;
  onChange?: (selected: string[]) => void;
}) {
  const [selected, setSelected] = useState(initial);
  return withTooltips(
    <NamespacePicker
      label="Namespaces"
      names={list}
      unknownText="Test to list"
      selected={selected}
      error={error}
      onChange={(next) => {
        setSelected(next);
        onChange?.(next);
      }}
    />,
  );
}

beforeEach(async () => {
  await i18next.changeLanguage("en");
});

describe("namespace picker", () => {
  it("shows the chosen names as chips", () => {
    render(<Harness initial={["payments", "checkout"]} />);
    expect(screen.getByText("payments")).toBeInTheDocument();
    expect(screen.getByText("checkout")).toBeInTheDocument();
    expect(screen.queryByText("not found now")).not.toBeInTheDocument();
  });

  it("removes a chip", async () => {
    const onChange = vi.fn();
    render(<Harness initial={["checkout", "payments"]} onChange={onChange} />);
    await userEvent.setup().click(screen.getByRole("button", { name: "Remove checkout" }));
    expect(onChange).toHaveBeenCalledWith(["payments"]);
  });

  it("opens the popover with the list, the count, and the filter", async () => {
    render(<Harness initial={["checkout"]} />);
    await userEvent.setup().click(screen.getByRole("button", { name: /Add a namespace/ }));
    expect(await screen.findAllByRole("checkbox")).toHaveLength(3);
    expect(screen.getByText("Found: 3 · Selected: 1")).toBeInTheDocument();
    expect(screen.getByRole("searchbox", { name: "Filter namespaces" })).toBeInTheDocument();
  });

  it("filters the list", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.click(screen.getByRole("button", { name: /Add a namespace/ }));
    await user.type(await screen.findByRole("searchbox"), "batch");
    expect(screen.getAllByRole("checkbox")).toHaveLength(1);
    await user.clear(screen.getByRole("searchbox"));
    await user.type(screen.getByRole("searchbox"), "zzz");
    expect(screen.getByText("No namespace matches the filter.")).toBeInTheDocument();
  });

  it("calls onChange with a sorted list", async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<Harness initial={["payments"]} onChange={onChange} />);
    await user.click(screen.getByRole("button", { name: /Add a namespace/ }));
    await user.click(await screen.findByRole("checkbox", { name: "checkout" }));
    expect(onChange).toHaveBeenLastCalledWith(["checkout", "payments"]);
    await user.click(screen.getByRole("checkbox", { name: "payments-batch" }));
    expect(onChange).toHaveBeenLastCalledWith(["checkout", "payments", "payments-batch"]);
  });

  it("clears the selection", async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();
    render(<Harness initial={["checkout", "payments"]} onChange={onChange} />);
    await user.click(screen.getByRole("button", { name: /Add a namespace/ }));
    await user.click(await screen.findByRole("button", { name: "Clear" }));
    expect(onChange).toHaveBeenLastCalledWith([]);
  });

  it("marks a chosen name the source does not show and lists it first", async () => {
    const user = userEvent.setup();
    render(<Harness initial={["gone"]} />);
    expect(screen.getByText("not found now")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /Add a namespace/ }));
    const boxes = await screen.findAllByRole("checkbox");
    expect(boxes).toHaveLength(4);
    expect(boxes[0]).toBe(screen.getByRole("checkbox", { name: `gone ${i18next.t("namespaces.notFound")}` }));
    expect(within(boxes[0]?.closest("[role=group]") as HTMLElement).getAllByText("not found now")).toHaveLength(1);
  });

  it("shows the unknown text and marks nothing when the names are null", async () => {
    const user = userEvent.setup();
    render(<Harness list={null} initial={["checkout"]} />);
    expect(screen.queryByText("not found now")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /Add a namespace/ }));
    expect(await screen.findByText("Test to list")).toBeInTheDocument();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    expect(screen.queryByRole("searchbox")).not.toBeInTheDocument();
    expect(screen.queryByText(/Found:/)).not.toBeInTheDocument();
  });

  it("describes the error", () => {
    render(<Harness error="Choose at least one namespace." />);
    const group = screen.getByRole("group", { name: "Namespaces" });
    expect(group).toHaveAttribute("aria-invalid", "true");
    expect(group).toHaveAccessibleDescription("Choose at least one namespace.");
  });
});
