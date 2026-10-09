import AxeBuilder from "@axe-core/playwright";
import { expect, test as base, type Locator, type Page } from "@playwright/test";
import en from "../src/locales/en.json" with { type: "json" };

const policyMessage = /Content[- ]Security[- ]Policy|securitypolicyviolation/i;

// Collects every policy violation the page reports; the caller expects the
// list to be empty when the page is done.
export async function watchPolicy(page: Page): Promise<string[]> {
  const messages: string[] = [];
  page.on("console", (message) => {
    if (policyMessage.test(message.text())) {
      messages.push(message.text());
    }
  });
  await page.addInitScript({ path: "e2e/csp-listener.js" });
  return messages;
}

export const test = base.extend<{ cspGuard: undefined }>({
  cspGuard: [
    async ({ page }, use) => {
      const messages = await watchPolicy(page);
      await use(undefined);
      expect(messages).toEqual([]);
    },
    { auto: true },
  ],
});

export async function expectNoViolations(page: Page) {
  const result = await new AxeBuilder({ page }).analyze();
  expect(result.violations).toEqual([]);
}

export { expect };

// The window sizes at which an opened table row must fit.
export const fitSizes = [
  { width: 1366, height: 768 },
  { width: 1200, height: 1000 },
  { width: 1024, height: 768 },
] as const;

// Waits until the opened row of a table and the row above it lie inside the viewport and the page does not
// scroll sideways. The scroll that reveals a row is smooth, so the check polls until it settles.
export async function expectFits(page: Page, opened: Locator) {
  const above = opened.locator("xpath=preceding-sibling::tr[1]");
  await expect
    .poll(
      async () => {
        const size = page.viewportSize();
        const rows = { opened: await opened.boundingBox(), above: await above.boundingBox() };
        const sideways = await page.evaluate(
          () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
        );
        const outside = Object.entries(rows).filter(
          ([, box]) =>
            size === null ||
            box === null ||
            box.y < 0 ||
            box.x < 0 ||
            box.y + box.height > size.height ||
            box.x + box.width > size.width,
        );
        return { outside: outside.map(([name]) => name), sideways };
      },
      { timeout: 10_000 },
    )
    .toEqual({ outside: [], sideways: false });
}

export const admin = { username: "e2e-admin", password: "e2e-Fixture-Pass-01!" };

// Opens the path, which asks for sign-in, and signs the user in.
export async function signIn(page: Page, user: { username: string; password: string }, path: string) {
  await page.goto(path);
  await expect(page.getByRole("heading", { name: en.login.title })).toBeVisible({ timeout: 10_000 });
  await page.getByLabel(en.login.username, { exact: true }).fill(user.username);
  await page.getByLabel(en.login.password, { exact: true }).fill(user.password);
  await page.getByRole("button", { name: en.login.submit, exact: true }).click();
}
