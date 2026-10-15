/// <reference types="node" />
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { buttonStyles } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Dialog } from "@/components/ui/dialog";
import { IconButton } from "@/components/ui/icon-button";
import { Menu, MenuContent, MenuRadioGroup, MenuRadioItem, MenuTrigger } from "@/components/ui/menu";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { i18next } from "@/i18n/i18n";
import { withTooltips } from "../../tests/render";

const css = readFileSync(join(import.meta.dirname, "theme.css"), "utf8");

// The at-rules and selectors that enclose each animation declaration of the stylesheet, outermost first.
function animationContexts(): string[][] {
  const found: string[][] = [];
  const stack: string[] = [];
  let start = 0;
  for (let at = 0; at < css.length; at++) {
    const char = css[at];
    if (char === "{") {
      stack.push(css.slice(start, at).replace(/\/\*[\s\S]*?\*\//g, "").trim());
      start = at + 1;
    } else if (char === "}") {
      stack.pop();
      start = at + 1;
    } else if (char === ";") {
      if (/^animation\s*:/.test(css.slice(start, at).replace(/\/\*[\s\S]*?\*\//g, "").trim())) {
        found.push([...stack]);
      }
      start = at + 1;
    }
  }
  return found;
}

describe("motion", () => {
  it("every animation of the stylesheet runs only when the reader allows motion", () => {
    const contexts = animationContexts();
    expect(contexts).toHaveLength(6);
    for (const context of contexts) {
      expect(context).toContain("@media (prefers-reduced-motion: no-preference)");
    }
  });

  it("a button changes colour under the pointer only while enabled, and sinks a pixel when pressed", () => {
    const hover = { primary: "not-disabled:hover:bg-action-hover", secondary: "not-disabled:hover:bg-hover", danger: "not-disabled:hover:bg-critical-hover" };
    for (const [variant, name] of Object.entries(hover)) {
      const classes = buttonStyles(variant as keyof typeof hover).split(" ");
      expect(classes).toContain(name);
      expect(classes).toContain("not-disabled:active:translate-y-px");
      expect(classes).toContain("motion-safe:transition-transform");
      expect(classes.filter((c) => c.startsWith("hover:") || c.startsWith("active:"))).toEqual([]);
    }
  });

  it("the menu, the popover, and the tooltip open with the layer motion", async () => {
    await i18next.changeLanguage("en");
    const user = userEvent.setup();
    render(
      withTooltips(
        <>
          <Menu>
            <MenuTrigger>{i18next.t("nav.account")}</MenuTrigger>
            <MenuContent>
              <MenuRadioGroup value="light">
                <MenuRadioItem value="light">{i18next.t("theme.light")}</MenuRadioItem>
              </MenuRadioGroup>
            </MenuContent>
          </Menu>
          <Popover>
            <PopoverTrigger>{i18next.t("nav.reports")}</PopoverTrigger>
            <PopoverContent>{i18next.t("nav.users")}</PopoverContent>
          </Popover>
          <IconButton label={i18next.t("header.signOut")} />
        </>,
      ),
    );
    await user.click(screen.getByRole("button", { name: "Account" }));
    expect(await screen.findByRole("menu")).toHaveClass("layer-motion");
    await user.keyboard("{Escape}");
    await user.click(screen.getByRole("button", { name: "Reports" }));
    expect((await screen.findByText("Users")).closest("[data-radix-popper-content-wrapper] > *")).toHaveClass("layer-motion");
    await user.keyboard("{Escape}");
    screen.getByRole("button", { name: "Sign out" }).focus();
    const tip = await screen.findByRole("tooltip");
    expect(tip.closest("[data-radix-popper-content-wrapper] > *")).toHaveClass("layer-motion");
  });

  it("the dialog and the confirmation open over a fading scrim", () => {
    const dialog = render(
      <Dialog open title={i18next.t("nav.users")} onOpenChange={() => undefined}>
        {null}
      </Dialog>,
    );
    expect(screen.getByRole("dialog")).toHaveClass("dialog-motion");
    expect(document.querySelectorAll(".scrim-motion")).toHaveLength(1);
    dialog.unmount();
    render(
      <ConfirmDialog
        open
        title={i18next.t("nav.profiles")}
        body={i18next.t("nav.reports")}
        confirm={i18next.t("nav.account")}
        cancel={i18next.t("theme.light")}
        onConfirm={() => undefined}
        onOpenChange={() => undefined}
      />,
    );
    expect(screen.getByRole("alertdialog")).toHaveClass("dialog-motion");
    expect(document.querySelectorAll(".scrim-motion")).toHaveLength(1);
  });
});
