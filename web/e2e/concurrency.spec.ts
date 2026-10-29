import type { Browser, BrowserContext, Page, Route } from "@playwright/test";
import en from "../src/locales/en.json" with { type: "json" };
import { admin, csrfHeaders, expect, expectNoViolations, requestReport, secret, signIn, test } from "./fixtures";

const operator = { username: "e2e-c-operator", password: secret("e2e-C-Operator-Pass-13!") };
const secondAdmin = { username: "e2e-c-admin", password: secret("e2e-C-Admin-Pass-14!") };
const replay = { id: "e2e-c-replay", name: "Replay" };
const admission = { id: "e2e-c-admission", name: "Admission" };
const namespace = "payments";

const escape = (text: string) => text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
// The sentence of a key with parameters, as a pattern that reads each parameter as digits.
const pattern = (text: string) => new RegExp("^" + text.split(/\{\{\w+\}\}/).map(escape).join("\\d+") + "$");

interface Summary {
  id: string;
  profile_id: string;
  status: string;
}

interface Account {
  id: string;
  username: string;
  role: string;
  active: boolean;
}

const alertWith = (page: Page, text: string | RegExp) => page.getByRole("alert").filter({ hasText: text });

// A context of its own, signed in as the user.
async function signedIn(browser: Browser, baseURL: string | undefined, user: { username: string; password: string }) {
  const context = await browser.newContext(baseURL === undefined ? {} : { baseURL });
  const page = await context.newPage();
  await signIn(page, user, "/reports");
  await expect(page.getByRole("heading", { name: en.reports.title, level: 1 })).toBeVisible();
  await expectNoViolations(page);
  return { context, page };
}

// Every report of the list, following the cursor through all pages.
async function allReports(page: Page): Promise<Summary[]> {
  const reports: Summary[] = [];
  let cursor: string | null = null;
  do {
    const query: string = cursor === null ? "" : `?cursor=${encodeURIComponent(cursor)}`;
    const answer = await page.request.get(`/api/v1/reports${query}`);
    expect(answer.status()).toBe(200);
    const body = (await answer.json()) as { items: Summary[]; next_cursor: string | null };
    reports.push(...body.items);
    cursor = body.next_cursor;
  } while (cursor !== null);
  return reports;
}

async function accounts(page: Page): Promise<Account[]> {
  const answer = await page.request.get("/api/v1/admin/users");
  expect(answer.status()).toBe(200);
  return ((await answer.json()) as { users: Account[] }).users;
}

const activeAdministrators = (list: Account[]) => list.filter((account) => account.active && account.role === "administrator");

// Fills the new report form for the profile in English, over all its namespaces: the one it allows.
async function fillForm(page: Page, profileName: string) {
  await expect(page.getByRole("heading", { name: en.generate.title, level: 1 })).toBeVisible();
  await page.getByLabel(en.generate.profile, { exact: true }).selectOption({ label: profileName });
  await page.getByRole("radio", { name: en.language.en, exact: true }).check();
}

// Creates the user, or, when a restarted worker finds it already made, brings it back to the role, active.
async function ensureUser(page: Page, user: { username: string; password: string }, role: "administrator" | "operator") {
  const created = await page.request.post("/api/v1/admin/users", {
    headers: await csrfHeaders(page),
    data: { username: user.username, password: user.password, role, language: "en" },
  });
  if (created.status() === 201) {
    return;
  }
  expect(created.status()).toBe(409);
  expect(((await created.json()) as { error?: { code?: string } }).error?.code).toBe("USERNAME_TAKEN");
  const existing = (await accounts(page)).find((account) => account.username === user.username);
  expect(existing).toBeDefined();
  if (existing !== undefined && (existing.role !== role || !existing.active)) {
    const changed = await page.request.patch(`/api/v1/admin/users/${existing.id}`, {
      headers: await csrfHeaders(page),
      data: { role, active: true },
    });
    expect(changed.status()).toBe(200);
  }
}

// Creates the profile, or accepts the one a restarted worker already made.
async function ensureProfile(page: Page, document: { id: string } & Record<string, unknown>) {
  const created = await page.request.post("/api/v1/admin/profiles", { headers: await csrfHeaders(page), data: document });
  if (created.status() !== 201) {
    expect(created.status()).toBe(409);
    expect(((await created.json()) as { error?: { code?: string } }).error?.code).toBe("PROFILE_ID_TAKEN");
  }
}

test.beforeAll(async ({ browser }, testInfo) => {
  const baseURL = testInfo.project.use.baseURL;
  const { context, page } = await signedIn(browser, baseURL, admin);
  try {
    await ensureUser(page, operator, "operator");
    await ensureUser(page, secondAdmin, "administrator");
    for (const profile of [replay, admission]) {
      await ensureProfile(page, {
        id: profile.id,
        display_name: profile.name,
        endpoint_url: "http://fake-prometheus:9091/api/v1",
        cluster_name: "prod",
        allowed_namespaces: [namespace],
        active: true,
      });
    }
  } finally {
    await context.close();
  }
});

test("a lost answer and a double click make one report each", async ({ page }) => {
  await signIn(page, operator, "/generate");
  await fillForm(page, replay.name);

  // The first request reaches the server and its answer is thrown away.
  let recorded = "";
  const lose = async (route: Route) => {
    if (route.request().method() !== "POST") {
      await route.fallback();
      return;
    }
    const answered = await route.fetch();
    recorded = ((await answered.json()) as { id: string }).id;
    await route.abort("failed");
  };
  await page.route("**/api/v1/reports", lose);
  await page.getByRole("button", { name: en.generate.submit, exact: true }).click();
  await expect(alertWith(page, en.error.network)).toBeVisible();
  expect(recorded).not.toBe("");
  await expectNoViolations(page);

  // The resend carries the same key, so the server replays the report it already made.
  await page.unroute("**/api/v1/reports", lose);
  await page.getByRole("button", { name: en.generate.submit, exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/jobs/${recorded}$`));
  await expectNoViolations(page);

  await page.goto("/generate");
  await fillForm(page, replay.name);
  await page.getByRole("button", { name: en.generate.submit, exact: true }).dblclick();
  await expect(page).toHaveURL(/\/jobs\/[0-9a-f-]{36}$/);
  expect(page.url()).not.toContain(recorded);
  await expectNoViolations(page);

  const made = (await allReports(page)).filter((report) => report.profile_id === replay.id);
  expect(made).toHaveLength(2);
});

test("a request beyond the admission limit of a profile is refused", async ({ page }) => {
  await signIn(page, operator, "/generate");
  await expect(page.getByRole("heading", { name: en.generate.title, level: 1 })).toBeVisible();

  for (let n = 0; n < 5; n++) {
    const accepted = await requestReport(page, { profile_id: admission.id, language: "en", namespaces: [namespace] });
    expect(accepted.status()).toBe(202);
  }
  const queued = (await allReports(page)).filter((report) => report.profile_id === admission.id);
  expect(queued).toHaveLength(5);
  for (const report of queued) {
    expect(["queued", "running"]).toContain(report.status);
  }

  await fillForm(page, admission.name);
  await page.getByRole("button", { name: en.generate.submit, exact: true }).click();
  await expect(alertWith(page, pattern(en.error.ADMISSION_PROFILE_LIMIT))).toBeVisible();
  await expect(page).toHaveURL(/\/generate$/);
  await expectNoViolations(page);
});

test("a second save of one profile is refused as stale", async ({ page }) => {
  await signIn(page, admin, `/profiles/${replay.id}/edit`);
  const nameOf = (tab: Page) => tab.getByLabel(en.admin.profile.name, { exact: true });
  await expect(nameOf(page)).toHaveValue(replay.name);

  const tabB = await page.context().newPage();
  await tabB.goto(`/profiles/${replay.id}/edit`);
  await expect(nameOf(tabB)).toHaveValue(replay.name);

  await nameOf(page).fill("Replay A");
  await page.getByRole("button", { name: en.admin.profile.save, exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/profiles\\?open=${replay.id}$`));
  await expectNoViolations(page);

  await nameOf(tabB).fill("Replay B");
  await tabB.getByRole("button", { name: en.admin.profile.save, exact: true }).click();
  await expect(alertWith(tabB, en.admin.profile.stale)).toBeVisible();
  await expect(tabB.getByRole("button", { name: en.admin.profile.loadLatest, exact: true })).toBeVisible();
  await expect(nameOf(tabB)).toHaveValue("Replay B");
  await expectNoViolations(tabB);
});

test("two administrators lowering each other leave one", async ({ browser, baseURL }) => {
  const contexts: BrowserContext[] = [];
  let failure: Error | undefined;
  try {
    const first = await signedIn(browser, baseURL, admin);
    contexts.push(first.context);
    const second = await signedIn(browser, baseURL, secondAdmin);
    contexts.push(second.context);

    const listed = await accounts(first.page);
    const firstId = listed.find((account) => account.username === admin.username)?.id;
    const secondId = listed.find((account) => account.username === secondAdmin.username)?.id;
    expect(firstId).toBeDefined();
    expect(secondId).toBeDefined();
    expect(activeAdministrators(listed).map((account) => account.username).sort()).toEqual([admin.username, secondAdmin.username]);

    // Each side's token is read first, since reading it is itself a request of the session.
    const firstHeaders = await csrfHeaders(first.page);
    const secondHeaders = await csrfHeaders(second.page);
    const [lowersSecond, lowersFirst] = await Promise.all([
      first.page.request.patch(`/api/v1/admin/users/${secondId ?? ""}`, { headers: firstHeaders, data: { role: "operator" } }),
      second.page.request.patch(`/api/v1/admin/users/${firstId ?? ""}`, { headers: secondHeaders, data: { role: "operator" } }),
    ]);

    const winners = [
      { by: first.page, answer: lowersSecond },
      { by: second.page, answer: lowersFirst },
    ].filter((side) => side.answer.status() === 200);
    expect(winners).toHaveLength(1);
    const winner = winners[0];
    const loser = winner?.answer === lowersSecond ? lowersFirst : lowersSecond;
    test.info().annotations.push({
      type: "race",
      description: `${winner?.by === first.page ? admin.username : secondAdmin.username} won, the other answered ${String(loser.status())}`,
    });
    const refusal = (await loser.json()) as { error?: { code?: string } };
    if (loser.status() === 409) {
      expect(refusal.error?.code).toBe("LAST_ADMIN");
    } else {
      expect(loser.status()).toBe(401);
    }

    const remaining = activeAdministrators(await accounts(winner?.by ?? first.page));
    expect(remaining).toHaveLength(1);
  } catch (thrown) {
    failure = thrown instanceof Error ? thrown : new Error(String(thrown));
  }
  // The restore runs whatever a close does; the first failure is the one reported.
  for (const closed of await Promise.allSettled(contexts.map((context) => context.close()))) {
    if (closed.status === "rejected") {
      failure ??= closed.reason instanceof Error ? closed.reason : new Error(String(closed.reason));
    }
  }
  try {
    await restoreAdministrator(browser, baseURL);
  } catch (thrown) {
    failure ??= thrown instanceof Error ? thrown : new Error(String(thrown));
  }
  if (failure !== undefined) {
    throw failure;
  }
});

// Leaves e2e-admin the one active administrator, whichever side of the race stayed.
async function restoreAdministrator(browser: Browser, baseURL: string | undefined) {
  const contexts: BrowserContext[] = [];
  try {
    let current = await signedIn(browser, baseURL, admin);
    contexts.push(current.context);
    if ((await current.page.request.get("/api/v1/admin/users")).status() !== 200) {
      // e2e-admin was lowered: the other administrator raises it, and its session starts again.
      const other = await signedIn(browser, baseURL, secondAdmin);
      contexts.push(other.context);
      const lowered = (await accounts(other.page)).find((account) => account.username === admin.username);
      const raised = await other.page.request.patch(`/api/v1/admin/users/${lowered?.id ?? ""}`, {
        headers: await csrfHeaders(other.page),
        data: { role: "administrator" },
      });
      expect(raised.status()).toBe(200);
      current = await signedIn(browser, baseURL, admin);
      contexts.push(current.context);
    }
    const second = (await accounts(current.page)).find((account) => account.username === secondAdmin.username);
    const deactivated = await current.page.request.patch(`/api/v1/admin/users/${second?.id ?? ""}`, {
      headers: await csrfHeaders(current.page),
      data: { active: false },
    });
    expect(deactivated.status()).toBe(200);
    expect(activeAdministrators(await accounts(current.page)).map((account) => account.username)).toEqual([admin.username]);
  } finally {
    await Promise.all(contexts.map((context) => context.close()));
  }
}
