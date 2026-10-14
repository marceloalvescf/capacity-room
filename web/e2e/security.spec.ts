import type { Browser, BrowserContext, Page, Response } from "@playwright/test";
import en from "../src/locales/en.json" with { type: "json" };
import ptBR from "../src/locales/pt-BR.json" with { type: "json" };
import { admin, createUser, csrfHeaders, expect, expectNoViolations, secret, signIn, test, watchPolicy } from "./fixtures";

const users = {
  ended: { username: "e2e-s-ended", password: secret("e2e-S-Ended-Pass-07!") },
  deactivated: { username: "e2e-s-deactivated", password: secret("e2e-S-Deactivated-Pass-08!") },
  role: { username: "e2e-s-role", password: secret("e2e-S-Role-Pass-09!") },
  csrf: { username: "e2e-s-csrf", password: secret("e2e-S-Csrf-Pass-10!") },
  policy: { username: "e2e-s-policy", password: secret("e2e-S-Policy-Pass-11!") },
  throttle: { username: "e2e-s-throttle", password: secret("e2e-S-Throttle-Pass-12!") },
};
const wrongPassword = secret("wrong-Password-00!");

const escape = (text: string) => text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
// The words of a name that come before its first parameter.
const before = (text: string) => text.split("{{")[0] ?? text;
const languageButton = (words: { header: { language: string } }) =>
  new RegExp("^" + escape(before(words.header.language)));
// The sentence of a key with parameters, as a pattern that reads each parameter as digits.
const pattern = (text: string) =>
  new RegExp("^" + text.split(/\{\{\w+\}\}/).map(escape).join("\\d+") + "$");

// Signs the user in on the login page that is open.
async function fillSignIn(page: Page, user: { username: string; password: string }) {
  await page.getByLabel(en.login.username, { exact: true }).fill(user.username);
  await page.getByLabel(en.login.password, { exact: true }).fill(user.password);
  await page.getByRole("button", { name: en.login.submit, exact: true }).click();
}

// A context of its own with a page whose policy violations are collected.
async function open(browser: Browser, baseURL: string | undefined) {
  const context = await browser.newContext(baseURL === undefined ? {} : { baseURL });
  const page = await context.newPage();
  const violations = await watchPolicy(page);
  return { context, page, violations };
}

// The administrator creates the user from a context of its own, which stays open for later changes.
async function createdBy(browser: Browser, baseURL: string | undefined, user: { username: string; password: string }, role: "operator" | "reader") {
  const opened = await open(browser, baseURL);
  await signIn(opened.page, admin, "/reports");
  await expect(opened.page.getByRole("heading", { name: en.reports.title, level: 1 })).toBeVisible();
  const id = await createUser(opened.page, user, role);
  return { ...opened, id };
}

async function patchUser(page: Page, id: string, data: Record<string, unknown>) {
  const changed = await page.request.patch(`/api/v1/admin/users/${id}`, { headers: await csrfHeaders(page), data });
  expect(changed.status()).toBe(200);
}

const navigation = (page: Page) => page.getByRole("navigation", { name: en.nav.label });

async function closeAll(...contexts: BrowserContext[]) {
  for (const context of contexts) {
    await context.close();
  }
}

test("a session ended in another tab sends the page to sign-in and back", async ({ browser, baseURL }) => {
  const maker = await createdBy(browser, baseURL, users.ended, "operator");
  const { context, page: tabA, violations } = await open(browser, baseURL);
  try {
    await signIn(tabA, users.ended, "/account");
    await expect(tabA.getByRole("heading", { name: en.account.title, level: 1 })).toBeVisible();
    await expectNoViolations(tabA);

    const tabB = await context.newPage();
    await tabB.goto("/reports");
    await tabB.getByRole("button", { name: en.header.signOut }).click();
    await expect(tabB.getByRole("heading", { name: en.login.title })).toBeVisible();

    await navigation(tabA).getByRole("link", { name: en.nav.reports, exact: true }).click();
    await expect
      .poll(() => {
        const url = new URL(tabA.url());
        return { path: url.pathname, next: url.searchParams.get("next"), ended: url.searchParams.get("ended") };
      })
      .toEqual({ path: "/login", next: "/reports", ended: "1" });
    await expect(tabA.getByText(en.login.sessionEnded)).toBeVisible();
    await expectNoViolations(tabA);

    await fillSignIn(tabA, users.ended);
    await expect(tabA).toHaveURL(/\/reports$/);
    await expect(tabA.getByRole("heading", { name: en.reports.title, level: 1 })).toBeVisible();
    await expectNoViolations(tabA);
    expect(violations).toEqual([]);
  } finally {
    await closeAll(context, maker.context);
  }
});

test("a deactivated user is signed out and cannot sign in", async ({ browser, baseURL }) => {
  const maker = await createdBy(browser, baseURL, users.deactivated, "operator");
  const { context, page, violations } = await open(browser, baseURL);
  try {
    await signIn(page, users.deactivated, "/account");
    await expect(page.getByRole("heading", { name: en.account.title, level: 1 })).toBeVisible();
    await expectNoViolations(page);

    await patchUser(maker.page, maker.id, { active: false });

    await navigation(page).getByRole("link", { name: en.nav.reports, exact: true }).click();
    await expect(page.getByRole("heading", { name: en.login.title })).toBeVisible();
    await expect(page.getByText(en.login.sessionEnded)).toBeVisible();
    await expectNoViolations(page);

    await fillSignIn(page, users.deactivated);
    await expect(page.getByText(en.error.INVALID_CREDENTIALS)).toBeVisible();
    await expectNoViolations(page);
    expect(violations).toEqual([]);
  } finally {
    await closeAll(context, maker.context);
  }
});

test("a role change ends the session and the new role holds", async ({ browser, baseURL }) => {
  const maker = await createdBy(browser, baseURL, users.role, "operator");
  const { context, page, violations } = await open(browser, baseURL);
  try {
    await signIn(page, users.role, "/account");
    await expect(page.getByRole("heading", { name: en.account.title, level: 1 })).toBeVisible();
    await expectNoViolations(page);
    await expect(navigation(page).getByRole("link", { name: en.nav.generate, exact: true })).toBeVisible();

    await patchUser(maker.page, maker.id, { role: "reader" });

    await navigation(page).getByRole("link", { name: en.nav.reports, exact: true }).click();
    await expect(page.getByRole("heading", { name: en.login.title })).toBeVisible();
    await expect(page.getByText(en.login.sessionEnded)).toBeVisible();

    await fillSignIn(page, users.role);
    await expect(page).toHaveURL(/\/reports$/);
    await expect(page.getByRole("heading", { name: en.reports.title, level: 1 })).toBeVisible();
    await expectNoViolations(page);
    await expect(navigation(page).getByRole("link", { name: en.nav.account, exact: true })).toBeVisible();
    await expect(navigation(page).getByRole("link", { name: en.nav.generate, exact: true })).toHaveCount(0);

    await page.goto("/generate");
    await expect(page.getByRole("heading", { name: en.notFound.title, level: 1 })).toBeVisible();
    await expectNoViolations(page);
    expect(violations).toEqual([]);
  } finally {
    await closeAll(context, maker.context);
  }
});

test("a write without the CSRF token is refused, and a rotated token heals", async ({ browser, baseURL }) => {
  const maker = await createdBy(browser, baseURL, users.csrf, "operator");
  const { context, page: tabA, violations } = await open(browser, baseURL);
  try {
    await signIn(tabA, users.csrf, "/account");
    await expect(tabA.getByRole("heading", { name: en.account.title, level: 1 })).toBeVisible();
    await expectNoViolations(tabA);

    const refused = await tabA.request.post("/api/v1/auth/logout");
    expect(refused.status()).toBe(403);
    expect(((await refused.json()) as { error?: { code?: string } }).error?.code).toBe("CSRF_REJECTED");
    const session = await tabA.request.get("/api/v1/auth/session");
    expect(((await session.json()) as { authenticated: boolean }).authenticated).toBe(true);

    const answers: Response[] = [];
    tabA.on("response", (response) => {
      if (response.request().method() === "PATCH" && new URL(response.url()).pathname.startsWith("/api/v1/account")) {
        answers.push(response);
      }
    });

    // Loading the app in a second tab reads the session, which rotates the token; nothing runs between this load
    // and the write of the first tab.
    const tabB = await context.newPage();
    await tabB.goto("/reports");
    await expect(tabB.getByRole("heading", { name: en.reports.title, level: 1 })).toBeVisible();

    await tabA.getByRole("button", { name: languageButton(en) }).click();
    await tabA.getByRole("menuitemradio", { name: ptBR.language["pt-BR"] }).click();
    await expect(tabA.getByRole("heading", { name: ptBR.account.title, level: 1 })).toBeVisible();
    await expect(tabA.getByRole("tooltip")).toBeVisible();
    await expectNoViolations(tabA);
    await expect.poll(() => answers.length).toBe(2);
    const [first, second] = answers;
    expect(first?.status()).toBe(403);
    expect(new URL(first?.url() ?? "").pathname).toBe("/api/v1/account");
    expect(((await first?.json()) as { error?: { code?: string } }).error?.code).toBe("CSRF_REJECTED");
    expect(second?.status()).toBe(200);
    expect(new URL(second?.url() ?? "").pathname).toBe("/api/v1/account");

    await tabA.getByRole("button", { name: languageButton(ptBR) }).click();
    await tabA.getByRole("menuitemradio", { name: en.language.en }).click();
    await expect(tabA.getByRole("heading", { name: en.account.title, level: 1 })).toBeVisible();
    await expect(tabA.getByRole("tooltip")).toBeVisible();
    await expectNoViolations(tabA);
    expect(violations).toEqual([]);
  } finally {
    await closeAll(context, maker.context);
  }
});

test("an injected inline script is blocked, and the headers and the cookie hold", async ({ browser, baseURL }) => {
  const maker = await createdBy(browser, baseURL, users.policy, "reader");
  const { context, page, violations } = await open(browser, baseURL);
  try {
    await signIn(page, users.policy, "/account");
    await expect(page.getByRole("heading", { name: en.account.title, level: 1 })).toBeVisible();
    await expectNoViolations(page);

    expect(violations).toEqual([]);
    await page.evaluate(() => {
      const script = document.createElement("script");
      script.textContent = "window.capacityroomInjected = true";
      document.body.appendChild(script);
    });
    expect(await page.evaluate(() => "capacityroomInjected" in window)).toBe(false);
    // The browser reports the refusal twice: its own console line and the page's listener.
    await expect.poll(() => violations.length).toBeGreaterThan(0);
    for (const violation of violations) {
      expect(violation).toContain("script-src");
    }

    const headers = {
      "content-security-policy":
        "default-src 'self'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'",
      "x-content-type-options": "nosniff",
      "referrer-policy": "same-origin",
      "x-frame-options": "DENY",
    };
    const root = await page.request.get("/");
    const asset = /src="(\/assets\/[^"]+\.js)"/.exec(await root.text())?.[1];
    expect(asset).toBeDefined();
    const caching = [
      ["/", "no-cache"],
      ["/api/v1/auth/session", "no-store"],
      [asset ?? "", "public, max-age=31536000, immutable"],
    ] as const;
    for (const [path, cacheControl] of caching) {
      const response = await page.request.get(path);
      expect(response.status(), path).toBe(200);
      expect(response.headers(), path).toMatchObject({ ...headers, "cache-control": cacheControl });
    }

    const cookie = (await context.cookies()).find((candidate) => candidate.name === "capacityroom_session");
    expect(cookie).toMatchObject({ httpOnly: true, secure: true, sameSite: "Lax", path: "/api" });
  } finally {
    await closeAll(context, maker.context);
  }
});

test("five wrong passwords block the sixth sign-in", async ({ browser, baseURL }) => {
  const maker = await createdBy(browser, baseURL, users.throttle, "reader");
  const { context, page, violations } = await open(browser, baseURL);
  try {
    await page.goto("/login");
    await expect(page.getByRole("heading", { name: en.login.title })).toBeVisible();
    const attempt = async (password: string) => {
      await page.getByLabel(en.login.username, { exact: true }).fill(users.throttle.username);
      await page.getByLabel(en.login.password, { exact: true }).fill(password);
      const answered = page.waitForResponse((response) => new URL(response.url()).pathname === "/api/v1/auth/login");
      await page.getByRole("button", { name: en.login.submit, exact: true }).click();
      return answered;
    };
    for (let n = 0; n < 5; n++) {
      expect((await attempt(wrongPassword)).status()).toBe(401);
      await expect(page.getByText(en.error.INVALID_CREDENTIALS)).toBeVisible();
    }
    await attempt(users.throttle.password);
    await expect(page.getByText(pattern(en.error.AUTH_RATE_LIMITED))).toBeVisible();
    await expect(page.getByRole("heading", { name: en.login.title })).toBeVisible();
    await expectNoViolations(page);
    expect(violations).toEqual([]);
  } finally {
    await closeAll(context, maker.context);
  }
});
