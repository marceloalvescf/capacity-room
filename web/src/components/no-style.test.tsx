import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { expect, it } from "vitest";
import { ThemeToggle } from "@/components/theme-toggle";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Menu, MenuContent, MenuRadioGroup, MenuRadioItem, MenuTrigger } from "@/components/ui/menu";
import { i18next } from "@/i18n/i18n";

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
