import { act, cleanup, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";
import type { components } from "@/api/schema.gen";
import { i18next } from "@/i18n/i18n";
import { installMockApi, profileSummary, report, signedIn } from "../../../tests/msw";
import { location, renderApp } from "../../../tests/render";
import { mockColourScheme } from "../../../tests/match-media";
import viewEn from "../../../tests/fixtures/view/edge-scaling.en.json";
import viewPt from "../../../tests/fixtures/view/edge-scaling.pt-BR.json";

const mock = installMockApi(signedIn());

type View = components["schemas"]["ReportView"];

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

async function open(language: "en" | "pt-BR" = "en") {
  const view = await renderApp(`/reports/${id}/overview`, language);
  await screen.findByRole("heading", { level: 2, name: language === "en" ? "Prioritized action plan" : "Plano de ação priorizado" });
  return view;
}

describe("overview tab", () => {
  it("growth side", async () => {
    await open();
    const band = screen.getByRole("heading", { level: 2, name: "Cluster autoscaling" }).parentElement as HTMLElement;
    expect(band).toHaveTextContent("3/3");
    expect(within(band).getByText("Nodes · current / maximum")).toBeInTheDocument();
    expect(within(band).getByText("The number of nodes is close to the observed maximum.")).toBeInTheDocument();
    expect(within(band).getByText("Attention")).toBeInTheDocument();
    expect(band).toHaveTextContent("Minimum nodes 2 · Unschedulable pods 2 · Scale events 1");
    const link = within(band).getByRole("link", { name: "See autoscaling" });
    expect(link.getAttribute("href")).toMatch(/autoscaling$/);
  });

  it("autoscaling that is not enabled shows its status word", async () => {
    mock.views.set(`${id}:en`, {
      ...english,
      summary: { ...english.summary, autoscaling: { ...english.summary.autoscaling, status: "unknown" } },
    });
    await open();
    const band = screen.getByRole("heading", { level: 2, name: "Cluster autoscaling" }).parentElement as HTMLElement;
    expect(within(band).getByText("Unknown")).toHaveClass("text-2xl");
    expect(band).not.toHaveTextContent("Nodes · current / maximum");
    expect(band).not.toHaveTextContent("Minimum nodes");
    expect(band.querySelector("svg[aria-hidden=true] [data-band]")).toBeNull();
    expect(within(band).getByRole("link", { name: "See autoscaling" })).toBeInTheDocument();
  });

  it("a depth gauge under the nodes, hatched at the maximum in the colour of the cluster finding", async () => {
    await open();
    const heading = screen.getByRole("heading", { level: 2, name: "Cluster autoscaling" });
    expect(heading).toHaveClass("label");
    expect(screen.getByRole("heading", { level: 2, name: "Pressure by scope" })).toHaveClass("label");
    const band = heading.parentElement as HTMLElement;
    expect(band.querySelectorAll("[data-band]")).toHaveLength(3);
    expect(band.querySelectorAll("[data-hazard]")).toHaveLength(1);
    expect(band.querySelector("pattern line")).toHaveClass("stroke-attention");
    expect(band.querySelector("[data-contour]")).not.toBeNull();
  });

  it("a full scale with no cluster finding is not hatched", async () => {
    mock.views.set(`${id}:en`, {
      ...english,
      findings: english.findings.filter((finding) => finding.scope_type !== "cluster"),
    });
    await open();
    const band = screen.getByRole("heading", { level: 2, name: "Cluster autoscaling" }).parentElement as HTMLElement;
    expect(band.querySelectorAll("[data-band]")).toHaveLength(3);
    expect(band.querySelector("[data-hazard]")).toBeNull();
    expect(band.querySelector("[data-closing]")).toHaveClass("stroke-neatline");
  });

  it("no depth gauge without a maximum", async () => {
    mock.views.set(`${id}:en`, {
      ...english,
      summary: {
        ...english.summary,
        autoscaling: { ...english.summary.autoscaling, max_nodes: { raw: null, text: "Unavailable", status: null } },
      },
    });
    await open();
    const band = screen.getByRole("heading", { level: 2, name: "Cluster autoscaling" }).parentElement as HTMLElement;
    expect(band).toHaveTextContent("Nodes · current / maximum");
    expect(band.querySelector("[data-band]")).toBeNull();
  });

  it("pressure lines", async () => {
    await open();
    const list = screen.getByRole("heading", { level: 2, name: "Pressure by scope" }).parentElement?.querySelector("ul") as HTMLElement;
    const lines = within(list).getAllByRole("listitem");
    expect(lines.map((line) => line.querySelector("span.inline-flex")?.textContent)).toEqual([
      "CriticalNodes",
      "AttentionNode pools",
      "CriticalNamespaces",
      "CriticalWorkloads",
      "CriticalHPAs",
    ]);
    const lead = (line: HTMLElement) => within(line).getAllByRole("link")[0] as HTMLElement;
    expect(lead(lines[0] as HTMLElement)).toHaveTextContent("Observed CPU usage on the node.");
    expect(lead(lines[0] as HTMLElement).getAttribute("href")).toMatch(/findings\?scope=node&finding=0$/);
    expect(lead(lines[1] as HTMLElement).getAttribute("href")).toMatch(/findings\?scope=nodepool&finding=14$/);
    expect(lead(lines[2] as HTMLElement).getAttribute("href")).toMatch(/findings\?scope=namespace&finding=1$/);
    expect(lead(lines[3] as HTMLElement).getAttribute("href")).toMatch(/findings\?scope=workload&finding=5$/);
    expect(lead(lines[4] as HTMLElement).getAttribute("href")).toMatch(/findings\?scope=hpa&finding=7$/);
    expect(within(lines[0] as HTMLElement).getByText("aks-agentpool-12345678-vmss000000")).toHaveClass("font-mono");
    expect(within(lines[0] as HTMLElement).getByText("Peak")).toBeInTheDocument();
    expect(within(lines[0] as HTMLElement).getByText("95%")).toHaveClass("font-mono");
    expect(within(lines[2] as HTMLElement).queryByText("Peak")).toBeInTheDocument();
    expect(lines[0]).toHaveTextContent(/Critical 2$/);
    expect(lines[1]).toHaveTextContent(/Attention 2$/);
    expect(lines[2]).toHaveTextContent(/Critical 1$/);
    expect(lines[3]).toHaveTextContent(/Critical 1\s*·\s*Attention 1$/);
    expect(lines[4]).toHaveTextContent(/Critical 1\s*·\s*Attention 1$/);
  });

  it("two workloads of one name and two kinds count as two rows", async () => {
    const deployment = english.findings.find((finding) => finding.scope_type === "workload" && finding.severity === "critical") as View["findings"][number];
    const statefulSet = { ...deployment, scope_kind: "StatefulSet", severity: "attention" as const };
    mock.views.set(`${id}:en`, { ...english, findings: [deployment, statefulSet] });
    await open();
    const list = screen.getByRole("heading", { level: 2, name: "Pressure by scope" }).parentElement?.querySelector("ul") as HTMLElement;
    const lines = within(list).getAllByRole("listitem");
    expect(lines).toHaveLength(1);
    expect(lines[0]).toHaveTextContent(/Critical 1\s*·\s*Attention 1$/);
  });

  it("an unknown node pool shows its word", async () => {
    mock.views.set(`${id}:en`, {
      ...english,
      findings: english.findings.filter((finding) => finding.scope_type !== "nodepool" || finding.scope_name === "unknown"),
    });
    await open();
    const only = screen.getByText("Node pools").closest("li") as HTMLElement;
    expect(within(only).getByText("Unknown pool")).not.toHaveClass("font-mono");
    expect(only).not.toHaveTextContent(/\bunknown\b/);
  });

  it("nothing needs attention", async () => {
    mock.views.set(`${id}:en`, {
      ...english,
      findings: english.findings.filter((finding) => finding.severity === "healthy"),
    });
    await open();
    expect(screen.getByText("Nothing needs attention.")).toBeInTheDocument();
    const heading = screen.getByRole("heading", { level: 2, name: "Pressure by scope" });
    expect(heading.parentElement?.querySelector("ul")).toBeNull();
  });

  it("the answer strip: the three totals above the band, the worst one in its colour, each a link", async () => {
    await open();
    const totals = screen.getByRole("group", { name: "Findings by severity" });
    const band = screen.getByRole("heading", { level: 2, name: "Cluster autoscaling" });
    expect(totals.compareDocumentPosition(band) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    const links = within(totals).getAllByRole("link");
    expect(links.map((link) => link.textContent)).toEqual(["Critical9", "Attention10", "Healthy7"]);
    expect(links.map((link) => link.getAttribute("href"))).toEqual([
      expect.stringMatching(/findings\?severity=critical$/),
      expect.stringMatching(/findings\?severity=attention$/),
      expect.stringMatching(/findings\?severity=healthy$/),
    ]);
    expect(links[0]).toHaveClass("text-critical", "font-semibold");
    expect(links[1]).toHaveClass("text-fg");
    expect(links[1]).not.toHaveClass("font-semibold");
    expect(within(links[0] as HTMLElement).getByText("9")).toHaveClass("font-mono", "italic");
    expect(totals).toHaveClass("text-xl");
    expect(screen.getByRole("link", { name: "See all findings" }).getAttribute("href")).toMatch(/\/findings$/);
  });

  it("the worst severity present leads the strip", async () => {
    mock.views.set(`${id}:en`, { ...english, summary: { ...english.summary, critical: { raw: 0, text: "0", status: null } } });
    await open();
    const links = within(screen.getByRole("group", { name: "Findings by severity" })).getAllByRole("link");
    expect(links[0]).not.toHaveClass("font-semibold");
    expect(links[1]).toHaveClass("text-attention", "font-semibold");
  });

  it("a total below the minimum severity keeps its count and no link", async () => {
    mock.views.set(`${id}:en`, { ...english, min_severity: "attention" });
    await open();
    let totals = screen.getByRole("group", { name: "Findings by severity" });
    expect(within(totals).getAllByRole("link").map((link) => link.getAttribute("href"))).toEqual([
      expect.stringMatching(/findings\?severity=critical$/),
      expect.stringMatching(/findings\?severity=attention$/),
    ]);
    const healthy = within(totals).getByText("Healthy").parentElement as HTMLElement;
    expect(healthy).toHaveTextContent("Healthy7");
    expect(healthy.closest("a")).toBeNull();
    expect(within(healthy).getByText("7")).toHaveClass("font-mono");

    mock.views.set(`${id}:en`, { ...english, min_severity: "critical" });
    cleanup();
    await open();
    totals = screen.getByRole("group", { name: "Findings by severity" });
    expect(within(totals).getAllByRole("link")).toHaveLength(1);
    expect((within(totals).getByText("Attention").parentElement as HTMLElement).closest("a")).toBeNull();
  });

  it("a report with no finding says so in the strip", async () => {
    const none = { raw: 0, text: "0", status: null };
    mock.views.set(`${id}:en`, {
      ...english,
      findings: [],
      recommendations: [],
      summary: { ...english.summary, critical: none, attention: none, healthy: none },
    });
    await open();
    expect(screen.getByText("This report holds no findings.")).toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "Findings by severity" })).toBeNull();
  });

  it("a data quality notice under the strip names each warning", async () => {
    await open();
    const notice = screen.getByRole("group", { name: "Data quality" });
    expect(notice).toHaveTextContent(/^Data quality2/);
    expect(within(notice).getByText("2")).toHaveClass("font-mono", "italic");
    for (const warning of english.quality.warnings) {
      expect(within(notice).getByText(warning.text)).toBeInTheDocument();
    }
    expect(within(notice).getByRole("link", { name: "See data quality" }).getAttribute("href")).toMatch(/\/quality$/);
  });

  it("no data quality notice without a warning", async () => {
    mock.views.set(`${id}:en`, { ...english, quality: { ...english.quality, warnings: [] } });
    await open();
    expect(screen.queryByRole("group", { name: "Data quality" })).toBeNull();
  });

  it("the notice shows three warnings and counts the rest", async () => {
    const warnings = ["a", "b", "c", "d", "e"].map((letter) => ({ code: `W_${letter}`, text: `Warning ${letter}.` }));
    mock.views.set(`${id}:en`, { ...english, quality: { ...english.quality, warnings } });
    await open();
    const notice = screen.getByRole("group", { name: "Data quality" });
    expect(within(notice).getByText("Warning c.")).toBeInTheDocument();
    expect(within(notice).queryByText("Warning d.")).toBeNull();
    expect(within(notice).getByText("+2")).toHaveClass("text-muted-fg");
  });

  it("portuguese words", async () => {
    mock.session = signedIn({ language: "pt-BR" });
    await open("pt-BR");
    expect(screen.getByRole("heading", { level: 2, name: "Pressão por escopo" })).toBeInTheDocument();
    expect(screen.getByText("Nodes · atual / máximo")).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Achados por severidade" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Ver todos os achados" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Ver qualidade dos dados" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Qualidade dos dados, avisos: 2" })).toBeInTheDocument();
  });

  it("action plan", async () => {
    await open();
    const table = screen.getByRole("table", { name: "Prioritized action plan" });
    const rows = within(table).getAllByRole("row").slice(1);
    expect(rows).toHaveLength(english.recommendations.length);
    english.recommendations.forEach((recommendation, index) => {
      expect(within(rows[index] as HTMLElement).getByText(recommendation.action)).toBeInTheDocument();
    });
    const cells = within(rows[0] as HTMLElement).getAllByRole("cell");
    expect(cells.map((cell) => cell.textContent)).toEqual(["1", "Critical", english.recommendations[0]?.action, "web", "Exact"]);
    expect(screen.getByRole("heading", { level: 2, name: "Prioritized action plan" })).toBeInTheDocument();
  });

  it("an action of the plan links to its finding", async () => {
    await open();
    const rows = within(screen.getByRole("table", { name: "Prioritized action plan" })).getAllByRole("row").slice(1);
    const first = within(rows[0] as HTMLElement).getByRole("link", { name: english.recommendations[0]?.action ?? "" });
    expect(first.getAttribute("href")).toMatch(/\/findings\?scope=hpa&finding=7$/);
    const eighth = within(rows[7] as HTMLElement).getByRole("link");
    expect(eighth.getAttribute("href")).toMatch(/\/findings\?scope=workload&finding=11$/);
  });

  it("an action with no matching finding stays plain text", async () => {
    const [first, ...rest] = english.recommendations;
    mock.views.set(`${id}:en`, { ...english, recommendations: [{ ...(first as View["recommendations"][number]), scopes: ["payments/nothing"] }, ...rest] });
    await open();
    const rows = within(screen.getByRole("table", { name: "Prioritized action plan" })).getAllByRole("row").slice(1);
    expect(within(rows[0] as HTMLElement).queryByRole("link")).toBeNull();
    expect(rows[0]).toHaveTextContent(first?.action ?? "");
  });

  it("overview empty plan", async () => {
    mock.views.set(`${id}:en`, { ...english, recommendations: [] });
    await open();
    expect(screen.getByText("No prioritized action was identified.")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("about folded", async () => {
    await open();
    const summary = screen.getByText("About this report");
    const details = summary.closest("details") as HTMLDetailsElement;
    expect(details.open).toBe(false);
    await userEvent.click(summary);
    expect(details.open).toBe(true);
    expect(details).toHaveTextContent("America/Sao_Paulo");
    expect(details).toHaveTextContent("payments");
    expect(details).toHaveTextContent("All");
    expect(details).toHaveTextContent(english.report.emitted_at_text);
    expect(details).toHaveTextContent("0.0.0-golden");
    expect(within(details).getByText("America/Sao_Paulo")).toHaveClass("font-mono");
    expect(within(details).getByText(english.report.emitted_at_text)).toHaveClass("font-mono");
  });

  it("language switch keeps the place", async () => {
    const { router } = await open();
    await act(async () => {
      await i18next.changeLanguage("pt-BR");
    });
    expect(await screen.findByRole("heading", { level: 2, name: "Plano de ação priorizado" })).toBeInTheDocument();
    expect(location(router)).toBe(`/reports/${id}/overview`);
    const rows = within(screen.getByRole("table")).getAllByRole("row").slice(1);
    portuguese.recommendations.forEach((recommendation, index) => {
      expect(within(rows[index] as HTMLElement).getByText(recommendation.action)).toBeInTheDocument();
    });
  });
});
