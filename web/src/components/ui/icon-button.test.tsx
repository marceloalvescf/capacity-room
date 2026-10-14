import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { withTooltips } from "../../../tests/render";
import { IconButton } from "./icon-button";

async function focusTooltip(ui: React.ReactElement) {
  render(withTooltips(ui));
  await userEvent.setup().tab();
  return screen.findByRole("tooltip");
}

describe("icon button tooltip", () => {
  it("is drawn inside the header that holds its button", async () => {
    const tooltip = await focusTooltip(
      <header>
        <IconButton label="Theme" />
      </header>,
    );
    expect(tooltip.closest("header")).not.toBeNull();
  });

  it("is drawn inside the main region that holds its button", async () => {
    const tooltip = await focusTooltip(
      <main>
        <span>
          <IconButton label="Copy" />
        </span>
      </main>,
    );
    expect(tooltip.closest("main")).not.toBeNull();
  });

  it("stays in the body when no landmark holds its button", async () => {
    const tooltip = await focusTooltip(
      <div>
        <IconButton label="Pick" />
      </div>,
    );
    expect(tooltip.closest("main, header, nav, aside, footer")).toBeNull();
  });
});
