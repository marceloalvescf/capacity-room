import AxeBuilder from "@axe-core/playwright";
import { expect, test as base, type Page } from "@playwright/test";

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
