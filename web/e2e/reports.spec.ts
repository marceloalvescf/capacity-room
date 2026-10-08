import { readFile } from "node:fs/promises";
import type { Page } from "@playwright/test";
import en from "../src/locales/en.json" with { type: "json" };
import { expect, expectNoViolations, test, watchPolicy } from "./fixtures";

const admin = { username: "e2e-admin", password: "e2e-Fixture-Pass-01!" };
const operator = { username: "e2e-operator", password: "e2e-Operator-Pass-03!" };
const reader = { username: "e2e-reader", password: "e2e-Reader-Pass-04!" };

const collectTimeout = 60_000;

// The profile e2e-prod names no time zone, so the report takes the fallback
// zone of the profiles.
const reportZone = "America/Sao_Paulo";

// The wall text, at 00:00 in the zone, of the Monday of the week that holds
// now, moved by whole weeks.
function mondayText(now: Date, zone: string, weeks: number): string {
  const parts = new Intl.DateTimeFormat("en-CA", { timeZone: zone, year: "numeric", month: "2-digit", day: "2-digit" }).formatToParts(now);
  const get = (type: string) => Number(parts.find((part) => part.type === type)?.value);
  const today = new Date(Date.UTC(get("year"), get("month") - 1, get("day")));
  const sinceMonday = (today.getUTCDay() + 6) % 7;
  const monday = new Date(today.getTime() + (weeks * 7 - sinceMonday) * 86_400_000);
  return `${monday.toISOString().slice(0, 10)} 00:00`;
}

async function signIn(page: Page, user: { username: string; password: string }, path: string) {
  await page.goto(path);
  await expect(page.getByRole("heading", { name: en.login.title })).toBeVisible({ timeout: 10_000 });
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
  let requestedAt = new Date();

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
    await page.getByRole("button", { name: en.generate.change, exact: true }).click();
    await page.getByRole("button", { name: new RegExp(`^${en.generate.period}`) }).click();
    const picker = page.getByRole("dialog");
    await picker.getByRole("radio", { name: en.period.mode.range, exact: true }).check();
    await picker.getByRole("button", { name: en.period.range.pickFrom, exact: true }).click();
    await expect(picker.getByRole("grid")).toBeVisible();
    await expectNoViolations(page);
    await page.keyboard.press("Escape");
    await expect(picker.getByRole("grid")).toHaveCount(0);
    await picker.getByRole("button", { name: en.period.calendar.previousWeek, exact: true }).click();
    await expectNoViolations(page);
    // The fake Prometheus serves its scenario once, so one stack runs exactly
    // one collection: this is the only report the spec requests, and the
    // Playwright configuration keeps retries at zero.
    requestedAt = new Date();
    await page.getByRole("button", { name: en.generate.submit, exact: true }).click();
    await expect(page).toHaveURL(/\/jobs\/[0-9a-f-]{36}$/);
    await expectNoViolations(page);
  });

  const jobUrl = page.url();

  await test.step("the stored selection is the previous calendar week", async () => {
    const id = /[0-9a-f-]{36}$/.exec(jobUrl)?.[0] ?? "";
    const response = await page.request.get(`/api/v1/reports/${id}`);
    expect(response.ok()).toBe(true);
    const { selection } = (await response.json()) as {
      selection: { period: string | null; start: string | null; end: string | null; timezone: string };
    };
    expect(selection).toMatchObject({
      period: null,
      timezone: reportZone,
      start: mondayText(requestedAt, reportZone, -1),
      end: mondayText(requestedAt, reportZone, 0),
    });
  });

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

  await test.step("the operator reads the report", async () => {
    await page.getByRole("link", { name: en.report.open, exact: true }).click();
    await expect(page).toHaveURL(/\/reports\/[0-9a-f-]{36}\/overview$/);
    await expect(page.getByRole("heading", { name: en.report.plan.title, level: 2 })).toBeVisible();

    const tabs = page.getByRole("navigation", { name: en.report.tabs });
    const shown = {
      overview: page.getByRole("heading", { name: en.report.plan.title, level: 2 }),
      findings: page.getByLabel(en.report.findings.search, { exact: true }).or(page.getByText(en.report.findings.none, { exact: true })),
      namespaces: page.getByRole("table", { name: en.report.tab.namespaces, exact: true }),
      workloads: page.getByRole("table", { name: en.report.tab.workloads, exact: true }),
      autoscaling: page.getByRole("heading", { name: en.report.autoscaling.title, level: 2 }),
      nodes: page.getByRole("table", { name: en.report.tab.nodes, exact: true }),
      quality: page.getByRole("heading", { name: en.report.quality.warnings, level: 2 }),
    };
    for (const tab of ["overview", "findings", "namespaces", "workloads", "autoscaling", "nodes", "quality"] as const) {
      await tabs.getByRole("link", { name: en.report.tab[tab], exact: true }).click();
      await expect(page).toHaveURL(new RegExp(`/${tab}$`));
      await expect(shown[tab]).toBeVisible();
      await expectNoViolations(page);
    }

    await tabs.getByRole("link", { name: en.report.tab.workloads, exact: true }).click();
    const workloads = page.getByRole("table", { name: en.report.tab.workloads, exact: true });
    const search = page.getByRole("searchbox", { name: en.report.search.workloads, exact: true });
    const closed = workloads.getByRole("button", { name: /^Show the details of / });
    const name = await workloads.getByRole("row").nth(1).locator("span.font-mono").first().innerText();
    // Typed at full speed, with no delay between keys, the field keeps every character and the rows follow it.
    await search.pressSequentially(name);
    await expect(search).toHaveValue(name);
    const rowCount = page.getByText(new RegExp(`^${en.report.table.rows.replace("{{count}}", "(\\d+)")}$`));
    await expect(rowCount).toBeVisible();
    const kept = Number((await rowCount.innerText()).replace(/\D/g, ""));
    expect(kept).toBeGreaterThan(0);
    await expect(closed).toHaveCount(kept);
    await search.fill("");
    await expect(search).toHaveValue("");
    await expect(page).not.toHaveURL(/[?&]q=/);
    await closed.first().click();
    await expect(workloads.getByRole("button", { name: /^Hide the details of / })).toHaveCount(1);
    await expectNoViolations(page);

    await page.getByRole("button", { name: en.progress.pdf, exact: true }).click();
    await expect(pdfGroup(page, en.language.en).getByText(en.pdfStatus.ready, { exact: true })).toBeVisible();
    // The table's sticky header must not cover the popover's link.
    await page.getByRole("dialog").getByRole("group", { name: en.language.en, exact: true }).getByRole("link", { name: en.progress.download, exact: true }).click({ trial: true });
    await expectNoViolations(page);
    await page.keyboard.press("Escape");

    await page.goto(jobUrl);
    await expect(page.getByText(en.reportStatus.collected, { exact: true })).toBeVisible();
  });

  // The reader looks at the progress page while the Portuguese PDF is not
  // generated, the one moment an operator is offered Generate PDF.
  const context = await browser.newContext(baseURL === undefined ? {} : { baseURL });
  try {
    const other = await context.newPage();
    const violations = await watchPolicy(other);

    await test.step("the reader sees no control on the progress page", async () => {
      await expect(pdfGroup(page, en.language["pt-BR"]).getByRole("button", { name: en.progress.generatePdf, exact: true })).toBeVisible();
      await signIn(other, reader, new URL(jobUrl).pathname);
      const english = pdfGroup(other, en.language.en);
      await expect(english.getByText(en.pdfStatus.ready, { exact: true })).toBeVisible();
      await expect(pdfGroup(other, en.language["pt-BR"]).getByText(en.reports.notGenerated)).toBeVisible();
      const section = other.locator("section").filter({ has: other.getByRole("heading", { name: en.progress.pdf, exact: true }) });
      await expect(section.getByRole("link", { name: en.progress.download, exact: true })).toHaveCount(1);
      await expect(section.getByRole("button")).toHaveCount(0);
      await expect(other.getByRole("button", { name: en.progress.deleteReport, exact: true })).toHaveCount(0);
      await expectNoViolations(other);
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
      await other.goto("/reports");
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
      await expect(other.getByRole("link", { name: en.progress.download, exact: true })).toHaveCount(2);
      await expect(other.getByRole("button", { name: en.progress.generatePdf, exact: true })).toHaveCount(0);
      await expectNoViolations(other);

      expect(violations).toEqual([]);
    });
  } finally {
    await context.close();
  }

  await test.step("the operator deletes the report", async () => {
    await page.getByRole("button", { name: en.reports.delete, exact: true }).click();
    const dialog = page.getByRole("alertdialog", { name: en.delete.oneTitle });
    await expect(dialog).toBeVisible();
    await expect(dialog).toContainText(en.delete.oneBody);
    await expectNoViolations(page);
    await dialog.getByRole("button", { name: en.delete.confirm, exact: true }).click();
    await expect(page.getByText(en.reports.empty)).toBeVisible();
    await expectNoViolations(page);
  });
});
