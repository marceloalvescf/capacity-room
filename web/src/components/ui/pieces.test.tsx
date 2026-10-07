import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { Checkbox } from "@/components/ui/checkbox";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Segmented } from "@/components/ui/segmented";
import { Select } from "@/components/ui/select";

describe("segmented control", () => {
  it("moves the checked value with the arrow keys", async () => {
    const onChange = vi.fn<(value: "a" | "b") => void>();
    function Harness() {
      const [value, setValue] = useState<"a" | "b">("a");
      return (
        <Segmented
          legend="Choice"
          name="choice"
          value={value}
          options={[
            { value: "a", label: "Alpha" },
            { value: "b", label: "Beta", lang: "pt-BR" },
          ]}
          onChange={(next) => {
            onChange(next);
            setValue(next);
          }}
        />
      );
    }
    const user = userEvent.setup();
    render(<Harness />);
    expect(screen.getByRole("group", { name: "Choice" })).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "Alpha" })).toBeChecked();
    await user.tab();
    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("radio", { name: "Beta" })).toBeChecked();
    expect(onChange).toHaveBeenCalledWith("b");
    expect(screen.getByText("Beta")).toHaveAttribute("lang", "pt-BR");
  });
});

describe("select and checkbox", () => {
  it("wires the error of a select and the indeterminate state of a checkbox", () => {
    render(
      <>
        <Select label="Zone" error="Not allowed" hint="Pick one">
          <option>{"a"}</option>
        </Select>
        <Checkbox label="All" indeterminate />
      </>,
    );
    const select = screen.getByRole("combobox", { name: "Zone" });
    expect(select).toHaveAttribute("aria-invalid", "true");
    const described = (select.getAttribute("aria-describedby") ?? "").split(" ");
    expect(described.map((id) => document.getElementById(id)?.textContent)).toEqual(["Pick one", "Not allowed"]);
    expect(screen.getByRole<HTMLInputElement>("checkbox", { name: "All" }).indeterminate).toBe(true);
  });
});

describe("confirm dialog", () => {
  it("is an alert dialog named by its title, focuses Cancel, and closes on Escape", async () => {
    const onOpenChange = vi.fn<(open: boolean) => void>();
    const onConfirm = vi.fn<() => void>();
    const user = userEvent.setup();
    render(
      <ConfirmDialog
        open
        title="Delete it?"
        body="This cannot be undone."
        confirm="Delete"
        cancel="Cancel"
        onConfirm={onConfirm}
        onOpenChange={onOpenChange}
      />,
    );
    const dialog = await screen.findByRole("alertdialog", { name: "Delete it?" });
    expect(dialog).toHaveAccessibleDescription("This cannot be undone.");
    expect(screen.getByRole("button", { name: "Cancel" })).toHaveFocus();
    await user.keyboard("{Escape}");
    expect(onOpenChange).toHaveBeenCalledWith(false);
    await user.click(screen.getByRole("button", { name: "Delete" }));
    expect(onConfirm).toHaveBeenCalledOnce();
  });
});
