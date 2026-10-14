import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { i18next } from "@/i18n/i18n";
import { PeriodPicker } from "./period-picker";
import type { PeriodValue } from "./period";
import { withTooltips } from "../../../tests/render";

const t = (key: string, options: Record<string, string> = {}): string => i18next.t(key, options);

beforeEach(async () => {
  await i18next.changeLanguage("en");
  vi.useFakeTimers({ shouldAdvanceTime: true, toFake: ["Date"] });
  vi.setSystemTime(new Date("2026-10-07T21:00:42Z"));
});

afterEach(() => {
  vi.useRealTimers();
});

const zone = "America/Sao_Paulo";
const length = (period: string): PeriodValue => ({ kind: "length", period });
const range = (start: string, end: string): PeriodValue => ({ kind: "range", start, end });

function open(permitted: string[], value: PeriodValue = length("24h")) {
  const onChange = vi.fn();
  render(withTooltips(<PeriodPicker value={value} zone={zone} permitted={permitted} onChange={onChange} />));
  return onChange;
}

async function trigger(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: new RegExp(t("generate.period")) }));
  return screen.findByRole("dialog");
}

const from = (dialog: HTMLElement) => within(dialog).getByLabelText<HTMLInputElement>(t("period.range.from"));
const to = (dialog: HTMLElement) => within(dialog).getByLabelText<HTMLInputElement>(t("period.range.to"));
const apply = (dialog: HTMLElement) => within(dialog).getByRole("button", { name: t("period.apply") });

async function retype(user: ReturnType<typeof userEvent.setup>, input: HTMLElement, text: string) {
  await user.clear(input);
  await user.type(input, text);
}

describe("period picker", () => {
  it("shows the period in words and opens the quick ranges and the length", async () => {
    const user = userEvent.setup();
    open([], length("36h"));
    expect(screen.getByRole("button", { name: new RegExp(t("generate.period")) })).toHaveTextContent("Last 36 hours");
    const dialog = await trigger(user);
    for (const quick of ["1h", "6h", "24h", "7d", "15d", "30d"]) {
      expect(within(dialog).getByRole("button", { name: t(`period.quick.${quick}`) })).toBeInTheDocument();
    }
    expect(within(dialog).getByRole("textbox", { name: t("period.custom.label") })).toHaveClass("font-mono");
    expect(within(dialog).queryByRole("heading")).not.toBeInTheDocument();
  });

  it("the popover is a dialog named by the Period label", async () => {
    const user = userEvent.setup();
    open([]);
    await user.click(screen.getByRole("button", { name: new RegExp(t("generate.period")) }));
    expect(await screen.findByRole("dialog", { name: t("generate.period") })).toBeInTheDocument();
  });

  it("the popover opens below, wide, with lengths and calendar periods in two lists", async () => {
    const user = userEvent.setup();
    open([]);
    const dialog = await trigger(user);
    expect(dialog).toHaveAttribute("data-side", "bottom");
    expect(dialog).toHaveClass("w-[min(46rem,calc(100vw-2rem))]");
    const lengths = within(dialog).getByRole("button", { name: t("period.quick.1h") }).parentElement as HTMLElement;
    const calendar = within(dialog).getByRole("button", { name: t("period.calendar.today") }).parentElement as HTMLElement;
    expect(lengths).not.toBe(calendar);
    expect(lengths.parentElement).toBe(calendar.parentElement);
    expect(lengths.parentElement).toHaveClass("grid-cols-2");
    expect(within(lengths).getAllByRole("button").map((button) => button.textContent)).toEqual(
      ["1h", "6h", "24h", "7d", "15d", "30d"].map((quick) => t(`period.quick.${quick}`)),
    );
    expect(within(calendar).queryByRole("button", { name: t("period.quick.1h") })).not.toBeInTheDocument();
    expect(within(lengths).queryByRole("button", { name: t("period.calendar.today") })).not.toBeInTheDocument();
    expect(dialog.querySelector("hr")).toBeNull();
  });

  it("a restricted profile has one list of its lengths", async () => {
    const user = userEvent.setup();
    open(["24h", "7d"]);
    const dialog = await trigger(user);
    expect(dialog).toHaveClass("w-[min(36rem,calc(100vw-2rem))]");
    const first = within(dialog).getByRole("button", { name: t("period.quick.24h") }).parentElement as HTMLElement;
    expect(within(first).getAllByRole("button")).toHaveLength(2);
    expect(first.parentElement).not.toHaveClass("grid-cols-2");
  });

  it("the popover is bounded to the window and scrolls inside", async () => {
    const user = userEvent.setup();
    open([], range("2026-09-01 00:00", "2026-10-01 00:00"));
    const dialog = await trigger(user);
    expect(dialog).toHaveClass("max-h-(--radix-popover-content-available-height)");
    expect(dialog).toHaveClass("overflow-y-auto");
  });

  it("the focus ring of a quick range sits outside the button, on the popover", async () => {
    const user = userEvent.setup();
    open([], length("7d"));
    const dialog = await trigger(user);
    const pressed = within(dialog).getByRole("button", { name: t("period.quick.7d") });
    expect(pressed).toHaveAttribute("aria-pressed", "true");
    expect(pressed).toHaveClass("focus-visible:outline-offset-2");
  });

  it("a quick range is applied at once", async () => {
    const user = userEvent.setup();
    const onChange = open([]);
    const dialog = await trigger(user);
    await user.click(within(dialog).getByRole("button", { name: t("period.quick.7d") }));
    expect(onChange).toHaveBeenCalledExactlyOnceWith(length("7d"));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("length grammar", async () => {
    const user = userEvent.setup();
    const onChange = open([]);
    const dialog = await trigger(user);
    const input = within(dialog).getByRole("textbox", { name: t("period.custom.label") });
    for (const bad of ["0h", "01h", "1H", "1.5h", "36", "h", ""]) {
      await user.clear(input);
      if (bad !== "") {
        await user.type(input, bad);
      }
      expect({ bad, invalid: within(dialog).queryByText(t("period.invalid")) !== null }).toEqual({ bad, invalid: true });
      expect({ bad, disabled: apply(dialog).hasAttribute("disabled") }).toEqual({ bad, disabled: true });
    }
    expect(onChange).not.toHaveBeenCalled();
    await retype(user, input, " 36h ");
    await user.click(apply(dialog));
    expect(onChange).toHaveBeenCalledExactlyOnceWith(length("36h"));
  });

  describe("opens on the value", () => {
    it("a range opens on Range with its walls", async () => {
      const user = userEvent.setup();
      open([], range("2026-09-01 00:00", "2026-10-01 00:00"));
      const dialog = await trigger(user);
      expect(within(dialog).getByRole("radio", { name: t("period.mode.range") })).toBeChecked();
      expect(from(dialog)).toHaveValue("2026-09-01 00:00");
      expect(to(dialog)).toHaveValue("2026-10-01 00:00");
      expect(from(dialog)).toHaveClass("font-mono");
      expect(to(dialog)).toHaveClass("font-mono");
      expect(within(dialog).getByText(t("period.range.hint", { zone }))).toBeInTheDocument();
    });

    it("a calendar name opens on Range with the name resolved now", async () => {
      const user = userEvent.setup();
      open([], { kind: "calendar", name: "previousWeek" });
      const dialog = await trigger(user);
      expect(within(dialog).getByRole("radio", { name: t("period.mode.range") })).toBeChecked();
      expect(from(dialog)).toHaveValue("2026-09-28 00:00");
      expect(to(dialog)).toHaveValue("2026-10-05 00:00");
      expect(within(dialog).getByRole("button", { name: t("period.calendar.previousWeek") })).toHaveAttribute(
        "aria-pressed",
        "true",
      );
    });

    it("a length opens on Length", async () => {
      const user = userEvent.setup();
      open([], length("7d"));
      const dialog = await trigger(user);
      expect(within(dialog).getByRole("radio", { name: t("period.mode.length") })).toBeChecked();
      expect(within(dialog).getByRole("textbox", { name: t("period.custom.label") })).toHaveValue("7d");
    });

    it("a length is pressed only on Length", async () => {
      const user = userEvent.setup();
      open([], length("24h"));
      const dialog = await trigger(user);
      const quick = within(dialog).getByRole("button", { name: t("period.quick.24h") });
      expect(quick).toHaveAttribute("aria-pressed", "true");
      await user.click(within(dialog).getByRole("radio", { name: t("period.mode.range") }));
      expect(within(dialog).queryAllByRole("button", { pressed: true })).toHaveLength(0);
      await user.click(within(dialog).getByRole("radio", { name: t("period.mode.length") }));
      expect(quick).toHaveAttribute("aria-pressed", "true");
    });

    it("a calendar name is unpressed once the draft is edited", async () => {
      const user = userEvent.setup();
      open([], { kind: "calendar", name: "yesterday" });
      const dialog = await trigger(user);
      const quick = within(dialog).getByRole("button", { name: t("period.calendar.yesterday") });
      expect(quick).toHaveAttribute("aria-pressed", "true");
      await user.type(from(dialog), "1");
      expect(quick).toHaveAttribute("aria-pressed", "false");
    });

    it("the switch opens Range on yesterday", async () => {
      const user = userEvent.setup();
      open([], length("7d"));
      const dialog = await trigger(user);
      await user.click(within(dialog).getByRole("radio", { name: t("period.mode.range") }));
      expect(from(dialog)).toHaveValue("2026-10-06 00:00");
      expect(to(dialog)).toHaveValue("2026-10-07 00:00");
    });
  });

  it("typed range applies", async () => {
    const user = userEvent.setup();
    const onChange = open([], range("2026-09-01 00:00", "2026-10-01 00:00"));
    const dialog = await trigger(user);
    await retype(user, from(dialog), " 2026-09-02 00:00 ");
    await retype(user, to(dialog), "2026-10-02 00:00");
    await user.click(apply(dialog));
    expect(onChange).toHaveBeenCalledExactlyOnceWith(range("2026-09-02 00:00", "2026-10-02 00:00"));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("the trigger reads the applied range", () => {
    open([], range("2026-09-01 00:00", "2026-10-01 00:00"));
    expect(screen.getByRole("button", { name: new RegExp(t("generate.period")) })).toHaveTextContent(
      "2026-09-01 00:00 to 2026-10-01 00:00",
    );
  });

  describe("range problems", () => {
    const cases = [
      { name: "invalid start", start: "2026-02-30 00:00", end: "2026-10-01 00:00", field: "start", word: "invalid" },
      { name: "invalid end", start: "2026-09-01 00:00", end: "nope", field: "end", word: "invalid" },
      { name: "order", start: "2026-10-01 00:00", end: "2026-09-01 00:00", field: "start", word: "order" },
      { name: "future", start: "2026-09-01 00:00", end: "2026-10-08 00:00", field: "end", word: "future" },
    ] as const;
    for (const c of cases) {
      it(c.name, async () => {
        const user = userEvent.setup();
        const onChange = open([], range("2026-09-01 00:00", "2026-10-01 00:00"));
        const dialog = await trigger(user);
        await retype(user, from(dialog), c.start);
        await retype(user, to(dialog), c.end);
        await user.click(apply(dialog));
        const field = c.field === "start" ? from(dialog) : to(dialog);
        const sentence = t(`period.range.${c.word}`);
        expect(within(dialog).getByText(sentence)).toBeInTheDocument();
        expect(field).toHaveAccessibleDescription(new RegExp(sentence));
        expect(field).toHaveAttribute("aria-invalid", "true");
        expect(onChange).not.toHaveBeenCalled();
        expect(screen.getByRole("dialog")).toBeInTheDocument();
        expect(field).toHaveFocus();
      });
    }

    it("focuses the first field with a problem when both have one", async () => {
      const user = userEvent.setup();
      open([], range("2026-09-01 00:00", "2026-10-01 00:00"));
      const dialog = await trigger(user);
      await retype(user, from(dialog), "nope");
      await retype(user, to(dialog), "nope");
      await user.click(apply(dialog));
      expect(from(dialog)).toHaveFocus();
      expect(from(dialog)).toHaveAttribute("aria-invalid", "true");
      expect(to(dialog)).toHaveAttribute("aria-invalid", "true");
    });
  });

  it("keeps the focus ring of the right column inside the scroll container", async () => {
    const user = userEvent.setup();
    open([]);
    const dialog = await trigger(user);
    const column = within(dialog).getByRole("button", { name: "Last hour" }).closest(".px-1");
    expect(column).not.toBeNull();
  });

  describe("calendar", () => {
    const pick = (user: ReturnType<typeof userEvent.setup>, field: "from" | "to") =>
      user.click(screen.getByRole("button", { name: t(field === "from" ? "period.range.pickFrom" : "period.range.pickTo") }));

    it("picks a day, keeps the time, closes, and returns focus", async () => {
      const user = userEvent.setup();
      const onChange = open([], range("2026-09-01 08:30", "2026-10-01 00:00"));
      const dialog = await trigger(user);
      await pick(user, "from");
      const grid = await within(dialog).findByRole("grid");
      await user.click(within(grid).getByRole("button", { name: /15th/ }));
      expect(from(dialog)).toHaveValue("2026-09-15 08:30");
      expect(within(dialog).queryByRole("grid")).not.toBeInTheDocument();
      expect(screen.getByRole("button", { name: t("period.range.pickFrom") })).toHaveFocus();
      expect(onChange).not.toHaveBeenCalled();
    });

    it("the button says whether its calendar is open", async () => {
      const user = userEvent.setup();
      open([], range("2026-09-01 00:00", "2026-10-01 00:00"));
      await trigger(user);
      const button = screen.getByRole("button", { name: t("period.range.pickFrom") });
      expect(button).toHaveAttribute("aria-expanded", "false");
      await pick(user, "from");
      expect(button).toHaveAttribute("aria-expanded", "true");
    });

    it("Escape closes the calendar, not the picker, and returns focus", async () => {
      const user = userEvent.setup();
      open([], range("2026-09-01 00:00", "2026-10-01 00:00"));
      const dialog = await trigger(user);
      await pick(user, "to");
      await within(dialog).findByRole("grid");
      await user.keyboard("{Escape}");
      expect(screen.queryByRole("grid")).not.toBeInTheDocument();
      expect(screen.getByRole("dialog")).toBeInTheDocument();
      expect(screen.getByRole("button", { name: t("period.range.pickTo") })).toHaveFocus();
    });

    it("one calendar at a time", async () => {
      const user = userEvent.setup();
      open([], range("2026-09-01 00:00", "2026-10-01 00:00"));
      await trigger(user);
      await pick(user, "from");
      await pick(user, "to");
      expect(screen.getAllByRole("grid")).toHaveLength(1);
      expect(screen.getByRole("button", { name: t("period.range.pickFrom") })).toHaveAttribute("aria-expanded", "false");
      expect(screen.getByRole("button", { name: t("period.range.pickTo") })).toHaveAttribute("aria-expanded", "true");
    });

    describe("the days around the month", () => {
      async function september(user: ReturnType<typeof userEvent.setup>, start = "2026-09-01 00:00") {
        open([], range(start, "2026-10-01 00:00"));
        const dialog = await trigger(user);
        await pick(user, "from");
        await within(dialog).findByRole("grid");
        return dialog;
      }
      const cell = (dialog: HTMLElement, day: string) => dialog.querySelector<HTMLElement>(`td[data-day="${day}"]`);

      it("shows an outside day muted and never in range", async () => {
        const dialog = await september(userEvent.setup());
        const aug = cell(dialog, "2026-08-31");
        expect(aug).not.toBeNull();
        expect(aug?.querySelector("button")).not.toBeNull();
        expect(aug).toHaveAttribute("data-outside", "true");
        expect(aug?.className).toContain("text-muted-fg");
        expect(aug?.className).not.toContain("bg-info-surface");
      });

      it("an outside day inside the range is not shaded", async () => {
        const user = userEvent.setup();
        open([], range("2026-09-01 00:00", "2026-10-01 00:00"));
        const dialog = await trigger(user);
        await pick(user, "to");
        await within(dialog).findByRole("grid");
        const sep = cell(dialog, "2026-09-30");
        expect(sep).toHaveAttribute("data-outside", "true");
        expect(sep?.className).toContain("data-[outside=true]:bg-transparent");
        expect(sep?.className).toContain("data-[outside=true]:text-muted-fg");
      });

      it("an outside day that ends the range is selected", async () => {
        const dialog = await september(userEvent.setup());
        const oct = cell(dialog, "2026-10-01");
        expect(oct).not.toBeNull();
        expect(oct?.className).toContain("bg-action");
        expect(oct?.className).toContain("data-[outside=true]:text-action-fg");
      });

      it("picking an outside day sets the date and keeps the time", async () => {
        const user = userEvent.setup();
        const dialog = await september(user);
        await user.click(within(cell(dialog, "2026-08-31") as HTMLElement).getByRole("button"));
        expect(from(dialog)).toHaveValue("2026-08-31 00:00");
        expect(within(dialog).queryByRole("grid")).not.toBeInTheDocument();
        expect(screen.getByRole("button", { name: t("period.range.pickFrom") })).toHaveFocus();
      });
    });

    it("a day's focus ring sits outside its cell", async () => {
      const user = userEvent.setup();
      open([], range("2026-09-01 00:00", "2026-10-01 00:00"));
      const dialog = await trigger(user);
      await pick(user, "from");
      await within(dialog).findByRole("grid");
      const day = dialog.querySelector<HTMLElement>('td[data-day="2026-09-10"] button');
      expect(day).toHaveClass("focus-visible:outline-offset-2");
      expect(day?.className).not.toMatch(/-outline-offset-/);
    });

    it("starts on Monday and speaks the language", async () => {
      await i18next.changeLanguage("pt-BR");
      const user = userEvent.setup();
      open([], range("2026-09-01 00:00", "2026-10-01 00:00"));
      const dialog = await trigger(user);
      await pick(user, "from");
      await within(dialog).findByRole("grid");
      expect(dialog.querySelector("th")?.getAttribute("aria-label")).toMatch(/segunda/i);
      expect(within(dialog).getByText(/setembro/)).toBeInTheDocument();
      expect(within(dialog).getByRole("button", { name: t("period.range.previous") })).toBeInTheDocument();
      expect(within(dialog).getByRole("button", { name: t("period.range.next") })).toBeInTheDocument();
    });

    it("starts on Monday in English", async () => {
      const user = userEvent.setup();
      open([], range("2026-09-01 00:00", "2026-10-01 00:00"));
      const dialog = await trigger(user);
      await pick(user, "from");
      await within(dialog).findByRole("grid");
      expect(dialog.querySelector("th")?.getAttribute("aria-label")).toMatch(/monday/i);
      expect(within(dialog).getByRole("button", { name: t("period.range.previous") })).toBeInTheDocument();
    });
  });

  it("a calendar name applies", async () => {
    const user = userEvent.setup();
    const onChange = open([]);
    const dialog = await trigger(user);
    await user.click(within(dialog).getByRole("button", { name: t("period.calendar.previousMonth") }));
    expect(onChange).toHaveBeenCalledExactlyOnceWith({ kind: "calendar", name: "previousMonth" });
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("the trigger reads a calendar name", () => {
    open([], { kind: "calendar", name: "yesterday" });
    expect(screen.getByRole("button", { name: new RegExp(t("generate.period")) })).toHaveTextContent("Yesterday");
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
    expect(within(dialog).queryByRole("radio")).not.toBeInTheDocument();
    expect(within(dialog).queryByRole("textbox", { name: t("period.custom.label") })).not.toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: t("period.quick.7d") }));
    expect(onChange).toHaveBeenCalledExactlyOnceWith(length("7d"));
  });

  it("a permitted period outside the quick ranges is worded as a custom one", async () => {
    const user = userEvent.setup();
    open(["90m"]);
    const dialog = await trigger(user);
    expect(within(dialog).getByRole("button", { name: "Last 90 minutes" })).toBeInTheDocument();
  });

  it("a server error is shown beside the period", () => {
    render(withTooltips(<PeriodPicker value={length("24h")} zone={zone} permitted={[]} error="Not allowed." onChange={vi.fn()} />));
    expect(screen.getByText("Not allowed.")).toBeInTheDocument();
  });
});
