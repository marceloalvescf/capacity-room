import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { beforeEach, describe, expect, it } from "vitest";
import { findBannedTerms } from "@/i18n/vocabulary";
import { i18next } from "@/i18n/i18n";
import type { Language } from "@/i18n/language";
import { adminProfile, adminUser, anonymous, installMockApi, profileSummary, report, signedIn } from "./msw";
import { location, renderApp } from "./render";
import { mockColourScheme } from "./match-media";
import { findShown } from "./shown";
import viewEn from "./fixtures/view/edge-scaling.en.json";
import viewPt from "./fixtures/view/edge-scaling.pt-BR.json";
import type { components } from "@/api/schema.gen";

const mock = installMockApi(signedIn());

beforeEach(() => {
  window.localStorage.clear();
  mockColourScheme(false);
  mock.session = signedIn();
});

const languages: Language[] = ["en", "pt-BR"];
const screens = [
  { name: "sign-in", path: "/login", signedIn: false, heading: "login.title" },
  { name: "reports", path: "/reports", signedIn: true, heading: "reports.title" },
  { name: "new report", path: "/generate", signedIn: true, heading: "generate.title" },
  {
    name: "progress",
    path: "/jobs/0b6c3b52-2f0e-4d0a-8d57-6f6f6c1f0a01",
    signedIn: true,
    heading: "progress.title",
    options: { profile: "Production" },
    title: "progress.screen",
  },
  { name: "account", path: "/account", signedIn: true, heading: "account.title" },
  { name: "not found", path: "/nothing", signedIn: true, heading: "notFound.title" },
] as const;

function readableText(): string {
  const names = [...document.body.querySelectorAll("[aria-label]")].map((node) => node.getAttribute("aria-label") ?? "");
  return [document.body.textContent, ...names].join(" ");
}

describe("screens", () => {
  for (const screenCase of screens) {
    for (const language of languages) {
      for (const theme of ["light", "dark"]) {
        it(`axe and vocabulary: ${screenCase.name}, ${language}, ${theme}`, async () => {
          document.documentElement.setAttribute("data-theme", theme);
          const options = "options" in screenCase ? screenCase.options : {};
          const title = "title" in screenCase ? screenCase.title : screenCase.heading;
          mock.reports = [
            report({ pdfs: [{ language: "en", status: "ready", failure_code: null }] }),
            report({ id: "0b6c3b52-2f0e-4d0a-8d57-6f6f6c1f0a02", status: "running", phase: "collecting" }),
          ];
          mock.profiles = [profileSummary()];
          mock.session = screenCase.signedIn ? signedIn({ language }) : anonymous();
          const { container } = await renderApp(screenCase.path, language);
          await screen.findByRole("heading", { level: 1, name: i18next.t(screenCase.heading, options) });
          if (screenCase.name === "reports") {
            await screen.findByRole("table");
          }
          const result = await axe.run(container, { rules: { "color-contrast": { enabled: false } } });
          expect(result.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
          expect(findBannedTerms(language === "en" ? "en" : "pt-BR", readableText())).toEqual([]);
          expect(document.title).toBe(i18next.t("app.title", { screen: i18next.t(title) }));
        });
      }
    }
  }

  for (const language of languages) {
    for (const theme of ["light", "dark"]) {
      it(`axe and vocabulary: the report overview, ${language}, ${theme}`, async () => {
        document.documentElement.setAttribute("data-theme", theme);
        const id = "0b6c3b52-2f0e-4d0a-8d57-6f6f6c1f0a01";
        mock.reports = [report()];
        mock.profiles = [profileSummary()];
        mock.views.set(`${id}:en`, viewEn as components["schemas"]["ReportView"]);
        mock.views.set(`${id}:pt-BR`, viewPt as components["schemas"]["ReportView"]);
        mock.session = signedIn({ language });
        const { container } = await renderApp(`/reports/${id}/overview`, language);
        await screen.findByRole("heading", { level: 1, name: i18next.t("progress.title", { profile: "Production" }) });
        await screen.findByRole("table");
        const result = await axe.run(container, { rules: { "color-contrast": { enabled: false } } });
        expect(result.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
        expect(findBannedTerms(language === "en" ? "en" : "pt-BR", readableText())).toEqual([]);
      });
    }
  }

  for (const language of languages) {
    for (const theme of ["light", "dark"]) {
      it(`axe and vocabulary: the profiles with a row opened and a test result, ${language}, ${theme}`, async () => {
        document.documentElement.setAttribute("data-theme", theme);
        mock.adminProfiles = [adminProfile({ credential_file: "production-token", allowed_namespaces: ["payments"] })];
        mock.session = signedIn({ language });
        const { container } = await renderApp("/profiles?open=production", language);
        await screen.findByRole("heading", { level: 1, name: i18next.t("admin.profiles.title") });
        await userEvent.setup().click(await screen.findByRole("button", { name: i18next.t("admin.profiles.test") }));
        await findShown(i18next.t("profileTest.PROFILE_OK"));
        const result = await axe.run(container, { rules: { "color-contrast": { enabled: false } } });
        expect(result.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
        expect(findBannedTerms(language === "en" ? "en" : "pt-BR", readableText())).toEqual([]);
        expect(document.title).toBe(i18next.t("app.title", { screen: i18next.t("admin.profiles.title") }));
      });
    }
  }

  for (const language of languages) {
    for (const theme of ["light", "dark"]) {
      for (const path of ["/profiles/new", "/profiles/production/edit"]) {
        it(`axe and vocabulary: ${path} with both folded sections open, ${language}, ${theme}`, async () => {
          document.documentElement.setAttribute("data-theme", theme);
          mock.adminProfiles = [adminProfile({ credential_file: "production-token", allowed_namespaces: ["payments"] })];
          mock.session = signedIn({ language });
          const { container } = await renderApp(path, language);
          const user = userEvent.setup();
          for (const section of ["choices", "thresholds"]) {
            const title = i18next.t(`admin.profile.section.${section}`);
            await user.click(await screen.findByRole("button", { name: `${i18next.t("admin.profile.change")} ${title}` }));
            await screen.findByRole("button", { name: `${i18next.t("admin.profile.fold")} ${title}` });
          }
          const result = await axe.run(container, { rules: { "color-contrast": { enabled: false } } });
          expect(result.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
          expect(findBannedTerms(language === "en" ? "en" : "pt-BR", readableText())).toEqual([]);
          expect(document.title).toBe(
            i18next.t("app.title", {
              screen: i18next.t(path === "/profiles/new" ? "admin.profile.newTitle" : "admin.profile.editScreen"),
            }),
          );
        });
      }
    }
  }

  for (const language of languages) {
    for (const theme of ["light", "dark"]) {
      it(`axe and vocabulary: the users with a row opened, ${language}, ${theme}`, async () => {
        document.documentElement.setAttribute("data-theme", theme);
        const other = adminUser();
        mock.adminUsers = [adminUser({ id: signedIn().principal.id, username: "ana", role: "administrator" }), other];
        mock.session = signedIn({ language });
        const { container } = await renderApp(`/users?open=${other.id}`, language);
        await screen.findByRole("heading", { level: 1, name: i18next.t("admin.users.title") });
        await screen.findByRole("button", { name: i18next.t("admin.users.save") });
        const result = await axe.run(container, { rules: { "color-contrast": { enabled: false } } });
        expect(result.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
        expect(findBannedTerms(language === "en" ? "en" : "pt-BR", readableText())).toEqual([]);
        expect(document.title).toBe(i18next.t("app.title", { screen: i18next.t("admin.users.title") }));
      });
    }
  }

  for (const tab of [
    { name: "namespaces", open: "payments" },
    { name: "workloads", open: "payments%2FDeployment%2Fpayments-web" },
    { name: "autoscaling", open: "payments%2Fweb" },
    { name: "nodes", open: "aks-agentpool-12345678-vmss000000" },
    { name: "quality", open: "hpa_history", ready: "report.quality.metrics" },
  ]) {
    for (const language of languages) {
      for (const theme of ["light", "dark"]) {
        it(`axe and vocabulary: the report ${tab.name} with a row opened, ${language}, ${theme}`, async () => {
          document.documentElement.setAttribute("data-theme", theme);
          const id = "0b6c3b52-2f0e-4d0a-8d57-6f6f6c1f0a01";
          mock.reports = [report()];
          mock.profiles = [profileSummary()];
          mock.views.set(`${id}:en`, viewEn as components["schemas"]["ReportView"]);
          mock.views.set(`${id}:pt-BR`, viewPt as components["schemas"]["ReportView"]);
          mock.session = signedIn({ language });
          const { container } = await renderApp(`/reports/${id}/${tab.name}?open=${tab.open}`, language);
          await screen.findByRole("heading", { level: 1, name: i18next.t("progress.title", { profile: "Production" }) });
          if (tab.ready === undefined) {
            await screen.findByRole("heading", { level: 2, name: i18next.t("report.tab.findings") });
          } else {
            await screen.findByText(i18next.t(tab.ready));
          }
          const result = await axe.run(container, { rules: { "color-contrast": { enabled: false } } });
          expect(result.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
          expect(findBannedTerms(language === "en" ? "en" : "pt-BR", readableText())).toEqual([]);
        });
      }
    }
  }

  for (const language of languages) {
    for (const theme of ["light", "dark"]) {
      it(`axe and vocabulary: the report findings with a finding selected, ${language}, ${theme}`, async () => {
        document.documentElement.setAttribute("data-theme", theme);
        const id = "0b6c3b52-2f0e-4d0a-8d57-6f6f6c1f0a01";
        mock.reports = [report()];
        mock.profiles = [profileSummary()];
        mock.views.set(`${id}:en`, viewEn as components["schemas"]["ReportView"]);
        mock.views.set(`${id}:pt-BR`, viewPt as components["schemas"]["ReportView"]);
        mock.session = signedIn({ language });
        const { container } = await renderApp(`/reports/${id}/findings?finding=2`, language);
        await screen.findByRole("heading", { level: 1, name: i18next.t("progress.title", { profile: "Production" }) });
        await screen.findByRole("article");
        const result = await axe.run(container, { rules: { "color-contrast": { enabled: false } } });
        expect(result.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
        expect(findBannedTerms(language === "en" ? "en" : "pt-BR", readableText())).toEqual([]);
      });
    }
  }

  for (const language of languages) {
    for (const theme of ["light", "dark"]) {
      it(`axe and vocabulary: the period picker open on Range with a calendar, ${language}, ${theme}`, async () => {
        document.documentElement.setAttribute("data-theme", theme);
        mock.profiles = [profileSummary()];
        mock.session = signedIn({ language });
        await renderApp("/generate", language);
        const user = userEvent.setup();
        await user.click(await screen.findByRole("button", { name: i18next.t("generate.change") }));
        await user.click(screen.getByRole("button", { name: new RegExp(i18next.t("generate.period")) }));
        await user.click(await screen.findByRole("radio", { name: i18next.t("period.mode.range") }));
        await user.click(screen.getByRole("button", { name: i18next.t("period.range.pickFrom") }));
        await screen.findByRole("grid");
        const result = await axe.run(document.body, { rules: { "color-contrast": { enabled: false } } });
        expect(result.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
        expect(findBannedTerms(language === "en" ? "en" : "pt-BR", readableText())).toEqual([]);
      });
    }
  }

  async function expectClean(language: Language) {
    const result = await axe.run(document.body, { rules: { "color-contrast": { enabled: false } } });
    expect(result.violations.map((v) => `${v.id}: ${v.help}`)).toEqual([]);
    expect(findBannedTerms(language === "en" ? "en" : "pt-BR", readableText())).toEqual([]);
  }

  const states: { name: string; run: (language: Language) => Promise<void> }[] = [
    {
      name: "the profile form after a refusal on a field",
      run: async (language) => {
        mock.adminProfiles = [adminProfile({ credential_file: "production-token", allowed_namespaces: ["payments"] })];
        mock.failures.set("PATCH /admin/profiles/production", {
          status: 422,
          code: "PROFILE_DOCUMENT_INVALID",
          fields: { default_timezone: "invalid", nope: "invalid" },
        });
        await renderApp("/profiles/production/edit", language);
        const user = userEvent.setup();
        await user.click(await screen.findByRole("button", { name: i18next.t("admin.profile.save") }));
        await screen.findByRole("alert");
        await waitFor(() => {
          expect(document.querySelector("[aria-invalid=true]")).not.toBeNull();
        });
      },
    },
    {
      name: "the profile form with the stale alert",
      run: async (language) => {
        mock.adminProfiles = [adminProfile({ credential_file: "production-token", allowed_namespaces: ["payments"] })];
        mock.failures.set("PATCH /admin/profiles/production", { status: 409, code: "PROFILE_REVISION_STALE" });
        await renderApp("/profiles/production/edit", language);
        const user = userEvent.setup();
        await user.type(await screen.findByLabelText(i18next.t("admin.profile.name")), "x");
        await user.click(screen.getByRole("button", { name: i18next.t("admin.profile.save") }));
        await screen.findByRole("button", { name: i18next.t("admin.profile.loadLatest") });
      },
    },
    {
      name: "the deactivate dialog",
      run: async (language) => {
        const other = adminUser();
        mock.adminUsers = [adminUser({ id: signedIn().principal.id, username: "ana", role: "administrator" }), other];
        await renderApp(`/users?open=${other.id}`, language);
        await userEvent.setup().click(await screen.findByRole("button", { name: i18next.t("admin.users.deactivate") }));
        await screen.findByRole("alertdialog");
      },
    },
    {
      name: "the profile delete dialog",
      run: async (language) => {
        mock.adminProfiles = [adminProfile({ credential_file: "production-token", allowed_namespaces: ["payments"] })];
        await renderApp("/profiles?open=production", language);
        await userEvent.setup().click(await screen.findByRole("button", { name: i18next.t("admin.profiles.delete") }));
        await screen.findByRole("alertdialog");
      },
    },
    {
      name: "the new user dialog with field errors",
      run: async (language) => {
        mock.adminUsers = [adminUser()];
        await renderApp("/users", language);
        const user = userEvent.setup();
        await user.click(await screen.findByRole("button", { name: i18next.t("admin.users.new") }));
        await user.click(await screen.findByRole("button", { name: i18next.t("admin.users.create") }));
        await waitFor(() => {
          expect(document.querySelectorAll("[aria-invalid=true]").length).toBeGreaterThan(0);
        });
      },
    },
  ];
  for (const state of states) {
    for (const language of languages) {
      for (const theme of ["light", "dark"]) {
        it(`axe and vocabulary: ${state.name}, ${language}, ${theme}`, async () => {
          document.documentElement.setAttribute("data-theme", theme);
          mock.session = signedIn({ language });
          await state.run(language);
          await expectClean(language);
        });
      }
    }
  }

  it("a notice holds no live-region role beside the shell's region", async () => {
    mock.session = anonymous();
    await renderApp("/login?changed=1&ended=1");
    await screen.findByText(i18next.t("login.passwordChanged"));
    expect(screen.getByText(i18next.t("login.sessionEnded"))).toBeInTheDocument();
    expect(screen.queryAllByRole("status")).toHaveLength(1);
    expect(document.querySelectorAll("[aria-live='polite']")).toHaveLength(1);
  });

  it("routes: anonymous account opens sign-in with next", async () => {
    mock.session = anonymous();
    const { router } = await renderApp("/account");
    await waitFor(() => {
      expect(location(router)).toBe("/login?next=%2Faccount");
    });
  });

  it("routes: anonymous start page opens sign-in", async () => {
    mock.session = anonymous();
    const { router } = await renderApp("/");
    await waitFor(() => {
      expect(location(router)).toBe("/login");
    });
  });

  it("routes: signed-in sign-in and start page lead to the reports", async () => {
    for (const path of ["/login", "/"]) {
      const { router, unmount } = await renderApp(path);
      await waitFor(() => {
        expect(location(router)).toBe("/reports");
      });
      unmount();
    }
  });

  it("routes: an unknown path shows not found inside the shell", async () => {
    await renderApp("/nothing");
    expect(await screen.findByRole("heading", { name: i18next.t("notFound.title") })).toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: i18next.t("nav.label") })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: i18next.t("notFound.home") })).toHaveAttribute("href", "/");
  });

  it("the shell holds the skip link, one live region, and the sidebar name", async () => {
    await renderApp("/account");
    expect(await screen.findByRole("link", { name: i18next.t("app.skip") })).toHaveAttribute("href", "#main");
    expect(document.querySelectorAll("[aria-live='polite']")).toHaveLength(1);
    expect(screen.getByText("ana")).toHaveClass("font-mono");
    expect(document.getElementById("main")).toHaveAttribute("tabindex", "-1");
  });
});
