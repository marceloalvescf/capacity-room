import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import axe from "axe-core";
import { beforeEach, describe, expect, it } from "vitest";
import { findBannedTerms } from "@/i18n/vocabulary";
import { i18next } from "@/i18n/i18n";
import type { Language } from "@/i18n/language";
import { anonymous, installMockApi, profileSummary, report, signedIn } from "./msw";
import { location, renderApp } from "./render";
import { mockColourScheme } from "./match-media";
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

  for (const tab of [
    { name: "namespaces", open: "payments" },
    { name: "workloads", open: "payments%2FDeployment%2Fpayments-web" },
    { name: "autoscaling", open: "payments%2Fweb" },
    { name: "nodes", open: "aks-agentpool-12345678-vmss000000" },
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
          await screen.findByRole("heading", { level: 2, name: i18next.t("report.tab.findings") });
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
