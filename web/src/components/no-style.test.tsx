import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { expect, it } from "vitest";
import { ThemeToggle } from "@/components/theme-toggle";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Menu, MenuContent, MenuRadioGroup, MenuRadioItem, MenuTrigger } from "@/components/ui/menu";
import { PeriodPicker } from "@/features/reports/period-picker";
import { reportTabs } from "@/features/report/tabs";
import { i18next } from "@/i18n/i18n";
import { installMockApi, profileSummary, report, signedIn } from "../../tests/msw";
import { renderApp } from "../../tests/render";
import { mockColourScheme } from "../../tests/match-media";
import viewEn from "../../tests/fixtures/view/edge-scaling.en.json";
import type { components } from "@/api/schema.gen";

const mock = installMockApi(signedIn());
const reportId = "0b6c3b52-2f0e-4d0a-8d57-6f6f6c1f0a01";

// Watches the document for any style element added to it.
function watchStyles() {
  const seen: string[] = [];
  function collect(records: MutationRecord[]): void {
    for (const record of records) {
      record.addedNodes.forEach((node) => {
        if (node instanceof HTMLStyleElement) {
          seen.push(node.outerHTML);
        }
      });
    }
  }
  const observer = new MutationObserver(collect);
  observer.observe(document, { childList: true, subtree: true });
  return () => {
    collect(observer.takeRecords());
    observer.disconnect();
    return seen;
  };
}

it("opening and closing a modal menu injects no style element", async () => {
  await i18next.changeLanguage("en");
  const stop = watchStyles();

  const user = userEvent.setup();
  render(
    <Menu>
      <MenuTrigger>{i18next.t("nav.account")}</MenuTrigger>
      <MenuContent>
        <MenuRadioGroup value="a">
          <MenuRadioItem value="a">{i18next.t("theme.light")}</MenuRadioItem>
        </MenuRadioGroup>
      </MenuContent>
    </Menu>,
  );
  expect(document.querySelectorAll("style")).toHaveLength(0);

  await user.click(screen.getByRole("button", { name: "Account" }));
  expect(await screen.findByRole("menu")).toBeInTheDocument();
  expect(document.body).toHaveAttribute("data-scroll-locked");
  expect(document.querySelectorAll("style")).toHaveLength(0);

  await user.keyboard("{Escape}");
  expect(screen.queryByRole("menu")).toBeNull();
  expect(document.body).not.toHaveAttribute("data-scroll-locked");
  expect(document.querySelectorAll("style")).toHaveLength(0);

  expect(stop()).toEqual([]);
});

it("an icon button shows its name on focus and on hover, opens its menu, and injects no style element", async () => {
  await i18next.changeLanguage("en");
  const stop = watchStyles();
  const user = userEvent.setup();
  render(<ThemeToggle />);
  const button = screen.getByRole("button", { name: "Theme: System" });
  expect(button).toHaveTextContent("");

  await user.tab();
  expect(button).toHaveFocus();
  expect(within(await screen.findByRole("tooltip")).getByText("Theme: System")).toBeInTheDocument();

  await user.keyboard("{Enter}");
  expect(await screen.findByRole("menu")).toBeInTheDocument();
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("menu")).toBeNull();

  await user.tab();
  await user.hover(button);
  expect(within(await screen.findByRole("tooltip", {}, { timeout: 2000 })).getByText("Theme: System")).toBeInTheDocument();
  await user.unhover(button);

  expect(document.querySelectorAll("style")).toHaveLength(0);
  expect(stop()).toEqual([]);
});

it("no style from the popover and the dialog", async () => {
  await i18next.changeLanguage("en");
  const stop = watchStyles();
  const user = userEvent.setup();
  function Both() {
    const [open, setOpen] = useState(false);
    return (
      <>
        <Popover>
          <PopoverTrigger>{"Pick"}</PopoverTrigger>
          <PopoverContent>{"Inside"}</PopoverContent>
        </Popover>
        <button type="button" onClick={() => { setOpen(true); }}>
          {"Ask"}
        </button>
        <ConfirmDialog
          open={open}
          title="Title"
          body="Body"
          confirm="Yes"
          cancel="No"
          onConfirm={() => { setOpen(false); }}
          onOpenChange={setOpen}
        />
      </>
    );
  }
  render(<Both />);
  await user.click(screen.getByRole("button", { name: "Pick" }));
  expect(await screen.findByText("Inside")).toBeInTheDocument();
  expect(document.querySelectorAll("style")).toHaveLength(0);
  await user.keyboard("{Escape}");
  await user.click(screen.getByRole("button", { name: "Ask" }));
  expect(await screen.findByRole("alertdialog")).toBeInTheDocument();
  expect(document.querySelectorAll("style")).toHaveLength(0);
  await user.keyboard("{Escape}");
  expect(screen.queryByRole("alertdialog")).toBeNull();
  expect(document.querySelectorAll("style")).toHaveLength(0);
  expect(stop()).toEqual([]);
});

it("the period picker on Range with a calendar open writes no style", async () => {
  await i18next.changeLanguage("en");
  const stop = watchStyles();
  const user = userEvent.setup();
  render(
    <PeriodPicker
      value={{ kind: "range", start: "2026-09-01 00:00", end: "2026-10-01 00:00" }}
      zone="UTC"
      permitted={[]}
      onChange={() => undefined}
    />,
  );
  await user.click(screen.getByRole("button", { name: /Period/ }));
  await user.click(await screen.findByRole("button", { name: i18next.t("period.range.pickFrom") }));
  expect(await screen.findByRole("grid")).toBeInTheDocument();
  // Radix sets custom properties on its own layers through the DOM; the calendar writes none.
  const calendar = (await screen.findByRole("grid")).closest(".rdp-root");
  expect(calendar).not.toBeNull();
  expect([calendar, ...(calendar?.querySelectorAll("[style]") ?? [])].filter((node) => node?.hasAttribute("style"))).toEqual([]);
  expect(screen.getByRole("dialog").querySelector("form")?.querySelectorAll("[style]")).toHaveLength(0);
  expect(document.querySelectorAll("style")).toHaveLength(0);
  expect(stop()).toEqual([]);
});

// The properties an element's style attribute declares.
function declared(node: Element): string[] {
  const style = (node as HTMLElement).style;
  return Array.from({ length: style.length }, (_, index) => style.item(index));
}

// Elements with a style attribute, leaving out what Radix sets through the DOM itself: the popper wrapper element, the
// focus guard elements, a `--radix-` custom property inside a popper wrapper, and the body while its style holds only what the
// scroll lock sets. A style on anything else inside a layer is reported.
function styled(): Element[] {
  return Array.from(document.querySelectorAll("[style]")).filter((node) => {
    if (node.matches("[data-radix-popper-content-wrapper], [data-radix-focus-guard]")) {
      return false;
    }
    const names = declared(node);
    if (node === document.body) {
      return names.some((name) => name !== "pointer-events");
    }
    return !(node.closest("[data-radix-popper-content-wrapper]") !== null && names.length > 0 && names.every((name) => name.startsWith("--radix-")));
  });
}

it("the report writes no style", async () => {
  window.localStorage.clear();
  mockColourScheme(false);
  mock.profiles = [profileSummary()];
  mock.reports = [report()];
  mock.views.set(`${reportId}:en`, viewEn as components["schemas"]["ReportView"]);
  await i18next.changeLanguage("en");
  const stop = watchStyles();
  const user = userEvent.setup();

  for (const tab of reportTabs) {
    const { unmount } = await renderApp(`/reports/${reportId}/${tab}`, "en");
    await screen.findByRole("navigation", { name: i18next.t("report.tabs") });
    await screen.findByRole("heading", { level: 1, name: i18next.t("progress.title", { profile: "Production" }) });
    if (tab === "findings") {
      await screen.findByRole("searchbox");
      const items = screen
        .getAllByRole("list")
        .flatMap((list) => within(list).queryAllByRole("button"))
        .filter((button) => button.hasAttribute("aria-pressed"));
      expect(items.length).toBeGreaterThan(1);
      await user.click(items[1] as HTMLElement);
      await screen.findByRole("article");
    } else if (tab === "overview") {
      await user.click(screen.getByRole("button", { name: i18next.t("progress.pdf") }));
      expect(await screen.findByRole("dialog")).toBeInTheDocument();
    } else {
      await waitFor(() => { expect(document.querySelector("table button[aria-expanded='false']"), tab).not.toBeNull(); });
      const opener = document.querySelector("table button[aria-expanded='false']");
      await user.click(opener as Element);
      expect(document.querySelector("table button[aria-expanded='true']"), tab).not.toBeNull();
    }
    expect(styled(), tab).toEqual([]);
    expect(document.querySelectorAll("style"), tab).toHaveLength(0);
    unmount();
  }
  expect(stop()).toEqual([]);
});
