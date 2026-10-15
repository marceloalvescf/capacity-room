import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { mockColourScheme } from "../../../tests/match-media";
import { withTooltips } from "../../../tests/render";
import { ThemeToggle } from "@/components/theme-toggle";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Select } from "@/components/ui/select";
import { TextArea } from "@/components/ui/textarea";
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
        withTooltips(<main>
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
        </main>),
      );
      expect(await violations(container)).toEqual([]);

      const user = userEvent.setup();
      await user.click(screen.getByRole("button", { name: /Theme/ }));
      await screen.findByRole("menu");
      expect(await violations(document.body)).toEqual([]);
    });
  }

  it("a text box is labelled, described, and marked invalid", async () => {
    const { container } = render(
      <main>
        <TextArea label="Names" hint="One per line." error="Fill in this field." rows={3} />
      </main>,
    );
    const box = screen.getByLabelText("Names");
    expect(box.tagName).toBe("TEXTAREA");
    expect(box).toHaveAttribute("rows", "3");
    expect(box).toHaveAttribute("aria-invalid", "true");
    expect(box).toHaveAccessibleDescription("One per line. Fill in this field.");
    expect(box).toHaveClass("font-mono", "py-2");
    expect(await violations(container)).toEqual([]);
  });

  it("a select keeps a caller's description beside its own", () => {
    render(
      <main>
        <Select label="Zone" hint="Pick one." error="Not allowed." aria-describedby="x" aria-invalid={false}>
          <option>{"a"}</option>
        </Select>
      </main>,
    );
    const select = screen.getByLabelText("Zone");
    const ids = (select.getAttribute("aria-describedby") ?? "").split(" ");
    expect(ids).toHaveLength(3);
    expect(ids[0]).toBe("x");
    expect(select).toHaveAccessibleDescription("Pick one. Not allowed.");
    expect(select).toHaveAttribute("aria-invalid", "true");
  });

  it("a select is the browser's own control with the caret of the icon set drawn over it", () => {
    render(
      <Select label="Zone">
        <option>{"a"}</option>
      </Select>,
    );
    const select = screen.getByLabelText("Zone");
    expect(select.tagName).toBe("SELECT");
    expect(select).toHaveClass("appearance-none", "pr-9");
    const caret = select.parentElement?.querySelector("svg");
    expect(caret).toHaveAttribute("aria-hidden", "true");
    expect(caret).toHaveClass("pointer-events-none", "absolute");
  });

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

  it("a keyboard-highlighted item carries the focus outline", async () => {
    const user = userEvent.setup();
    render(withTooltips(<ThemeToggle />));
    screen.getByRole("button", { name: /Theme/ }).focus();
    await user.keyboard("{Enter}");
    await screen.findByRole("menu");
    await user.keyboard("{ArrowDown}");
    const highlighted = document.querySelector("[data-highlighted]");
    expect(highlighted).not.toBeNull();
    for (const name of ["focus-visible:outline-solid", "focus-visible:outline-2", "focus-visible:outline-accent", "focus-visible:-outline-offset-2!"]) {
      expect(highlighted?.classList.contains(name)).toBe(true);
    }
  });

  it("an item under the pointer keeps the tint and drops the focus ring", async () => {
    const user = userEvent.setup();
    render(withTooltips(<ThemeToggle />));
    await user.click(screen.getByRole("button", { name: /Theme/ }));
    const item = (await screen.findAllByRole("menuitemradio"))[0];
    const classes = [...(item?.classList ?? [])];
    expect(classes).toContain("data-[highlighted]:bg-sidebar-active");
    expect(classes).toContain("hover:outline-none!");
    expect(classes.filter((name) => name.startsWith("data-[highlighted]:outline"))).toEqual([]);
  });

  it("the theme toggle chooses dark, then system", async () => {
    const media = mockColourScheme(false);
    const user = userEvent.setup();
    render(withTooltips(<ThemeToggle />));

    await user.click(screen.getByRole("button", { name: /Theme/ }));
    const items = await screen.findAllByRole("menuitemradio");
    expect(items.filter((item) => item.querySelector("svg") !== null)).toHaveLength(1);
    expect(screen.getByRole("menuitemradio", { name: "System" }).querySelector("svg")).not.toBeNull();
    await user.click(screen.getByRole("menuitemradio", { name: "Dark" }));
    expect(root.getAttribute("data-theme")).toBe("dark");
    expect(window.localStorage.getItem("capacityroom.theme")).toBe("dark");

    media.setDark(false);
    await user.click(screen.getByRole("button", { name: /Theme/ }));
    await user.click(await screen.findByRole("menuitemradio", { name: "System" }));
    expect(root.getAttribute("data-theme")).toBe("light");
    expect(window.localStorage.getItem("capacityroom.theme")).toBe("system");
  });
});
