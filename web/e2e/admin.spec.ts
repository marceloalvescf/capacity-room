import en from "../src/locales/en.json" with { type: "json" };
import { admin, buttonName, createProfile, createUser, deleteProfile, expect, expectFits, expectNoViolations, fitSizes, lastRow, secret, signIn, test, watchPolicy } from "./fixtures";

const newUser = { username: "e2e-admin-reader", password: secret("e2e-Admin-Reader-Pass-05!") };
const profile = {
  id: "e2e-lab",
  name: "Lab",
  cluster: "lab",
  refused: "http://169.254.169.254/api/v1",
  unreachable: "http://fake-prometheus:9091/api/v1",
};

// A profile whose endpoint and namespaces are as long as a profile gets in practice, so that its opened row is the
// tallest and the widest of the list. It is inactive and goes at the end of the test.
const wide = {
  id: "e2e-wide",
  endpoint: `http://fake-prometheus:9090/${Array.from({ length: 12 }, (_, n) => `long-path-segment-${String(n).padStart(2, "0")}`).join("/")}/api/v1`,
  namespaces: Array.from({ length: 40 }, (_, n) => `wide-namespace-${String(n).padStart(2, "0")}-abcdefghij`),
};

const fill = buttonName;

test("an administrator manages users and profiles", async ({ page, browser, baseURL }) => {
  const navigation = page.getByRole("navigation", { name: en.nav.label });
  const folded = (section: string) => page.getByRole("button", { name: `${en.admin.profile.change} ${section}`, exact: true });

  await test.step("the administrator creates a user", async () => {
    await signIn(page, admin, "/users");
    await expect(page.getByRole("heading", { name: en.admin.users.title, level: 1 })).toBeVisible();
    await page.getByRole("button", { name: en.admin.users.new, exact: true }).click();
    const dialog = page.getByRole("dialog", { name: en.admin.users.new });
    await expect(dialog).toBeVisible();
    await dialog.getByLabel(en.admin.users.username, { exact: true }).fill(newUser.username);
    await dialog.getByLabel(en.admin.users.passwordLabel, { exact: true }).fill(newUser.password);
    await dialog.getByLabel(en.admin.users.roleLabel, { exact: true }).selectOption({ label: en.admin.users.role.reader });
    await dialog.getByLabel(en.admin.users.languageLabel, { exact: true }).selectOption({ label: en.language.en });
    await expectNoViolations(page);
    await dialog.getByRole("button", { name: en.admin.users.create, exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await expect(page).toHaveURL(/\/users\?open=/);
    const users = page.getByRole("table", { name: en.admin.users.title, exact: true });
    await expect(users.getByRole("button", { name: fill(en.table.close, newUser.username), exact: true })).toBeVisible();
    await expectNoViolations(page);
  });

  const context = await browser.newContext(baseURL === undefined ? {} : { baseURL });
  try {
    const other = await context.newPage();
    const violations = await watchPolicy(other);

    await test.step("the new reader sees no administration", async () => {
      await signIn(other, newUser, "/reports");
      const links = other.getByRole("navigation", { name: en.nav.label });
      await expect(links.getByRole("link", { name: en.nav.reports, exact: true })).toBeVisible();
      await expect(links.getByRole("link", { name: en.nav.account, exact: true })).toBeVisible();
      await expect(links.getByRole("link", { name: en.nav.profiles, exact: true })).toHaveCount(0);
      await expect(links.getByRole("link", { name: en.nav.users, exact: true })).toHaveCount(0);
      await other.goto("/users");
      await expect(other.getByRole("heading", { name: en.notFound.title, level: 1 })).toBeVisible();
      expect(violations).toEqual([]);
    });
  } finally {
    await context.close();
  }

  await test.step("a metadata address is refused under the endpoint", async () => {
    await page.goto("/profiles/new");
    await expect(page.getByRole("heading", { name: en.admin.profile.newTitle, level: 1 })).toBeVisible();
    await page.getByLabel(en.admin.profile.id, { exact: true }).fill(profile.id);
    await page.getByLabel(en.admin.profile.name, { exact: true }).fill(profile.name);
    await page.getByLabel(en.admin.profile.url, { exact: true }).fill(profile.refused);
    await page.getByLabel(en.admin.profile.clusterName, { exact: true }).fill(profile.cluster);
    await page.getByRole("button", { name: en.admin.profiles.test, exact: true }).click();
    await expect(page.locator("#main").getByText(en.admin.profile.testRefused, { exact: true })).toBeVisible();
    await expect(page.getByLabel(en.admin.profile.url, { exact: true })).toHaveAccessibleDescription(
      en.error.ENDPOINT_ADDRESS_REJECTED,
    );
  });

  await test.step("an address nothing listens at is unreachable", async () => {
    await page.getByLabel(en.admin.profile.url, { exact: true }).fill(profile.unreachable);
    await expect(page.locator("#main").getByText(en.admin.profile.testRefused, { exact: true })).toHaveCount(0);
    await page.getByRole("button", { name: en.admin.profiles.test, exact: true }).click();
    await expect(page.locator("#main").getByText(en.profileTest.ENDPOINT_UNREACHABLE, { exact: true })).toBeVisible();
    await expect(page.getByLabel(en.admin.profile.url, { exact: true })).not.toHaveAccessibleDescription(
      en.error.ENDPOINT_ADDRESS_REJECTED,
    );
  });

  await test.step("the profile is saved active and offered by the report form", async () => {
    await page.getByRole("checkbox", { name: en.admin.profile.active, exact: true }).check();
    await page.getByRole("button", { name: en.admin.profile.save, exact: true }).click();
    await expect(page).toHaveURL(new RegExp(`/profiles\\?open=${profile.id}$`));
    const profiles = page.getByRole("table", { name: en.admin.profiles.title, exact: true });
    await expect(profiles.getByRole("button", { name: fill(en.table.close, profile.id), exact: true })).toBeVisible();
    await expectNoViolations(page);

    await page.goto("/generate");
    await expect(page.getByRole("heading", { name: en.generate.title, level: 1 })).toBeVisible();
    await expect(page.getByLabel(en.generate.profile, { exact: true }).getByRole("option", { name: profile.name, exact: true })).toHaveCount(1);
  });

  await test.step("leaving an edited form asks first", async () => {
    await page.goto(`/profiles?open=${profile.id}`);
    await page.getByRole("link", { name: en.admin.profiles.edit, exact: true }).click();
    await expect(page.getByRole("heading", { name: profile.name, level: 1 })).toBeVisible();
    await folded(en.admin.profile.section.choices).click();
    await folded(en.admin.profile.section.thresholds).click();
    await expect(page.getByLabel(en.admin.profile.periods, { exact: true })).toBeVisible();
    await expect(page.getByLabel(en.admin.profile.threshold.warning_percent, { exact: true })).toBeVisible();
    await expectNoViolations(page);

    const name = page.getByLabel(en.admin.profile.name, { exact: true });
    await name.fill(`${profile.name} renamed`);
    await navigation.getByRole("link", { name: en.nav.reports, exact: true }).click();
    const leave = page.getByRole("alertdialog", { name: en.admin.profile.leaveTitle });
    await expect(leave).toBeVisible();
    await expectNoViolations(page);
    await leave.getByRole("button", { name: en.admin.profile.stay, exact: true }).click();
    await expect(leave).toHaveCount(0);
    await expect(page).toHaveURL(new RegExp(`/profiles/${profile.id}/edit$`));
    await expect(name).toHaveValue(`${profile.name} renamed`);

    await page.getByRole("link", { name: en.admin.profile.cancel, exact: true }).click();
    await expect(page).toHaveURL(/\/profiles$/);
    await expect(page.getByRole("heading", { name: en.admin.profiles.title, level: 1 })).toBeVisible();
    await expect(page.getByRole("alertdialog")).toHaveCount(0);
  });

  await test.step("the profile is deleted", async () => {
    const profiles = page.getByRole("table", { name: en.admin.profiles.title, exact: true });
    await profiles.getByRole("button", { name: fill(en.table.open, profile.id), exact: true }).click();
    await page.getByRole("button", { name: en.admin.profiles.delete, exact: true }).click();
    const dialog = page.getByRole("alertdialog", { name: fill(en.admin.profiles.deleteTitle, profile.name) });
    await expect(dialog).toBeVisible();
    await dialog.getByRole("button", { name: en.admin.profiles.deleteConfirm, exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await expect(profiles.getByRole("button", { name: fill(en.table.open, profile.id), exact: true })).toHaveCount(0);
    await expect(profiles.getByRole("button", { name: fill(en.table.close, profile.id), exact: true })).toHaveCount(0);

    await page.goto("/generate");
    await expect(page.getByRole("heading", { name: en.generate.title, level: 1 })).toBeVisible();
    await expect(page.getByLabel(en.generate.profile, { exact: true }).getByRole("option", { name: profile.name, exact: true })).toHaveCount(0);
  });

  await test.step("the last row of each list fits the window when opened", async () => {
    // Twenty more users make the users table taller than the window, so opening its last row needs the scroll.
    for (let n = 1; n <= 20; n += 1) {
      const username = `e2e-fit-${String(n).padStart(2, "0")}`;
      await createUser(page, { username, password: secret(`${username}-Pass-06!`) }, "reader");
    }
    await createProfile(page, {
      id: wide.id,
      display_name: "Wide",
      endpoint_url: wide.endpoint,
      cluster_name: "wide",
      allowed_namespaces: wide.namespaces,
      active: false,
    });
    try {
      const lists = [
        { path: "/users", title: en.admin.users.title, tall: true },
        { path: "/profiles", title: en.admin.profiles.title, tall: false },
      ];
      for (const list of lists) {
        for (const size of fitSizes) {
          await page.setViewportSize(size);
          await page.goto(list.path);
          const table = page.getByRole("table", { name: list.title, exact: true });
          const last = await lastRow(table);
          if (list.tall) {
            await page.evaluate(() => {
              window.scrollTo(0, 0);
            });
            const box = await last.button.boundingBox();
            expect(box?.y ?? 0).toBeGreaterThan(size.height);
          } else {
            expect(last.name).toBe(wide.id);
          }
          await last.button.click();
          await expectFits(page, table, last.name);
          if (!list.tall) {
            await expect(table.getByText(wide.endpoint, { exact: true })).toBeVisible();
            await expect(table.getByText(wide.namespaces.join(", "), { exact: true })).toBeVisible();
          }
        }
      }
      await expectNoViolations(page);
    } finally {
      await deleteProfile(page, wide.id);
    }
  });
});
