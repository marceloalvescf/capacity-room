import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { i18next } from "@/i18n/i18n";
import { PeriodPicker } from "./period-picker";

const t = (key: string): string => i18next.t(key);

beforeEach(async () => {
  await i18next.changeLanguage("en");
});

function open(permitted: string[], value = "24h") {
  const onChange = vi.fn();
  render(<PeriodPicker value={value} permitted={permitted} onChange={onChange} />);
  return onChange;
}

async function trigger(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: new RegExp(t("generate.period")) }));
  return screen.findByRole("dialog");
}

describe("period picker", () => {
  it("shows the period in words and opens the quick ranges and the custom period", async () => {
    const user = userEvent.setup();
    open([], "36h");
    expect(screen.getByRole("button", { name: new RegExp(t("generate.period")) })).toHaveTextContent("Last 36 hours");
    const dialog = await trigger(user);
    for (const range of ["1h", "6h", "24h", "7d", "15d", "30d"]) {
      expect(
        within(dialog).getByRole("button", {
          name: t(`period.quick.${range}`),
        }),
      ).toBeInTheDocument();
    }
    expect(within(dialog).getByText(t("period.custom.title"))).toBeInTheDocument();
    expect(within(dialog).queryByRole("heading")).not.toBeInTheDocument();
    expect(within(dialog).getByLabelText(t("period.custom.label"))).toHaveClass("font-mono");
  });

  it("the popover is a dialog named by the Period label", async () => {
    const user = userEvent.setup();
    open([]);
    await trigger(user);
    expect(screen.getByRole("dialog", { name: t("generate.period") })).toBeInTheDocument();
  });

  it("the focus ring of a quick range sits outside the button, on the popover", async () => {
    const user = userEvent.setup();
    open([], "7d");
    const dialog = await trigger(user);
    const pressed = within(dialog).getByRole("button", { name: t("period.quick.7d") });
    expect(pressed).toHaveAttribute("aria-pressed", "true");
    expect(pressed).toHaveClass("focus-visible:outline-offset-2");
    expect(pressed.className).not.toMatch(/outline-offset-\[-/);
  });

  it("a quick range is applied at once", async () => {
    const user = userEvent.setup();
    const onChange = open([]);
    const dialog = await trigger(user);
    await user.click(within(dialog).getByRole("button", { name: t("period.quick.7d") }));
    expect(onChange).toHaveBeenCalledExactlyOnceWith("7d");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("period grammar", async () => {
    const user = userEvent.setup();
    const onChange = open([]);
    const dialog = await trigger(user);
    const input = within(dialog).getByLabelText(t("period.custom.label"));
    const apply = within(dialog).getByRole("button", {
      name: t("period.apply"),
    });
    for (const bad of ["0h", "01h", "1H", "1.5h", "36", "h", ""]) {
      await user.clear(input);
      if (bad !== "") {
        await user.type(input, bad);
      }
      expect({
        bad,
        invalid: within(dialog).queryByText(t("period.invalid")) !== null,
      }).toEqual({ bad, invalid: true });
      expect({ bad, disabled: apply.hasAttribute("disabled") }).toEqual({
        bad,
        disabled: true,
      });
    }
    expect(onChange).not.toHaveBeenCalled();
    await user.clear(input);
    await user.type(input, " 36h ");
    expect(within(dialog).queryByText(t("period.invalid"))).not.toBeInTheDocument();
    await user.click(apply);
    expect(onChange).toHaveBeenCalledExactlyOnceWith("36h");
  });

  it("restricted periods", async () => {
    const user = userEvent.setup();
    const onChange = open(["24h", "7d"]);
    const dialog = await trigger(user);
    expect(within(dialog).getByText(t("period.restricted"))).toBeInTheDocument();
    expect(
      within(dialog)
        .getAllByRole("button")
        .map((button) => button.textContent),
    ).toEqual([t("period.quick.24h"), t("period.quick.7d")]);
    expect(within(dialog).queryByLabelText(t("period.custom.label"))).not.toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: t("period.quick.7d") }));
    expect(onChange).toHaveBeenCalledExactlyOnceWith("7d");
  });

  it("a permitted period outside the quick ranges is worded as a custom one", async () => {
    const user = userEvent.setup();
    open(["90m"]);
    const dialog = await trigger(user);
    expect(within(dialog).getByRole("button", { name: "Last 90 minutes" })).toBeInTheDocument();
  });

  it("a server error is shown beside the period", () => {
    render(<PeriodPicker value="24h" permitted={[]} error="Not allowed." onChange={vi.fn()} />);
    expect(screen.getByText("Not allowed.")).toBeInTheDocument();
  });
});
