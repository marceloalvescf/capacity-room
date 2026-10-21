import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";
import type { components } from "@/api/schema.gen";
import { i18next } from "@/i18n/i18n";
import { installMockApi, principal, report, signedIn } from "../../../tests/msw";
import { location, renderApp } from "../../../tests/render";
import { mockColourScheme } from "../../../tests/match-media";
import viewEn from "../../../tests/fixtures/view/direct-complete.en.json";
import viewPt from "../../../tests/fixtures/view/direct-complete.pt-BR.json";

const mock = installMockApi(signedIn());

type Role = components["schemas"]["Role"];
type View = components["schemas"]["ReportView"];
type Cluster = components["schemas"]["Home"]["clusters"][number];
type Summary = NonNullable<Cluster["latest"]>;

const t = (key: string, options: Record<string, string | number> = {}): string => i18next.t(key, options);
const english = viewEn as View;
const portuguese = viewPt as View;

const prodId = "11111111-0000-4000-8000-000000000001";
const paymentsId = "22222222-0000-4000-8000-000000000002";
const jobId = "33333333-0000-4000-8000-000000000003";

function summary(over: Partial<Summary>): Summary {
  return { ...report(), ...over };
}

function cluster(profile: string, name: string, latest: Summary | null, newest: Summary | null = latest): Cluster {
  return { profile_id: profile, display_name: name, latest, newest };
}

const prod = summary({ id: prodId, profile_id: "prod", worst_severity: "healthy", cluster_name: "prod-1" });
const payments = summary({ id: paymentsId, profile_id: "payments", worst_severity: "critical", cluster_name: "payments-eu" });

function useViews(...ids: string[]) {
  for (const id of ids) {
    mock.views.set(`${id}:en`, english);
    mock.views.set(`${id}:pt-BR`, portuguese);
  }
}

function signIn(role: Role) {
  mock.session = signedIn({ role });
}

beforeEach(() => {
  window.localStorage.clear();
  mockColourScheme(false);
  mock.home = {
    clusters: [cluster("edge", "Edge", null), cluster("prod", "Production", prod), cluster("payments", "Payments", payments)],
  };
  useViews(prodId, paymentsId);
});

function chips(): HTMLElement[] {
  const list = screen.getByRole("list", { name: t("home.strip.heading") });
  return within(list).getAllByRole("link");
}

async function open(path = "/") {
  const app = await renderApp(path);
  await screen.findByRole("heading", { level: 1, name: t("home.title") });
  await screen.findAllByRole("link", { name: /payments-eu|prod-1|Edge/ });
  return app;
}

describe("home page", () => {
  it("orders the chips worst first and chooses the first", async () => {
    await open();
    const links = chips();
    expect(links.map((link) => link.getAttribute("href"))).toEqual([
      "/?cluster=payments",
      "/?cluster=prod",
      "/?cluster=edge",
    ]);
    expect(links[0]).toHaveAttribute("aria-current", "true");
    expect(links[1]).not.toHaveAttribute("aria-current");
    expect(await screen.findByRole("heading", { level: 2, name: "Payments" })).toBeInTheDocument();
    expect(links[0]).toHaveTextContent("payments-eu");
    expect(within(links[0] as HTMLElement).getByText("payments-eu")).toHaveClass("font-mono");
    expect(links[2]).toHaveTextContent("Edge");
    expect(links[2]).toHaveTextContent(t("home.chip.none"));
  });

  it("chooses the cluster in the address", async () => {
    await open("/?cluster=prod");
    expect(chips()[1]).toHaveAttribute("aria-current", "true");
    expect(await screen.findByRole("heading", { level: 2, name: "Production" })).toBeInTheDocument();
    await waitFor(() => {
      expect(mock.calls.some((call) => call.path === `/reports/${prodId}/view`)).toBe(true);
    });
    expect(mock.calls.some((call) => call.path === `/reports/${paymentsId}/view`)).toBe(false);
  });

  it("writes the chosen cluster with replace", async () => {
    const user = userEvent.setup();
    const { router } = await open();
    await user.click(chips()[1] as HTMLElement);
    expect(location(router)).toBe("/?cluster=prod");
    await act(async () => {
      await router.navigate(-1);
    });
    expect(location(router)).not.toBe("/");
  });

  it("falls back to the first chip for an unknown cluster", async () => {
    await open("/?cluster=nowhere");
    expect(chips()[0]).toHaveAttribute("aria-current", "true");
    expect(await screen.findByRole("heading", { level: 2, name: "Payments" })).toBeInTheDocument();
  });

  it("links the answer into the report", async () => {
    await open();
    expect(await screen.findByRole("link", { name: t("home.answer.open") })).toHaveAttribute(
      "href",
      `/reports/${paymentsId}/overview`,
    );
    const severity = (await screen.findAllByRole("link", { name: new RegExp(t("status.critical")) })).filter(
      (link) => link.getAttribute("href") === `/reports/${paymentsId}/findings?severity=critical`,
    );
    expect(severity).toHaveLength(1);
    expect(screen.getAllByRole("row")).toHaveLength(4);
    expect(screen.getByRole("link", { name: t("home.plan.all") })).toHaveAttribute("href", `/reports/${paymentsId}/overview`);
  });

  it("names the report's namespaces", async () => {
    mock.views.set(`${paymentsId}:en`, { ...english, report: { ...english.report, namespaces: ["billing", "payments"] } });
    await open();
    const meta = (await screen.findByText(english.report.period_text)).parentElement as HTMLElement;
    expect(within(meta).getByText("billing, payments")).toHaveClass("font-mono");
  });

  it("says all namespaces for an empty list", async () => {
    mock.views.set(`${paymentsId}:en`, { ...english, report: { ...english.report, namespaces: [] } });
    await open();
    expect(await screen.findByText(t("home.meta.namespaces"))).toBeInTheDocument();
  });

  it("says a request is collecting, with Follow", async () => {
    const running = summary({ id: jobId, profile_id: "payments", status: "running", phase: "collecting", worst_severity: null, cluster_name: null, completed_at: null });
    mock.home = { clusters: [cluster("payments", "Payments", payments, running)] };
    await open();
    const notice = await screen.findByText(/A new report is collecting, requested/);
    expect(notice.parentElement).toContainElement(screen.getByRole("link", { name: t("home.flight.follow") }));
    expect(screen.getByRole("link", { name: t("home.flight.follow") })).toHaveAttribute("href", `/jobs/${jobId}`);
    expect(chips()[0]).toHaveTextContent(t("home.chip.step.collecting"));
    expect(await screen.findByRole("link", { name: t("home.answer.open") })).toHaveAttribute("href", `/reports/${paymentsId}/overview`);
  });

  it("says the newest request failed, above no report", async () => {
    const failed = summary({
      id: jobId,
      profile_id: "edge",
      status: "failed",
      phase: "complete",
      failure_code: "INTERNAL_FAILURE",
      worst_severity: null,
      cluster_name: null,
    });
    mock.home = { clusters: [cluster("edge", "Edge", null, failed)] };
    await open();
    const reason = t("reportFailure.INTERNAL_FAILURE").replace(/\.$/, "");
    expect(await screen.findByText(t("home.failed.notice", { reason }))).toBeInTheDocument();
    expect(screen.getByRole("link", { name: t("home.failed.open") })).toHaveAttribute("href", `/jobs/${jobId}`);
    expect(screen.getByText(t("home.answer.none"))).toBeInTheDocument();
    expect(chips()[0]).toHaveTextContent(t("reportStatus.failed"));
  });

  it("says only an expired report has expired", async () => {
    const old = summary({ id: jobId, profile_id: "edge", expired: true });
    mock.home = { clusters: [cluster("edge", "Edge", null, old)] };
    await open();
    expect(await screen.findByText(t("home.answer.expired"))).toBeInTheDocument();
    expect(chips()[0]).toHaveTextContent(t("reports.expired"));
    expect(screen.getByRole("link", { name: t("home.answer.request") })).toHaveAttribute("href", "/generate?profile=edge");
  });

  it("offers a report request only to an operator", async () => {
    mock.home = { clusters: [cluster("edge", "Edge", null), cluster("payments", "Payments", payments)] };
    signIn("reader");
    const reader = await open("/?cluster=edge");
    expect(await screen.findByText(t("home.answer.none"))).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: t("nav.generate") })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: t("home.answer.request") })).not.toBeInTheDocument();
    reader.unmount();

    signIn("operator");
    await open("/?cluster=edge");
    expect(await screen.findByRole("link", { name: t("home.answer.request") })).toHaveAttribute("href", "/generate?profile=edge");
    const newReport = screen.getAllByRole("link", { name: t("nav.generate") }).find((link) => link.getAttribute("href")?.includes("profile="));
    expect(newReport).toHaveAttribute("href", "/generate?profile=edge");
  });

  it("repeats the latest request through from", async () => {
    await open();
    const links = screen.getAllByRole("link", { name: t("nav.generate") });
    expect(links.map((link) => link.getAttribute("href"))).toContain(`/generate?from=${paymentsId}`);
  });

  it("says the report is gone on a 410 and fetches home again", async () => {
    mock.viewFailure.set(`${paymentsId}:en`, { status: 410, code: "REPORT_EXPIRED" });
    await open();
    expect(await screen.findByText(t("home.answer.gone"))).toBeInTheDocument();
    await waitFor(() => {
      expect(mock.calls.filter((call) => call.path === "/home")).toHaveLength(2);
    });
  });

  it("shows the empty state to an administrator with a link to /profiles/new", async () => {
    mock.home = { clusters: [] };
    await renderApp("/");
    expect(await screen.findByText(t("home.empty.title"))).toBeInTheDocument();
    expect(screen.getByRole("link", { name: t("home.empty.admin") })).toHaveAttribute("href", "/profiles/new");
    expect(screen.getByRole("link", { name: t("nav.generate") })).toHaveAttribute("href", "/generate");
  });

  it("shows the empty state to a reader without the link", async () => {
    mock.home = { clusters: [] };
    signIn("reader");
    await renderApp("/");
    expect(await screen.findByText(t("home.empty.title"))).toBeInTheDocument();
    expect(screen.getByText(t("home.empty.other"))).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: t("home.empty.admin") })).not.toBeInTheDocument();
  });

  it("draws the unseen dot with its accessible phrase", async () => {
    window.localStorage.setItem(`capacityroom-home-seen:${principal().id}`, JSON.stringify({ prod: "older", payments: paymentsId }));
    await open();
    await waitFor(() => {
      expect(chips()[1]).toHaveAccessibleName(new RegExp(t("home.chip.unseen")));
    });
    expect(chips()[0]).not.toHaveAccessibleName(new RegExp(t("home.chip.unseen")));
  });

  it("draws no dot on a first visit, and none once the answer is shown", async () => {
    await open("/?cluster=prod");
    await screen.findByRole("heading", { level: 2, name: "Production" });
    expect(chips().some((chip) => chip.textContent.includes(t("home.chip.unseen")))).toBe(false);
    expect(JSON.parse(window.localStorage.getItem(`capacityroom-home-seen:${principal().id}`) ?? "{}")).toMatchObject({ prod: prodId });
  });

  it("takes the dot away once the answer of that report is shown", async () => {
    window.localStorage.setItem(`capacityroom-home-seen:${principal().id}`, JSON.stringify({ prod: "older" }));
    await open("/?cluster=prod");
    await screen.findByRole("heading", { level: 2, name: "Production" });
    await waitFor(() => {
      expect(chips()[1]).not.toHaveAccessibleName(new RegExp(t("home.chip.unseen")));
    });
    expect(JSON.parse(window.localStorage.getItem(`capacityroom-home-seen:${principal().id}`) ?? "{}")).toMatchObject({ prod: prodId });
  });

  it("keeps the strip when the language changes", async () => {
    await open();
    await screen.findByRole("link", { name: t("home.answer.open") });
    mock.calls.length = 0;
    await act(async () => {
      await i18next.changeLanguage("pt-BR");
    });
    await waitFor(() => {
      expect(mock.calls.some((call) => call.path === `/reports/${paymentsId}/view` && call.search.includes("lang=pt-BR"))).toBe(true);
    });
    expect(screen.getByRole("list", { name: "Clusters, pelo relatório mais recente" })).toBeInTheDocument();
    expect(chips()).toHaveLength(3);
  });
});
