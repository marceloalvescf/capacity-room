import type { Page } from "@playwright/test";
import en from "../src/locales/en.json" with { type: "json" };
import { admin, createProfile, createUser, csrfHeaders, expect, expectNoViolations, secret, signIn, test } from "./fixtures";

const operator = { username: "e2e-h-operator", password: secret("e2e-H-Operator-Pass-07!") };
const reader = { username: "e2e-h-reader", password: secret("e2e-H-Reader-Pass-08!") };

// An active profile that has no report and never will: nothing listens at its endpoint, so no collection may run.
const empty = { id: "e2e-h-empty", name: "Home empty" };

// The chip of a profile in the strip of clusters, found by the address it opens.
const chip = (page: Page, profileId: string) =>
  page.getByRole("list", { name: en.home.strip.heading }).locator(`a[href="/?cluster=${profileId}"]`);

// Marks the profile inactive through the API, as its current revision allows.
async function deactivate(page: Page, id: string) {
  const read = await page.request.get(`/api/v1/admin/profiles/${id}`);
  expect(read.ok()).toBe(true);
  const { revision_id: revision } = (await read.json()) as { revision_id: string };
  const revised = await page.request.patch(`/api/v1/admin/profiles/${id}`, {
    headers: await csrfHeaders(page),
    data: { base_revision_id: revision, active: false },
  });
  expect(revised.ok()).toBe(true);
}

async function signOut(page: Page) {
  await page.getByRole("button", { name: en.header.signOut }).click();
  await expect(page.getByRole("heading", { name: en.login.title })).toBeVisible();
}

// No collection runs here: the home page reads what the other specs and the fixtures stored.
test("the home page shows the clusters and the way to request a report", async ({ page }) => {
  const made = { profile: false };
  try {
    await test.step("the administrator creates an operator, a reader, and a profile with no report", async () => {
      await signIn(page, admin, "/account");
      await expect(page.getByRole("heading", { name: en.account.title, level: 1 })).toBeVisible();
      await createUser(page, operator, "operator");
      await createUser(page, reader, "reader");
      await createProfile(page, {
        id: empty.id,
        display_name: empty.name,
        endpoint_url: "http://fake-prometheus:9091/api/v1",
        cluster_name: "prod",
        allowed_namespaces: ["payments"],
        active: true,
      });
      made.profile = true;
      await signOut(page);
    });

    await test.step("the operator sees the chips of the active profiles", async () => {
      await signIn(page, operator, "/");
      await expect(page.getByRole("heading", { name: en.home.title, level: 1 })).toBeVisible();
      await expect(chip(page, "e2e-prod")).toBeVisible();
      await expect(chip(page, "e2e-recovery")).toBeVisible();
      await expect(chip(page, empty.id)).toContainText(empty.name);
      await expect(chip(page, empty.id)).toContainText(en.home.chip.none);
      await expectNoViolations(page);
    });

    await test.step("the operator chooses the cluster with no report and can request one", async () => {
      await chip(page, empty.id).click();
      await expect(chip(page, empty.id)).toHaveAttribute("aria-current", "true");
      await expect(page.getByRole("heading", { name: empty.name, level: 2 })).toBeVisible();
      await expect(page.getByText(en.home.answer.none, { exact: true })).toBeVisible();
      await expect(page.getByRole("link", { name: en.home.answer.open, exact: true })).toHaveCount(0);
      await expectNoViolations(page);

      await page.getByRole("link", { name: en.home.answer.request, exact: true }).click();
      await expect(page).toHaveURL(new RegExp(`/generate\\?profile=${empty.id}$`));
      await expect(page.getByRole("heading", { name: en.generate.title, level: 1 })).toBeVisible();
      await expect(page.getByLabel(en.generate.profile, { exact: true })).toHaveValue(empty.id);
      await signOut(page);
    });

    await test.step("the reader sees the chips and no request", async () => {
      await signIn(page, reader, "/?cluster=" + empty.id);
      await expect(page.getByRole("heading", { name: en.home.title, level: 1 })).toBeVisible();
      await expect(chip(page, empty.id)).toHaveAttribute("aria-current", "true");
      await expect(page.getByText(en.home.answer.none, { exact: true })).toBeVisible();
      await expect(page.getByRole("link", { name: en.home.answer.request, exact: true })).toHaveCount(0);
      await expect(page.getByRole("link", { name: en.nav.generate, exact: true })).toHaveCount(0);
      await expectNoViolations(page);
    });
  } finally {
    if (made.profile) {
      await page.context().clearCookies();
      await signIn(page, admin, "/account");
      await expect(page.getByRole("heading", { name: en.account.title, level: 1 })).toBeVisible();
      await deactivate(page, empty.id);
    }
  }
});
