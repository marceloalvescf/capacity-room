import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { mockColourScheme } from "../../../tests/match-media";
import { ThemeToggle } from "@/components/theme-toggle";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Menu, MenuContent, MenuRadioGroup, MenuRadioItem, MenuTrigger } from "@/components/ui/menu";
import { i18next } from "@/i18n/i18n";

const root = document.documentElement;

beforeEach(async () => {
  await i18next.changeLanguage("en");
  root.removeAttribute("data-theme-choice");
  root.removeAttribute("data-theme");
  window.localStorage.clear();
  mockColourScheme(false);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

async function violations(container: HTMLElement): Promise<string[]> {
  const result = await axe.run(container, { rules: { "color-contrast": { enabled: false }, region: { enabled: false } } });
  return result.violations.map((v) => `${v.id}: ${v.help}`);
}

describe("ui", () => {
  for (const theme of ["light", "dark"]) {
    it(`axe: the primitives in the ${theme} theme`, async () => {
      root.setAttribute("data-theme", theme);
      const { container } = render(
        <main>
          <Button>{i18next.t("login.submit")}</Button>
          <Button variant="primary" disabled>
            {i18next.t("login.submitting")}
          </Button>
          <Field label={i18next.t("login.username")} />
          <Field
            label={i18next.t("account.password.new")}
            hint={i18next.t("account.password.hint")}
            error={i18next.t("validation.required")}
          />
          <Alert tone="error">{i18next.t("error.general")}</Alert>
          <Alert tone="notice">{i18next.t("login.sessionEnded")}</Alert>
          <ThemeToggle />
        </main>,
      );
      expect(await violations(container)).toEqual([]);

      const user = userEvent.setup();
      await user.click(screen.getByRole("button", { name: /Theme/ }));
      await screen.findByRole("menu");
      expect(await violations(document.body)).toEqual([]);
    });
  }

  it("a menu holds radio items", async () => {
    const user = userEvent.setup();
    render(
      <Menu>
        <MenuTrigger>{i18next.t("nav.account")}</MenuTrigger>
        <MenuContent>
          <MenuRadioGroup value="en">
            <MenuRadioItem value="en">{i18next.t("language.en")}</MenuRadioItem>
          </MenuRadioGroup>
        </MenuContent>
      </Menu>,
    );
    await user.click(screen.getByRole("button"));
    expect(await screen.findByRole("menuitemradio", { name: "English" })).toBeChecked();
  });

  it("the theme toggle chooses dark, then system", async () => {
    const media = mockColourScheme(false);
    const user = userEvent.setup();
    render(<ThemeToggle />);

    await user.click(screen.getByRole("button", { name: /Theme/ }));
    await user.click(await screen.findByRole("menuitemradio", { name: "Dark" }));
    expect(root.getAttribute("data-theme")).toBe("dark");
    expect(window.localStorage.getItem("kcr.theme")).toBe("dark");

    media.setDark(false);
    await user.click(screen.getByRole("button", { name: /Theme/ }));
    await user.click(await screen.findByRole("menuitemradio", { name: "System" }));
    expect(root.getAttribute("data-theme")).toBe("light");
    expect(window.localStorage.getItem("kcr.theme")).toBe("system");
  });
});
