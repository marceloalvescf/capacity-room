import type { Page } from "@playwright/test";
import en from "../src/locales/en.json" with { type: "json" };
import ptBR from "../src/locales/pt-BR.json" with { type: "json" };
import { expect, expectNoViolations, test } from "./fixtures";

const username = "e2e-admin";
const password = "e2e-Fixture-Pass-01!";
const newPassword = "e2e-Changed-Pass-02!";

// The words of a name that come before its first parameter.
const before = (text: string) => text.split("{{")[0] ?? text;

async function signIn(page: Page, secret: string) {
  await page.getByLabel(en.login.username, { exact: true }).fill(username);
  await page.getByLabel(en.login.password, { exact: true }).fill(secret);
  await page.getByRole("button", { name: en.login.submit, exact: true }).click();
}

test("sign in, switch language, change password, and sign out", async ({ page }) => {
  await page.goto("/account");
  await expect(page).toHaveURL(/\/login/);
  await expect(page.getByRole("heading", { name: en.login.title })).toBeVisible();
  await expectNoViolations(page);

  await signIn(page, password);
  await expect(page.getByRole("heading", { name: en.account.title, level: 1 })).toBeVisible();
  await expectNoViolations(page);

  await page.getByRole("button", { name: new RegExp("^" + before(en.header.language)) }).click();
  await page.getByRole("menuitemradio", { name: ptBR.language["pt-BR"] }).click();
  await expect(page.getByRole("heading", { name: ptBR.account.title, level: 1 })).toBeVisible();
  await page.getByRole("button", { name: new RegExp("^" + before(ptBR.header.language)) }).click();
  await page.getByRole("menuitemradio", { name: en.language.en }).click();
  await expect(page.getByRole("heading", { name: en.account.title, level: 1 })).toBeVisible();

  const themeTrigger = page.getByRole("button", { name: new RegExp("^" + before(en.header.theme)) });
  await themeTrigger.focus();
  await page.keyboard.press("Enter");
  const highlighted = page.locator("[data-highlighted]");
  await expect(highlighted).toBeVisible();
  const outline = await highlighted.evaluate((element) => {
    const style = getComputedStyle(element);
    return { style: style.outlineStyle, width: style.outlineWidth };
  });
  expect(outline).toEqual({ style: "solid", width: "2px" });
  await page.keyboard.press("Escape");

  await page.goto("/no-such-screen");
  await expect(page.getByRole("heading", { name: en.notFound.title })).toBeVisible();
  await expectNoViolations(page);

  await page.goto("/account");
  await page.getByLabel(en.account.password.current, { exact: true }).fill(password);
  await page.getByLabel(en.account.password.new, { exact: true }).fill(newPassword);
  await page.getByLabel(en.account.password.repeat, { exact: true }).fill(newPassword);
  await page.getByRole("button", { name: en.account.password.submit }).click();
  await expect(page).toHaveURL(/\/login/);
  await expect(page.getByText(en.login.passwordChanged)).toBeVisible();

  await signIn(page, password);
  await expect(page.getByText(en.error.INVALID_CREDENTIALS)).toBeVisible();

  await page.getByLabel(en.login.password, { exact: true }).fill(newPassword);
  await page.getByRole("button", { name: en.login.submit, exact: true }).click();
  await expect(page.getByRole("heading", { name: en.reports.title, level: 1 })).toBeVisible();

  // The password goes back to the original, so that the other specs, run in
  // any order or alone, sign in with it.
  await page.goto("/account");
  await page.getByLabel(en.account.password.current, { exact: true }).fill(newPassword);
  await page.getByLabel(en.account.password.new, { exact: true }).fill(password);
  await page.getByLabel(en.account.password.repeat, { exact: true }).fill(password);
  await page.getByRole("button", { name: en.account.password.submit }).click();
  await expect(page).toHaveURL(/\/login/);
  await expect(page.getByText(en.login.passwordChanged)).toBeVisible();
  await signIn(page, password);
  await expect(page.getByRole("heading", { name: en.reports.title, level: 1 })).toBeVisible();
  await page.getByRole("button", { name: en.header.signOut }).click();
  await expect(page.getByRole("heading", { name: en.login.title })).toBeVisible();
});
