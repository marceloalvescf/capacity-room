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
import directComplete from "../../../tests/fixtures/view/direct-complete.en.json";
import directCompletePt from "../../../tests/fixtures/view/direct-complete.pt-BR.json";

const mock = installMockApi(signedIn());

type View = components["schemas"]["ReportView"];
type Gauge = NonNullable<View["verdict"]["gauge"]>;
type Pressure = View["verdict"]["pressure"][number];

const id = "0b6c3b52-2f0e-4d0a-8d57-6f6f6c1f0a01";
const english = viewEn as View;
const portuguese = viewPt as View;

const count = (value: number) => ({ raw: value, text: String(value), status: null });

// bound gives a golden view the autoscaling of a report stored before the
// program read node autoscaling from kube-state-metrics: enabled, with the
// bounds of the cluster autoscaler, three nodes of a maximum of three, and a
// finding of near maximum at the place of the cluster finding.
function bound(view: View, sentence: string): View {
  const at = view.verdict.cluster_finding as number;
  const stored = { ...(view.findings[at] as View["findings"][number]), code: "NODE_AUTOSCALING_NEAR_MAX", severity: "attention" as const, sentence };
  return {
    ...view,
    summary: {
      ...view.summary,
      autoscaling: {
        ...view.summary.autoscaling,
        status: "enabled",
        managed_node_groups: count(2),
        current_nodes: count(3),
        min_nodes: count(2),
        max_nodes: count(3),
        scale_events: count(1),
        unschedulable_pods: count(2),
      },
    },
    findings: view.findings.map((finding, index) => (index === at ? stored : finding)),
    verdict: { ...view.verdict, gauge: { current: 3, max: 3, min: 2, hazard: "attention" } },
  };
}

const bounded = bound(english, "The number of nodes reached or came close to the observed maximum.");
const boundedPortuguese = bound(portuguese, "O número de nodes atingiu ou ficou próximo do máximo observado.");

beforeEach(() => {
  window.localStorage.clear();
  mockColourScheme(false);
  mock.profiles = [profileSummary()];
  mock.reports = [report()];
  mock.views.set(`${id}:en`, english);
  mock.views.set(`${id}:pt-BR`, portuguese);
});

function growth(name = "Cluster autoscaling"): HTMLElement {
  return screen.getByRole("heading", { level: 3, name }).closest("li") as HTMLElement;
}

function pairs(): HTMLElement[] {
  return within(screen.getByRole("list", { name: "Pressure by scope" })).getAllByRole("listitem");
}

async function open(language: "en" | "pt-BR" = "en") {
  const view = await renderApp(`/reports/${id}/overview`, language);
  await screen.findByRole("heading", { level: 2, name: language === "en" ? "Prioritized action plan" : "Plano de ação priorizado" });
  return view;
}

describe("overview tab", () => {
  it("growth side", async () => {
    mock.views.set(`${id}:en`, bounded);
    await open();
    const pair = growth();
    expect(pair).toHaveTextContent("3/3");
    expect(within(pair).getByText("Nodes · current / maximum")).toBeInTheDocument();
    expect(within(pair).getByText("The number of nodes reached or came close to the observed maximum.")).toBeInTheDocument();
    expect(within(pair).getByText("Attention")).toBeInTheDocument();
    expect(pair).toHaveTextContent("Minimum nodes 2 · Unschedulable pods 2 · Nodes added 1");
    const values = pair.querySelectorAll("span.font-mono");
    expect(values).toHaveLength(3);
    for (const value of values) {
      expect(value).not.toHaveClass("italic");
    }
    const link = within(pair).getByRole("link", { name: "See autoscaling" });
    expect(link.getAttribute("href")).toMatch(/autoscaling$/);
  });

  it("enabled autoscaling without a maximum shows the state word and the known counts", async () => {
    mock.views.set(`${id}:en`, directComplete as View);
    await open();
    const pair = growth();
    expect(within(pair).getByText("Enabled")).toHaveClass("text-2xl");
    expect(pair).not.toHaveTextContent("Nodes · current / maximum");
    expect(pair.querySelector("svg[viewBox='0 0 1000 16']")).toBeNull();
    expect(pair).toHaveTextContent("Current nodes 1 · Unschedulable pods 1 · Nodes added 1");
    expect(pair).not.toHaveTextContent("Unavailable");
    expect(pair).not.toHaveTextContent("Minimum nodes");
  });

  it("enabled autoscaling with a maximum and no minimum draws the pair and the bar", async () => {
    const complete = directComplete as View;
    mock.views.set(`${id}:en`, {
      ...complete,
      summary: {
        ...complete.summary,
        autoscaling: {
          ...complete.summary.autoscaling,
          current_nodes: count(3),
          max_nodes: count(3),
          unschedulable_pods: count(1),
          scale_events: count(1),
        },
      },
      verdict: { ...complete.verdict, gauge: { current: 3, max: 3, min: null, hazard: "attention" } },
    });
    await open();
    const pair = growth();
    expect(within(pair).getByText("Nodes · current / maximum")).toBeInTheDocument();
    expect(pair).toHaveTextContent("3/3");
    expect(pair.querySelector("svg[viewBox='0 0 1000 16']")).not.toBeNull();
    expect(pair).toHaveTextContent("Unschedulable pods 1 · Nodes added 1");
    expect(pair).not.toHaveTextContent("Minimum nodes");
    expect(pair).not.toHaveTextContent("Unavailable");
  });

  it("autoscaling disabled from raw metrics shows its word and no counts", async () => {
    await open();
    const pair = growth();
    expect(within(pair).getByText("Disabled")).toHaveClass("text-2xl");
    expect(pair).not.toHaveTextContent("Current nodes");
    expect(pair).not.toHaveTextContent("Nodes added");
  });

  it("enabled autoscaling in portuguese", async () => {
    mock.session = signedIn({ language: "pt-BR" });
    mock.views.set(`${id}:pt-BR`, directCompletePt as View);
    await open("pt-BR");
    const pair = growth("Autoscaling do cluster");
    expect(within(pair).getByText("Ativo")).toBeInTheDocument();
    expect(pair).toHaveTextContent("Nodes atuais 1");
  });

  it("autoscaling that is not enabled shows its status word", async () => {
    mock.views.set(`${id}:en`, {
      ...english,
      summary: { ...english.summary, autoscaling: { ...english.summary.autoscaling, status: "unknown" } },
    });
    await open();
    const pair = growth();
    expect(within(pair).getByText("Unknown")).toHaveClass("text-2xl");
    expect(pair).not.toHaveTextContent("Nodes · current / maximum");
    expect(pair).not.toHaveTextContent("Minimum nodes");
    expect(pair.querySelector("svg[viewBox='0 0 1000 16']")).toBeNull();
    expect(within(pair).getByRole("link", { name: "See autoscaling" })).toBeInTheDocument();
  });

  it("the headings of the overview are sized as headings, not labels", async () => {
    await open();
    expect(screen.getByRole("heading", { level: 3, name: "Cluster autoscaling" })).toHaveClass("font-semibold");
    const pressure = screen.getByRole("heading", { level: 2, name: "Pressure by scope" });
    expect(pressure).toHaveClass("text-lg", "font-semibold");
    expect(pressure).not.toHaveClass("label");
  });

  it("a proof bar under the nodes, filled to the nodes in the colour of the gauge hazard", async () => {
    mock.views.set(`${id}:en`, bounded);
    await open();
    const bar = growth().querySelector("svg[viewBox='0 0 1000 16']") as SVGElement;
    const fill = bar.querySelector("[data-fill]") as SVGElement;
    expect(fill).toHaveClass("fill-attention");
    expect(fill.getAttribute("width")).toBe("1000");
    expect(fill.parentElement).toHaveClass("motion-safe:animate-proof");
    expect(bar.querySelectorAll("[data-unit]")).toHaveLength(2);
    expect(bar.querySelectorAll("[data-floor]")).toHaveLength(1);
    expect(bar.querySelector("[data-reference]")?.getAttribute("x1")).toBe("1000");
    expect(bar.querySelector("[data-overrun]")).toBeNull();
  });

  it("the fill of the gauge is the view's hazard, whatever the cluster finding", async () => {
    mock.views.set(`${id}:en`, bounded);
    const gauge = bounded.verdict.gauge as Gauge;
    expect(bounded.findings[bounded.verdict.cluster_finding as number]?.severity).toBe("attention");
    mock.views.set(`${id}:en`, { ...bounded, verdict: { ...bounded.verdict, gauge: { ...gauge, hazard: "critical" } } });
    await open();
    expect(growth().querySelector("[data-fill]")).toHaveClass("fill-critical");

    mock.views.set(`${id}:en`, { ...bounded, verdict: { ...bounded.verdict, cluster_finding: null, gauge: { ...gauge, hazard: null } } });
    cleanup();
    await open();
    const fill = growth().querySelector("[data-fill]");
    expect(fill).toHaveClass("fill-neatline");
    expect(fill).not.toHaveClass("fill-attention");
  });

  it("the bar divides the maximum into units and draws the floor only between zero and the maximum", async () => {
    mock.views.set(`${id}:en`, bounded);
    const gauge = bounded.verdict.gauge as Gauge;
    const draw = async (next: Gauge) => {
      mock.views.set(`${id}:en`, { ...bounded, verdict: { ...bounded.verdict, gauge: next } });
      cleanup();
      await open();
      return growth().querySelector("svg[viewBox='0 0 1000 16']") as SVGElement;
    };
    let bar = await draw({ ...gauge, current: 3, max: 5, min: 1 });
    expect(bar.querySelectorAll("[data-unit]")).toHaveLength(4);
    expect(bar.querySelectorAll("[data-floor]")).toHaveLength(1);
    expect(bar.querySelector("[data-fill]")?.getAttribute("width")).toBe("600");
    bar = await draw({ ...gauge, current: 3, max: 5, min: 0 });
    expect(bar.querySelector("[data-floor]")).toBeNull();
    bar = await draw({ ...gauge, current: 3, max: 5, min: 5 });
    expect(bar.querySelector("[data-floor]")).toBeNull();
  });

  it("a count past its maximum runs past the reference line", async () => {
    mock.views.set(`${id}:en`, bounded);
    const gauge = bounded.verdict.gauge as Gauge;
    mock.views.set(`${id}:en`, { ...bounded, verdict: { ...bounded.verdict, gauge: { ...gauge, current: 4, max: 3 } } });
    await open();
    const bar = growth().querySelector("svg[viewBox='0 0 1000 16']") as SVGElement;
    expect(bar.querySelectorAll("[data-reference]")).toHaveLength(1);
    expect(bar.querySelector("[data-reference]")?.getAttribute("x1")).toBe("750");
    expect(bar.querySelector("[data-fill]")?.getAttribute("width")).toBe("750");
    expect(bar.querySelector("[data-overrun]")?.getAttribute("x")).toBe("750");
    expect(bar.querySelector("[data-overrun]")?.getAttribute("width")).toBe("250");
  });

  it("no proof bar when the view gives no gauge, though autoscaling is enabled", async () => {
    mock.views.set(`${id}:en`, bounded);
    mock.views.set(`${id}:en`, { ...bounded, verdict: { ...bounded.verdict, gauge: null } });
    await open();
    const pair = growth();
    expect(pair).toHaveTextContent("Nodes · current / maximum");
    expect(pair.querySelector("svg[viewBox='0 0 1000 16']")).toBeNull();
  });

  it("the pressure follows the order of the view and the cluster sentence follows its index", async () => {
    const [first, second, ...rest] = english.verdict.pressure;
    mock.views.set(`${id}:en`, {
      ...english,
      verdict: { ...english.verdict, cluster_finding: 12, pressure: [second as Pressure, first as Pressure, ...rest] },
    });
    await open();
    expect(pairs().slice(0, 3).map((line) => line.querySelector("h3")?.textContent)).toEqual([
      "Cluster autoscaling",
      "CriticalNamespaces",
      "CriticalNodes",
    ]);
    const pair = growth();
    expect(pair).toHaveTextContent(english.findings[12]?.sentence ?? "");
    expect(pair).not.toHaveTextContent(english.findings[10]?.sentence ?? "");
  });

  it("pressure lines, the worst scopes first and the fixed order among equals", async () => {
    await open();
    const lines = pairs().slice(1);
    expect(lines.map((line) => line.querySelector("h3")?.textContent)).toEqual([
      "CriticalNodes",
      "CriticalNamespaces",
      "CriticalWorkloads",
      "CriticalHPAs",
      "AttentionNode pools",
    ]);
    const lead = (line: HTMLElement) => within(line).getAllByRole("link")[0] as HTMLElement;
    expect(lead(lines[0] as HTMLElement)).toHaveTextContent("Observed CPU usage on the node.");
    expect(lead(lines[0] as HTMLElement).getAttribute("href")).toMatch(/findings\?scope=node&finding=0$/);
    expect(lead(lines[1] as HTMLElement).getAttribute("href")).toMatch(/findings\?scope=namespace&finding=2$/);
    expect(lead(lines[2] as HTMLElement).getAttribute("href")).toMatch(/findings\?scope=workload&finding=6$/);
    expect(lead(lines[3] as HTMLElement).getAttribute("href")).toMatch(/findings\?scope=hpa&finding=8$/);
    expect(lead(lines[4] as HTMLElement).getAttribute("href")).toMatch(/findings\?scope=nodepool&finding=14$/);
    expect(within(lines[0] as HTMLElement).getByText("aks-agentpool-12345678-vmss000000")).toHaveClass("font-mono");
    expect(within(lines[0] as HTMLElement).getByText("Peak")).toBeInTheDocument();
    expect(within(lines[0] as HTMLElement).getByText("95%")).toHaveClass("font-mono");
    expect(within(lines[1] as HTMLElement).queryByText("Peak")).toBeInTheDocument();
    expect(lines[0]).toHaveTextContent(/Critical 2\s*·\s*Attention 1/);
    expect(lines[1]).toHaveTextContent(/Critical 1/);
    expect(lines[2]).toHaveTextContent(/Critical 1\s*·\s*Attention 1/);
    expect(lines[3]).toHaveTextContent(/Critical 1\s*·\s*Attention 1/);
    expect(lines[4]).toHaveTextContent(/Attention 2/);
  });

  it("a pressure pair with no proof gives its claim the whole row", async () => {
    mock.views.set(`${id}:en`, {
      ...english,
      verdict: { ...english.verdict, pressure: [{ scope_type: "nodepool", severity: "attention", finding: 20, critical: 0, attention: 1 }] },
    });
    await open();
    const only = pairs()[1] as HTMLElement;
    expect(only.children).toHaveLength(1);
    expect(only.children[0]).toHaveClass("lg:col-span-2");
    expect(pairs()[0]?.children).toHaveLength(2);
  });

  it("the counts of a pressure line are the view's, whatever the findings", async () => {
    const [first, ...rest] = english.verdict.pressure;
    mock.views.set(`${id}:en`, {
      ...english,
      verdict: { ...english.verdict, pressure: [{ ...(first as Pressure), critical: 7, attention: 4 }, ...rest] },
    });
    await open();
    const line = pairs()[1] as HTMLElement;
    expect(line).toHaveTextContent(/Critical 7\s*·\s*Attention 4/);
    expect(within(line).getByText("7")).toHaveClass("font-mono");
    expect(within(line).getByText("7")).not.toHaveClass("italic");
  });

  it("a pressure link names the node it leads to", async () => {
    await open();
    const first = within(pairs()[1] as HTMLElement).getAllByRole("link")[0];
    expect(first).toHaveAccessibleName("Observed CPU usage on the node. (aks-agentpool-12345678-vmss000000)");
  });

  it("plan links with the same action are told apart by their scope", async () => {
    await open();
    const plan = screen.getByRole("heading", { level: 2, name: "Prioritized action plan" }).parentElement as HTMLElement;
    const names = within(plan)
      .getAllByRole("link")
      .map((link) => link.getAttribute("aria-label"));
    expect(names).toContain("Review the memory request and the observed consumption. (payments)");
    expect(names).toContain("Review the memory request and the observed consumption. (payments/payments-web)");
  });

  it("the node evidence warning stands under the verdict of the growth side", async () => {
    await open();
    expect(within(growth()).getByText("The capacity evidence of the nodes is incomplete.").parentElement?.querySelector("svg")).toHaveClass("text-attention");
  });

  it("no node evidence warning when the view holds none", async () => {
    mock.views.set(`${id}:en`, directComplete as View);
    await open();
    expect(growth()).not.toHaveTextContent("The capacity evidence of the nodes is incomplete.");
  });

  it("two workloads of one name and two kinds count as two rows", async () => {
    const deployment = english.findings.find((finding) => finding.scope_type === "workload" && finding.severity === "critical") as View["findings"][number];
    const statefulSet = { ...deployment, scope_kind: "StatefulSet", severity: "attention" as const };
    mock.views.set(`${id}:en`, {
      ...english,
      findings: [deployment, statefulSet],
      verdict: {
        cluster_finding: null,
        node_evidence_warning: null,
        gauge: null,
        pressure: [{ scope_type: "workload", severity: "critical", finding: 0, critical: 1, attention: 1 }],
      },
    });
    await open();
    const lines = pairs();
    expect(lines).toHaveLength(2);
    expect(lines[1]).toHaveTextContent(/Critical 1\s*·\s*Attention 1/);
  });

  it("an unknown node pool shows its word", async () => {
    mock.views.set(`${id}:en`, {
      ...english,
      verdict: { ...english.verdict, pressure: [{ scope_type: "nodepool", severity: "attention", finding: 20, critical: 0, attention: 1 }] },
    });
    await open();
    const only = screen.getByText("Node pools").closest("li") as HTMLElement;
    expect(within(only).getByText("Undetermined")).not.toHaveClass("font-mono");
    expect(only).not.toHaveTextContent(/\bunknown\b/);
  });

  it("an unknown node pool shows its word in portuguese", async () => {
    mock.session = signedIn({ language: "pt-BR" });
    mock.views.set(`${id}:pt-BR`, {
      ...portuguese,
      verdict: { ...portuguese.verdict, pressure: [{ scope_type: "nodepool", severity: "attention", finding: 20, critical: 0, attention: 1 }] },
    });
    await open("pt-BR");
    const only = screen.getByText("Node pools").closest("li") as HTMLElement;
    expect(within(only).getByText("Indeterminado")).not.toHaveClass("font-mono");
  });

  it("nothing needs attention", async () => {
    mock.views.set(`${id}:en`, {
      ...english,
      verdict: { ...english.verdict, cluster_finding: null, pressure: [] },
    });
    await open();
    expect(screen.getByText("Nothing needs attention.")).toBeInTheDocument();
    expect(pairs()).toHaveLength(1);
    expect(pairs()[0]).toBe(growth());
  });

  it("the answer strip: the three totals above the band, each a link with its shape in colour", async () => {
    await open();
    const totals = screen.getByRole("group", { name: "Findings by severity" });
    const band = screen.getByRole("heading", { level: 3, name: "Cluster autoscaling" });
    expect(totals.compareDocumentPosition(band) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    const links = within(totals).getAllByRole("link");
    expect(links.map((link) => link.textContent)).toEqual(["10Critical", "11Attention", "7Healthy"]);
    expect(links.map((link) => link.getAttribute("href"))).toEqual([
      expect.stringMatching(/findings\?severity=critical$/),
      expect.stringMatching(/findings\?severity=attention$/),
      expect.stringMatching(/findings\?severity=healthy$/),
    ]);
    for (const link of links) {
      expect(link).toHaveClass("text-fg");
    }
    expect(links.map((link) => link.querySelector("svg")?.getAttribute("class"))).toEqual([
      expect.stringContaining("text-critical"),
      expect.stringContaining("text-attention"),
      expect.stringContaining("text-healthy"),
    ]);
    expect(within(links[0] as HTMLElement).getByText("10")).toHaveClass("font-mono", "text-display", "font-semibold");
    expect(within(links[0] as HTMLElement).getByText("10")).not.toHaveClass("italic");
    expect(within(links[0] as HTMLElement).getByText("Critical")).toHaveClass("text-sm", "text-muted-fg");
    expect(screen.getByRole("link", { name: "See all findings" }).getAttribute("href")).toMatch(/\/findings$/);
  });

  it("the worst severity present is not coloured in the strip", async () => {
    mock.views.set(`${id}:en`, { ...english, summary: { ...english.summary, critical: { raw: 0, text: "0", status: null } } });
    await open();
    const links = within(screen.getByRole("group", { name: "Findings by severity" })).getAllByRole("link");
    expect(links[0]?.querySelector("svg")?.getAttribute("class")).toContain("text-critical");
    expect(links[1]).toHaveClass("text-fg");
    expect(links[1]).not.toHaveClass("text-attention");
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
    expect(healthy).toHaveTextContent("7Healthy");
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
      verdict: { cluster_finding: null, node_evidence_warning: null, gauge: null, pressure: [] },
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
    expect(within(notice).getByText("2")).toHaveClass("font-mono");
    expect(within(notice).getByText("2")).not.toHaveClass("italic");
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
    expect(within(notice).getByText("+2").closest("ul")).toHaveClass("text-muted-fg");
  });

  it("portuguese words", async () => {
    mock.views.set(`${id}:pt-BR`, boundedPortuguese);
    mock.session = signedIn({ language: "pt-BR" });
    await open("pt-BR");
    expect(screen.getByRole("heading", { level: 2, name: "Pressão por escopo" })).toBeInTheDocument();
    expect(screen.getByText("Nodes · atual / máximo")).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Achados por severidade" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Ver todos os achados" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Ver qualidade dos dados" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Qualidade dos dados, avisos: 2" })).toBeInTheDocument();
  });

  it("explains the confidence under the action plan", async () => {
    await open();
    const table = screen.getByRole("table", { name: "Prioritized action plan" });
    const note = screen.getByText(
      "Confidence: exact, read from the metrics; inferred, deduced from approximate or partial evidence; unknown, not established by the metrics.",
    );
    expect(table.compareDocumentPosition(note) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
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
    expect(cells.map((cell) => cell.textContent)).toEqual(["1", "Critical", english.recommendations[0]?.action, "HPA payments/web", "Exact"]);
    expect(screen.getByRole("heading", { level: 2, name: "Prioritized action plan" })).toBeInTheDocument();
  });

  it("the scope of a plan row is its kind, muted, then its full name in mono", async () => {
    await open();
    const rows = within(screen.getByRole("table", { name: "Prioritized action plan" })).getAllByRole("row").slice(1);
    const hpa = within(rows[0] as HTMLElement).getAllByRole("cell")[3] as HTMLElement;
    expect(within(hpa).getByText("HPA")).toHaveClass("text-muted-fg");
    expect(within(hpa).getByText("payments/web")).toHaveClass("font-mono");
    const workload = english.recommendations.findIndex((row) => row.scope_type === "workload");
    const cell = within(rows[workload] as HTMLElement).getAllByRole("cell")[3] as HTMLElement;
    expect(cell).toHaveTextContent("Workload payments/payments-web");
  });

  it("a scope of more than three is its summary alone", async () => {
    const [first, ...rest] = english.recommendations;
    const many = { ...(first as View["recommendations"][number]), scopes: ["ns/a", "ns/b", "ns/c", "ns/d"], scope_text: "4 workloads identified", scope_type: "workload" as const };
    mock.views.set(`${id}:en`, { ...english, recommendations: [many, ...rest] });
    await open();
    const rows = within(screen.getByRole("table", { name: "Prioritized action plan" })).getAllByRole("row").slice(1);
    const cell = within(rows[0] as HTMLElement).getAllByRole("cell")[3] as HTMLElement;
    expect(cell.textContent).toBe("4 workloads identified");
  });

  it("an action of the plan links to its finding", async () => {
    await open();
    const rows = within(screen.getByRole("table", { name: "Prioritized action plan" })).getAllByRole("row").slice(1);
    const first = within(rows[0] as HTMLElement).getByRole("link", { name: "Review the configured maximum and the capacity of the workload. (payments/web)" });
    expect(first.getAttribute("href")).toMatch(/\/findings\?scope=hpa&finding=8$/);
    const eighth = within(rows[7] as HTMLElement).getByRole("link");
    expect(eighth.getAttribute("href")).toMatch(/\/findings\?scope=workload&finding=11$/);
  });

  it("an action links to the finding the view names, whatever its code", async () => {
    const [first, ...rest] = english.recommendations;
    expect(first?.finding).toBe(8);
    expect(english.findings[3]?.code).not.toBe(english.findings[8]?.code);
    mock.views.set(`${id}:en`, { ...english, recommendations: [{ ...(first as View["recommendations"][number]), finding: 3 }, ...rest] });
    await open();
    const rows = within(screen.getByRole("table", { name: "Prioritized action plan" })).getAllByRole("row").slice(1);
    expect(within(rows[0] as HTMLElement).getByRole("link").getAttribute("href")).toMatch(/[?&]finding=3$/);
  });

  it("an action the view ties to no finding stays plain text", async () => {
    const [first, ...rest] = english.recommendations;
    mock.views.set(`${id}:en`, { ...english, recommendations: [{ ...(first as View["recommendations"][number]), finding: null }, ...rest] });
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
