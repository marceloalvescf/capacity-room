import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it } from "vitest";
import { Menu, MenuContent, MenuRadioGroup, MenuRadioItem, MenuTrigger } from "@/components/ui/menu";
import { i18next } from "@/i18n/i18n";

it("opening and closing a modal menu injects no style element", async () => {
  await i18next.changeLanguage("en");
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

  collect(observer.takeRecords());
  observer.disconnect();
  expect(seen).toEqual([]);
});
