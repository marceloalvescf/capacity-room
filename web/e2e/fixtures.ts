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

// Matches the open button of any row of a table.
export const openPattern = new RegExp(
  `^${en.table.open.split("{{name}}").map((part) => part.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")).join(".*")}$`,
);

// The words of a row's button, with the row's name.
export const buttonName = (word: string, name: string) => word.replace("{{name}}", name);

// The last row of a table: its open button and its name, read from the button.
export async function lastRow(table: Locator): Promise<{ button: Locator; name: string }> {
  const last = table.locator(":scope > tbody > tr > td:first-child").getByRole("button", { name: openPattern }).last();
  const label = (await last.getAttribute("aria-label")) ?? "";
  const [before = "", after = ""] = en.table.open.split("{{name}}");
  const name = label.slice(before.length, label.length - after.length);
  return { button: table.getByRole("button", { name: buttonName(en.table.open, name), exact: true }), name };
}

// Waits until the row named `name` is open, then until it and its opened row lie inside the viewport, below the
// sticky header of the table, and the page does not scroll sideways. The scroll that reveals a row is smooth, so
// the check polls until it settles.
export async function expectFits(page: Page, table: Locator, name: string) {
  const closeButton = (scope: Locator) => scope.getByRole("button", { name: buttonName(en.table.close, name), exact: true });
  // Only the rows of the table itself: an opened row may hold tables of its own.
  const rows = table.locator(":scope > tbody > tr");
  await expect(closeButton(rows.locator(":scope > td:first-child"))).toBeVisible();
  const row = rows.filter({ has: closeButton(page.locator("td:first-child")) });
  const opened = row.locator("xpath=following-sibling::tr[1]");
  await expect(opened).toBeVisible();
  await expect
    .poll(
      async () => {
        const size = page.viewportSize();
        const headBox = await table.locator(":scope > thead").boundingBox();
        const rowBox = await row.boundingBox();
        const openedBox = await opened.boundingBox();
        const sideways = await page.evaluate(
          () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
        );
        if (size === null || rowBox === null || openedBox === null) {
          return { outside: ["not measured"], sideways };
        }
        const head = headBox === null ? 0 : Math.max(0, headBox.y + headBox.height);
        const numbers = (label: string, top: number, bottom: number) =>
          `${label} top=${top.toFixed(1)} bottom=${bottom.toFixed(1)} head=${head.toFixed(1)} innerHeight=${String(size.height)}`;
        const rowTop = rowBox.y;
        const openedTop = openedBox.y;
        const openedBottom = openedBox.y + openedBox.height;
        const pair = openedBottom - rowTop;
        const room = size.height - head - 16;
        const outside: string[] = [];
        const wholly = (label: string, top: number, bottom: number) => {
          if (top < head - 1 || bottom > size.height) {
            outside.push(numbers(label, top, bottom));
          }
        };
        if (pair <= room) {
          wholly("row", rowTop, rowBox.y + rowBox.height);
          wholly("opened", openedTop, openedBottom);
        } else {
          // The pair is taller than the window: the row is wholly inside, just under the header, and the opened row
          // starts inside the window.
          wholly("row", rowTop, rowBox.y + rowBox.height);
          if (Math.abs(rowTop - (head + 8)) > 12) {
            outside.push(`${numbers("row", rowTop, rowBox.y + rowBox.height)} expected top near ${(head + 8).toFixed(1)}`);
          }
          if (openedTop < head - 1 || openedTop >= size.height) {
            outside.push(numbers("opened", openedTop, openedBottom));
          }
        }
        for (const [label, box] of [["row", rowBox], ["opened", openedBox]] as const) {
          if (box.x < 0 || box.x + box.width > size.width) {
            outside.push(`${label} left=${box.x.toFixed(1)} right=${(box.x + box.width).toFixed(1)} innerWidth=${String(size.width)}`);
          }
        }
        return { outside, sideways };
      },
      { timeout: 10_000 },
    )
    .toEqual({ outside: [], sideways: false });
}

// The session and the CSRF token of the signed-in page, for a change made through the API.
async function csrfHeaders(page: Page): Promise<{ "X-CSRF-Token": string }> {
  const session = await page.request.get("/api/v1/auth/session");
  expect(session.ok()).toBe(true);
  const { csrf_token: csrf } = (await session.json()) as { csrf_token: string };
  return { "X-CSRF-Token": csrf };
}

// Creates a user through the API, with the session and the CSRF token of the signed-in page.
export async function createUser(page: Page, user: { username: string; password: string }, role: "operator" | "reader") {
  const created = await page.request.post("/api/v1/admin/users", {
    headers: await csrfHeaders(page),
    data: { username: user.username, password: user.password, role, language: "en" },
  });
  expect(created.status()).toBe(201);
}

// Creates a profile through the API.
export async function createProfile(page: Page, document: Record<string, unknown>) {
  const created = await page.request.post("/api/v1/admin/profiles", { headers: await csrfHeaders(page), data: document });
  expect(created.status()).toBe(201);
}

// Deletes a profile through the API.
export async function deleteProfile(page: Page, id: string) {
  const deleted = await page.request.delete(`/api/v1/admin/profiles/${id}`, { headers: await csrfHeaders(page) });
  expect(deleted.ok()).toBe(true);
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
