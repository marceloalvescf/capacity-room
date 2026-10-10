import { readFile } from "node:fs/promises";
import type { Frame, Locator, Page } from "@playwright/test";
import en from "../src/locales/en.json" with { type: "json" };
import { admin, createUser, expect, expectNoViolations, previousWeek, reportRow, requestReport, secret, signIn, switchCompiles, test } from "./fixtures";

const operator = { username: "e2e-r-operator", password: secret("e2e-R-Operator-Pass-15!") };
const zone = "America/Sao_Paulo";
const wait = { timeout: 60_000 };

const pdfGroup = (page: Page, language: string) => page.getByRole("group", { name: language, exact: true });

// The first bytes of the file a link downloads.
async function downloaded(page: Page, link: Locator): Promise<string> {
  const downloading = page.waitForEvent("download");
  await link.click();
  const bytes = await readFile(await (await downloading).path());
  return bytes.subarray(0, 5).toString("latin1");
}

// The stack's recovery fake serves its scenario once, so one stack runs exactly one collection of this profile:
// this is the only report the spec requests, and the Playwright configuration keeps retries at zero.
test("a failed PDF is made again once the cause is gone", async ({ page }) => {
  test.setTimeout(300_000);
  let id = "";
  try {
    await test.step("the list shows the report collected with no reload", async () => {
      await signIn(page, admin, "/account");
      await expect(page.getByRole("heading", { name: en.account.title, level: 1 })).toBeVisible();
      await createUser(page, operator, "operator");
      await page.getByRole("button", { name: en.header.signOut }).click();
      await expect(page.getByRole("heading", { name: en.login.title })).toBeVisible();
      await switchCompiles(true);

      await signIn(page, operator, "/reports");
      await expect(page.getByRole("heading", { name: en.reports.title, level: 1 })).toBeVisible();
      const created = await requestReport(page, {
        profile_id: "e2e-recovery",
        language: "en",
        namespaces: ["payments"],
        timezone: zone,
        ...previousWeek(new Date(), zone),
      });
      expect(created.status()).toBe(202);
      id = ((await created.json()) as { id: string }).id;

      // The last navigation of the step: it loads the list with the new row.
      await page.reload();
      const row = reportRow(page, id);
      await expect(row).toHaveCount(1);
      await expect(row.getByText(new RegExp(`^(${en.reportStatus.queued}|${en.reportStatus.running})$`))).toBeVisible();

      let navigations = 0;
      const count = (frame: Frame) => {
        if (frame === page.mainFrame()) {
          navigations += 1;
        }
      };
      page.on("framenavigated", count);
      await expect(row.getByText(en.reportStatus.collected, { exact: true })).toBeVisible(wait);
      await expect(row.getByText(en.pdfStatus.failed, { exact: true })).toBeVisible(wait);
      expect(navigations).toBe(0);
      page.off("framenavigated", count);
      await expectNoViolations(page);
    });

    await test.step("a failed PDF is made again once the cause is gone", async () => {
      await page.goto(`/jobs/${id}`);
      const english = pdfGroup(page, en.language.en);
      await expect(english.getByText(en.pdfStatus.failed, { exact: true })).toBeVisible();
      await expect(english.getByText(en.pdfFailure.PDF_COMPILE_FAILED, { exact: true })).toBeVisible();
      await expect(english.getByRole("button", { name: en.progress.retryPdf, exact: true })).toBeVisible();
      await expectNoViolations(page);

      await switchCompiles(false);
      await english.getByRole("button", { name: en.progress.retryPdf, exact: true }).click();
      await expect(english.getByText(en.pdfStatus.ready, { exact: true })).toBeVisible(wait);
      expect(await downloaded(page, english.getByRole("link", { name: en.progress.download, exact: true }))).toBe("%PDF-");
    });

    await test.step("a failed PDF of the second language leaves the first", async () => {
      await switchCompiles(true);
      const portuguese = pdfGroup(page, en.language["pt-BR"]);
      await portuguese.getByRole("button", { name: en.progress.generatePdf, exact: true }).click();
      await expect(portuguese.getByText(en.pdfStatus.failed, { exact: true })).toBeVisible(wait);
      const english = pdfGroup(page, en.language.en);
      await expect(english.getByText(en.pdfStatus.ready, { exact: true })).toBeVisible();
      expect(await downloaded(page, english.getByRole("link", { name: en.progress.download, exact: true }))).toBe("%PDF-");
    });

    await test.step("two requests for one PDF make one", async () => {
      await switchCompiles(false);
      const other = await page.context().newPage();
      await other.goto(`/jobs/${id}`);
      const tabs = [page, other];
      const retry = (tab: Page) => pdfGroup(tab, en.language["pt-BR"]).getByRole("button", { name: en.progress.retryPdf, exact: true });
      for (const tab of tabs) {
        await expect(retry(tab)).toBeVisible();
      }
      await Promise.all(tabs.map((tab) => retry(tab).click()));
      for (const tab of tabs) {
        await expect(pdfGroup(tab, en.language["pt-BR"]).getByText(en.pdfStatus.ready, { exact: true })).toBeVisible(wait);
        await expect(tab.locator("#main").getByRole("alert")).toHaveCount(0);
      }

      const answer = await page.request.get(`/api/v1/reports/${id}`);
      expect(answer.status()).toBe(200);
      const { pdfs } = (await answer.json()) as { pdfs: { language: string; status: string; failure_code?: string | null }[] };
      expect(pdfs.map((pdf) => pdf.language).sort()).toEqual(["en", "pt-BR"]);
      for (const pdf of pdfs) {
        expect(pdf.status).toBe("ready");
        expect(pdf.failure_code ?? null).toBeNull();
      }

      const link = pdfGroup(page, en.language["pt-BR"]).getByRole("link", { name: en.progress.download, exact: true });
      expect(await downloaded(page, link)).toBe("%PDF-");
      for (const tab of tabs) {
        await expectNoViolations(tab);
      }
    });
  } finally {
    await switchCompiles(false);
  }
});
