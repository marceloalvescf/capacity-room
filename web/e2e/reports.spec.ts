import { readFile } from "node:fs/promises";
import type { Page } from "@playwright/test";
import en from "../src/locales/en.json" with { type: "json" };
import { expect, expectNoViolations, test, watchPolicy } from "./fixtures";

const admin = { username: "e2e-admin", password: "e2e-Fixture-Pass-01!" };
const operator = { username: "e2e-operator", password: "e2e-Operator-Pass-03!" };
const reader = { username: "e2e-reader", password: "e2e-Reader-Pass-04!" };

const collectTimeout = 60_000;

async function signIn(page: Page, user: { username: string; password: string }, path: string) {
  await page.goto(path);
  await page.getByLabel(en.login.username, { exact: true }).fill(user.username);
  await page.getByLabel(en.login.password, { exact: true }).fill(user.password);
  await page.getByRole("button", { name: en.login.submit, exact: true }).click();
}

// Creates a user through the API, with the session and the CSRF token of the
// signed-in page.
async function createUser(page: Page, user: { username: string; password: string }, role: "operator" | "reader") {
  const session = await page.request.get("/api/v1/auth/session");
  expect(session.ok()).toBe(true);
  const { csrf_token: csrf } = (await session.json()) as { csrf_token: string };
  const created = await page.request.post("/api/v1/admin/users", {
    headers: { "X-CSRF-Token": csrf },
    data: { username: user.username, password: user.password, role, language: "en" },
  });
  expect(created.status()).toBe(201);
}

const pdfGroup = (page: Page, language: string) => page.getByRole("group", { name: language, exact: true });

test("an operator requests a report and a reader reads it", async ({ page, browser, baseURL }) => {
  test.setTimeout(240_000);

  await test.step("the administrator creates an operator and a reader", async () => {
    await signIn(page, admin, "/account");
    await expect(page.getByRole("heading", { name: en.account.title, level: 1 })).toBeVisible();
    await createUser(page, operator, "operator");
    await createUser(page, reader, "reader");
    await page.getByRole("button", { name: en.header.signOut }).click();
    await expect(page.getByRole("heading", { name: en.login.title })).toBeVisible();
  });

  await test.step("the operator requests a report", async () => {
    await signIn(page, operator, "/reports");
    await expect(page.getByRole("heading", { name: en.reports.title, level: 1 })).toBeVisible();
    await expect(page.getByText(en.reports.empty)).toBeVisible();
    await expectNoViolations(page);

    await page.getByRole("link", { name: en.nav.generate, exact: true }).first().click();
    await expect(page.getByRole("heading", { name: en.generate.title, level: 1 })).toBeVisible();
    await page.getByLabel(en.generate.profile, { exact: true }).selectOption({ label: "Production" });
    await page.getByRole("checkbox", { name: "payments", exact: true }).check();
    await page.getByRole("radio", { name: en.language.en, exact: true }).check();
    await expectNoViolations(page);
    // The fake Prometheus serves its scenario once, so one stack runs exactly
    // one collection: this is the only report the spec requests, and the
    // Playwright configuration keeps retries at zero.
    await page.getByRole("button", { name: en.generate.submit, exact: true }).click();
    await expect(page).toHaveURL(/\/jobs\/[0-9a-f-]{36}$/);
  });

  const jobUrl = page.url();

  await test.step("the progress page reaches Collected and the English PDF", async () => {
    await expect(page.getByText(en.reportStatus.collected, { exact: true })).toBeVisible({ timeout: collectTimeout });
    const english = pdfGroup(page, en.language.en);
    await expect(english.getByText(en.pdfStatus.ready, { exact: true })).toBeVisible({ timeout: collectTimeout });
    await expectNoViolations(page);

    const downloading = page.waitForEvent("download");
    await english.getByRole("link", { name: en.progress.download, exact: true }).click();
    const file = await (await downloading).path();
    const bytes = await readFile(file);
    expect(bytes.subarray(0, 5).toString("latin1")).toBe("%PDF-");
  });

  await test.step("the operator generates the Portuguese PDF", async () => {
    const portuguese = pdfGroup(page, en.language["pt-BR"]);
    await portuguese.getByRole("button", { name: en.progress.generatePdf, exact: true }).click();
    await expect(portuguese.getByText(en.pdfStatus.ready, { exact: true })).toBeVisible({ timeout: collectTimeout });
    await expectNoViolations(page);
  });

  await test.step("the list shows the report without a reload", async () => {
    await page.getByRole("navigation", { name: en.nav.label }).getByRole("link", { name: en.nav.reports, exact: true }).click();
    const row = page.getByRole("row").filter({ hasText: en.reportStatus.collected });
    await expect(row).toHaveCount(1);
    await expect(row.getByText(en.reportStatus.collected, { exact: true })).toBeVisible();
    await expect(row.getByText(en.pdfStatus.ready, { exact: true })).toBeVisible();
    await expectNoViolations(page);
  });

  await test.step("the reader sees no control that changes anything", async () => {
    const context = await browser.newContext(baseURL === undefined ? {} : { baseURL });
    try {
      const other = await context.newPage();
      const violations = await watchPolicy(other);

      await signIn(other, reader, "/reports");
      await expect(other.getByRole("heading", { name: en.reports.title, level: 1 })).toBeVisible();
      await expect(other.getByRole("row").filter({ hasText: en.reportStatus.collected })).toHaveCount(1);
      await expect(other.getByRole("link", { name: en.nav.generate, exact: true })).toHaveCount(0);
      await expect(other.getByRole("checkbox")).toHaveCount(0);
      await expect(other.getByRole("button", { name: en.reports.delete, exact: true })).toHaveCount(0);
      await expectNoViolations(other);

      await other.goto("/generate");
      await expect(other.getByRole("heading", { name: en.notFound.title, level: 1 })).toBeVisible();
      await expectNoViolations(other);

      await other.goto(new URL(jobUrl).pathname);
      await expect(other.getByText(en.reportStatus.collected, { exact: true })).toBeVisible();
      await expect(other.getByRole("link", { name: en.progress.download, exact: true })).toHaveCount(2);
      await expect(other.getByRole("button", { name: en.progress.generatePdf, exact: true })).toHaveCount(0);
      await expect(other.getByRole("button", { name: en.progress.retryPdf, exact: true })).toHaveCount(0);
      await expect(other.getByRole("button", { name: en.progress.deleteReport, exact: true })).toHaveCount(0);
      await expectNoViolations(other);

      expect(violations).toEqual([]);
    } finally {
      await context.close();
    }
  });

  await test.step("the operator deletes the report", async () => {
    await page.getByRole("button", { name: en.reports.delete, exact: true }).click();
    const dialog = page.getByRole("alertdialog", { name: en.delete.oneTitle });
    await expect(dialog).toBeVisible();
    await expect(dialog).toContainText("Production");
    await expect(dialog).toContainText(operator.username);
    await dialog.getByRole("button", { name: en.delete.confirm, exact: true }).click();
    await expect(page.getByText(en.reports.empty)).toBeVisible();
    await expectNoViolations(page);
  });
});
