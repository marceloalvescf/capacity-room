import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, it } from "vitest";
import type { components } from "@/api/schema.gen";
import { i18next } from "@/i18n/i18n";
import { failure, installMockApi, profileSummary, report, server, signedIn } from "../../../tests/msw";
import { location, renderApp } from "../../../tests/render";
import { mockColourScheme } from "../../../tests/match-media";
import viewEn from "../../../tests/fixtures/view/edge-scaling.en.json";
import viewPt from "../../../tests/fixtures/view/edge-scaling.pt-BR.json";
import { reportTabs } from "./tabs";

const mock = installMockApi(signedIn());

type Role = components["schemas"]["Role"];
type View = components["schemas"]["ReportView"];

const t = (key: string, options: Record<string, string | number> = {}): string => i18next.t(key, options);
const id = "0b6c3b52-2f0e-4d0a-8d57-6f6f6c1f0a01";
const english = viewEn as View;
const portuguese = viewPt as View;

beforeEach(() => {
  window.localStorage.clear();
  mockColourScheme(false);
  mock.profiles = [profileSummary()];
  mock.reports = [report()];
  mock.views.set(`${id}:en`, english);
  mock.views.set(`${id}:pt-BR`, portuguese);
});

async function open(path = `/reports/${id}/overview`, language: "en" | "pt-BR" = "en") {
  const view = await renderApp(path, language);
  await screen.findByRole("heading", { level: 1, name: i18next.t("progress.title", { profile: "Production", lng: language }) });
  return view;
}

describe("report screen", () => {
  it("header facts", async () => {
    await open();
    const heading = screen.getByRole("heading", { level: 1 });
    const facts = heading.nextElementSibling as HTMLElement;
    expect(heading).toHaveTextContent("Report on Production");
    expect(within(facts).getByText("prod")).toHaveClass("font-mono");
    expect(facts).toHaveTextContent(t("report.cluster"));
    expect(facts).toHaveTextContent(english.report.period_text);
    expect(facts).toHaveTextContent(t("reportProfile.detailed"));
    expect(facts).toHaveTextContent(/Requested \d{4}-\d{2}-\d{2} \d{2}:\d{2} by ana/);
    expect(facts).not.toHaveTextContent("America/Sao_Paulo");
  });

  it("seven tabs", async () => {
    await open();
    const nav = screen.getByRole("navigation", { name: t("report.tabs") });
    const links = within(nav).getAllByRole("link");
    expect(links.map((link) => link.textContent)).toEqual(reportTabs.map((tab) => t(`report.tab.${tab}`)));
    links.forEach((link, index) => {
      expect(link.getAttribute("href")).toMatch(new RegExp(`/${reportTabs[index] ?? ""}$`));
    });
    expect(links[0]).toHaveAttribute("aria-current", "page");
    expect(links.slice(1).some((link) => link.hasAttribute("aria-current"))).toBe(false);
  });

  it("index opens the overview", async () => {
    const { router } = await open(`/reports/${id}`);
    expect(location(router)).toBe(`/reports/${id}/overview`);
  });

  it("unknown tab", async () => {
    await open(`/reports/${id}/nope`);
    expect(screen.getByRole("heading", { level: 1, name: t("notFound.title") })).toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: t("report.tabs") })).toBeInTheDocument();
  });

  it("pdf button", async () => {
    for (const role of ["operator", "reader"] as Role[]) {
      mock.session = signedIn({ role });
      mock.reports = [report({ pdfs: [{ language: "pt-BR", status: "ready", failure_code: null }] })];
      const { unmount } = await open();
      await userEvent.click(screen.getByRole("button", { name: t("progress.pdf") }));
      const groups = within(await screen.findByRole("dialog", { name: t("progress.pdf") })).getAllByRole("group");
      expect(groups.map((group) => group.getAttribute("aria-labelledby"))).toEqual(["pdf-en", "pdf-pt-BR"]);
      const [first, second] = groups as [HTMLElement, HTMLElement];
      expect(within(second).getByRole("link", { name: t("progress.download") })).toBeInTheDocument();
      expect(within(first).queryByRole("button", { name: t("progress.generatePdf") }) !== null).toBe(role === "operator");
      unmount();
    }
  });

  it("states", async () => {
    mock.viewFailure.set(`${id}:en`, { status: 409, code: "SNAPSHOT_NOT_READY" });
    mock.reports = [report({ status: "running", phase: "collecting", completed_at: null, collected_at: null })];
    const { router, unmount } = await renderApp("/reports");
    await router.navigate(`/reports/${id}/overview`);
    await screen.findByRole("heading", { level: 1, name: t("progress.title", { profile: "Production" }) });
    expect(location(router)).toBe(`/jobs/${id}`);
    // The redirect replaced the entry of the view, so going back reaches the list.
    await router.navigate(-1);
    expect(location(router)).toBe("/reports");
    unmount();

    mock.viewFailure.set(`${id}:en`, { status: 410, code: "REPORT_EXPIRED" });
    const expired = await renderApp(`/reports/${id}/overview`);
    await screen.findByText(t("progress.expired"));
    expired.unmount();

    mock.viewFailure.set(`${id}:en`, { status: 404, code: "REPORT_NOT_FOUND" });
    const missing = await renderApp(`/reports/${id}/overview`);
    await screen.findByText(t("progress.notFound"));
    missing.unmount();

    mock.viewFailure.set(`${id}:en`, { status: 500, code: "INTERNAL_ERROR" });
    await renderApp(`/reports/${id}/overview`);
    await screen.findByText(t("error.INTERNAL_ERROR"));
    expect(screen.getByRole("link", { name: t("progress.back") })).toHaveAttribute("href", "/reports");
  });

  it("language", async () => {
    mock.session = signedIn({ language: "pt-BR" });
    await open(`/reports/${id}/overview`, "pt-BR");
    expect(mock.calls.some((call) => call.path === `/reports/${id}/view` && call.search === "?lang=pt-BR")).toBe(true);
    const nav = screen.getByRole("navigation", { name: t("report.tabs") });
    expect(within(nav).getByRole("link", { name: "Visão geral" })).toBeInTheDocument();
    expect(within(nav).getByRole("link", { name: "Qualidade dos dados" })).toBeInTheDocument();
  });

  it("keeps the frame while the view of a new language loads", async () => {
    await open();
    let release: () => void = () => undefined;
    const late = new Promise<void>((resolve) => {
      release = resolve;
    });
    server.use(
      http.get(`${window.location.origin}/api/v1/reports/:id/view`, async () => {
        await late;
        return HttpResponse.json(portuguese);
      }),
    );
    await act(async () => {
      await i18next.changeLanguage("pt-BR");
    });
    expect(screen.getByRole("navigation", { name: "Seções do relatório" })).toBeInTheDocument();
    expect(screen.getAllByRole("heading", { level: 2 }).length).toBeGreaterThan(0);
    release();
    const nav = await screen.findByRole("navigation", { name: "Seções do relatório" });
    expect(await within(nav).findByRole("link", { name: "Qualidade dos dados" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "Plano de ação priorizado" })).toBeInTheDocument();
    expect(screen.getByText(portuguese.report.period_text)).toBeInTheDocument();
  });

  it("does not keep the view of another report while one loads", async () => {
    const other = "9c1c1f6a-7f0a-4c11-8d57-6f6f6c1f0a02";
    const second = { ...english, report: { ...english.report, period_text: "the second period" } };
    mock.reports = [report(), report({ id: other })];
    const { router } = await open();
    expect(screen.getByText(english.report.period_text)).toBeInTheDocument();
    let release: () => void = () => undefined;
    const late = new Promise<void>((resolve) => {
      release = resolve;
    });
    server.use(
      http.get(`${window.location.origin}/api/v1/reports/:id/view`, async () => {
        await late;
        return HttpResponse.json(second);
      }),
    );
    await act(async () => {
      await router.navigate(`/reports/${other}/overview`);
    });
    // The report itself has answered; only its view is late.
    await waitFor(() => {
      expect(mock.calls.some((call) => call.path === `/reports/${other}`)).toBe(true);
    });
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 50));
    });
    expect(screen.queryByText(english.report.period_text)).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1, name: t("progress.screen") })).toBeInTheDocument();
    release();
    expect(await screen.findByText("the second period")).toBeInTheDocument();
  });

  it("does not refetch the view when a PDF is requested", async () => {
    await open();
    const views = () => mock.calls.filter((call) => call.path === `/reports/${id}/view`).length;
    expect(views()).toBe(1);
    await userEvent.click(screen.getByRole("button", { name: t("progress.pdf") }));
    const dialog = await screen.findByRole("dialog", { name: t("progress.pdf") });
    await userEvent.click(within(dialog).getAllByRole("button", { name: t("progress.generatePdf") })[0] as HTMLElement);
    await waitFor(() => {
      expect(mock.calls.some((call) => call.method === "POST" && call.path === `/reports/${id}/pdfs`)).toBe(true);
    });
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 50));
    });
    expect(views()).toBe(1);
  });

  it("keeps the tabs when the view fails on a later fetch", async () => {
    const { queryClient } = await open();
    server.use(http.get(`${window.location.origin}/api/v1/reports/:id/view`, () => failure(500, "INTERNAL_ERROR")));
    await act(async () => {
      await queryClient.invalidateQueries();
    });
    expect(screen.getByRole("navigation", { name: t("report.tabs") })).toBeInTheDocument();
    expect(screen.queryByText(t("error.INTERNAL_ERROR"))).not.toBeInTheDocument();
  });

  it("shows the title alone until the report answers, then the failure of the view", async () => {
    mock.viewFailure.set(`${id}:en`, { status: 500, code: "INTERNAL_ERROR" });
    let release: () => void = () => undefined;
    const late = new Promise<void>((resolve) => {
      release = resolve;
    });
    server.use(
      http.get(`${window.location.origin}/api/v1/reports/:id`, async () => {
        await late;
        return HttpResponse.json(report());
      }),
    );
    await renderApp(`/reports/${id}/overview`);
    await waitFor(() => {
      expect(mock.calls.some((call) => call.path === `/reports/${id}/view`)).toBe(true);
    });
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 50));
    });
    expect(screen.getByRole("heading", { level: 1, name: t("progress.screen") })).toBeInTheDocument();
    expect(screen.queryByText(t("error.INTERNAL_ERROR"))).not.toBeInTheDocument();
    release();
    expect(await screen.findByText(t("error.INTERNAL_ERROR"))).toBeInTheDocument();
  });

  it("a gone report wins over a view that answered", async () => {
    mock.reports = [];
    await renderApp(`/reports/${id}/overview`);
    expect(await screen.findByText(t("progress.notFound"))).toBeInTheDocument();
  });

  it("keeps the tabs when the report fails on a later fetch", async () => {
    const { queryClient } = await open();
    server.use(http.get(`${window.location.origin}/api/v1/reports/:id`, () => failure(500, "INTERNAL_ERROR")));
    await act(async () => {
      await queryClient.invalidateQueries();
    });
    expect(screen.getByRole("navigation", { name: t("report.tabs") })).toBeInTheDocument();
    expect(screen.queryByText(t("error.INTERNAL_ERROR"))).not.toBeInTheDocument();
  });

  it("a report that goes missing on a later fetch takes its screen", async () => {
    const { queryClient } = await open();
    server.use(http.get(`${window.location.origin}/api/v1/reports/:id`, () => failure(404, "REPORT_NOT_FOUND")));
    await act(async () => {
      await queryClient.invalidateQueries();
    });
    expect(await screen.findByText(t("progress.notFound"))).toBeInTheDocument();
  });
});
