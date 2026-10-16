import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { components } from "@/api/schema.gen";
import { i18next } from "@/i18n/i18n";
import { installMockApi, profileSummary, report, signedIn } from "../../../tests/msw";
import { location, renderApp } from "../../../tests/render";
import { mockColourScheme } from "../../../tests/match-media";
import viewEn from "../../../tests/fixtures/view/edge-scaling.en.json";
import directComplete from "../../../tests/fixtures/view/direct-complete.en.json";
import viewPt from "../../../tests/fixtures/view/edge-scaling.pt-BR.json";

const mock = installMockApi(signedIn());

type View = components["schemas"]["ReportView"];

const id = "0b6c3b52-2f0e-4d0a-8d57-6f6f6c1f0a01";
const english = viewEn as View;
const portuguese = viewPt as View;
const firstNode = "aks-agentpool-12345678-vmss000000";

beforeEach(() => {
  window.localStorage.clear();
  mockColourScheme(false);
  mock.profiles = [profileSummary()];
  mock.reports = [report()];
  mock.views.set(`${id}:en`, english);
  mock.views.set(`${id}:pt-BR`, portuguese);
});

async function open(tab: "autoscaling" | "nodes", search = "") {
  const view = await renderApp(`/reports/${id}/${tab}${search}`, "en");
  await screen.findByRole("searchbox", { name: i18next.t(tab === "nodes" ? "report.search.nodes" : "report.search.hpas") });
  return view;
}

function rowOf(table: string, name: string): HTMLElement {
  const rows = within(screen.getByRole("table", { name: table })).getAllByRole("row");
  const found = rows.find((row) => within(row).queryAllByText(name, { selector: ".font-mono" }).length > 0);
  if (found === undefined) {
    throw new Error(`no row for ${name}`);
  }
  return found;
}

describe("autoscaling tab", () => {
  it("cluster line and counts", async () => {
    await open("autoscaling");
    expect(screen.getByRole("heading", { name: "Cluster autoscaling" })).toBeInTheDocument();
    expect(screen.getByText("Disabled")).toBeInTheDocument();
    expect(screen.getByText("Source: Raw metrics")).toBeInTheDocument();
    expect(screen.getByText("Confidence: Inferred. Deduced from approximate or partial evidence.")).toBeInTheDocument();
    const counts = screen.getByText("Current nodes", { selector: "dt" }).closest("dl");
    const list = within(counts as HTMLElement);
    const names = list.getAllByRole("term").map((term) => term.textContent);
    expect(names).toEqual([
      "Current nodes",
      "Minimum nodes",
      "Maximum nodes",
      "Managed node pools",
      "Scale events",
      "Unschedulable pods",
      "Workloads without HPA",
    ]);
    expect(list.getAllByRole("definition").map((value) => value.textContent)).toEqual(["4", "Unavailable", "Unavailable", "Unavailable", "2", "2", "2"]);
  });

  it("hpa rows", async () => {
    await open("autoscaling");
    const table = screen.getByRole("table", { name: "HPAs" });
    expect(within(table).getAllByRole("row")).toHaveLength(5);
    const web = within(rowOf("HPAs", "web")).getAllByRole("cell");
    expect(web[1]).toHaveTextContent("Deployment payments-web · payments");
    expect(web[3]).toHaveTextContent("Maximum reached");
    expect(within(web[3] as HTMLElement).getByText("Maximum reached").closest("span.inline-flex")?.querySelector("svg")).toHaveClass("text-critical");
    expect(web[4]).toHaveTextContent("2 / 4");
    expect(web[4]?.querySelector(".font-mono")).not.toBeNull();
    expect(web[4]).toHaveClass("text-right");
    expect(web[5]).toHaveTextContent("4");
    const dup = within(rowOf("HPAs", "dup")).getAllByRole("cell");
    expect(dup[4]).toHaveTextContent("1 / Unavailable");
    expect(within(dup[4] as HTMLElement).getByText("Unavailable")).toHaveClass("text-muted-fg");
  });

  it("hpa status column", async () => {
    await open("autoscaling");
    const headers = within(screen.getByRole("table", { name: "HPAs" })).getAllByRole("columnheader");
    expect(headers[2]).toHaveTextContent("Status");
    const status = (name: string) => within(rowOf("HPAs", name)).getAllByRole("cell")[2] as HTMLElement;
    expect(status("web")).toHaveTextContent("Critical");
    expect(status("idle")).toHaveTextContent("Attention");
    expect(status("fixed")).toHaveTextContent("Healthy");
    expect(status("dup")).toHaveTextContent("No finding");
    expect(status("dup").querySelector("svg")).toBeNull();
    expect(status("web").querySelector("svg")).not.toBeNull();
  });

  it("the status cell of an HPA says why under the severity, and none for no finding", async () => {
    await open("autoscaling");
    const status = (name: string) => within(rowOf("HPAs", name)).getAllByRole("cell")[2] as HTMLElement;
    expect(status("idle")).toHaveTextContent("There is a persistent average difference between current and desired replicas.");
    expect(status("web")).toHaveTextContent("The HPA reached its observed maximum of replicas.");
    expect(status("dup")).toHaveTextContent(/^No finding$/);
  });

  it("hpa default order", async () => {
    await open("autoscaling");
    const names = within(screen.getByRole("table", { name: "HPAs" }))
      .getAllByRole("row")
      .slice(1)
      .map((row) => within(row).getAllByRole("cell")[1]?.querySelector(".font-mono")?.textContent);
    expect(names).toEqual(["web", "idle", "fixed", "dup"]);
  });

  it("hpa opened", async () => {
    const user = userEvent.setup();
    const { router } = await open("autoscaling");
    await user.click(screen.getByRole("button", { name: "Show the details of web" }));
    expect(location(router)).toBe(`/reports/${id}/autoscaling?open=payments%2Fweb`);
    const grid = screen.getByRole("table", { name: "Replicas" });
    expect(within(grid).getAllByRole("rowheader").map((cell) => cell.textContent)).toEqual([
      "Current replicas",
      "Desired replicas",
    ]);
    const links = within(screen.getByRole("list", { name: "Findings" })).getAllByRole("link");
    expect(links.map((link) => link.getAttribute("href"))).toEqual([`/reports/${id}/findings?finding=8`]);
    expect(links[0]).toHaveTextContent(english.findings[8]?.sentence ?? "");
  });
});

describe("nodes tab", () => {
  it("node rows", async () => {
    await open("nodes");
    const orphan = within(rowOf("Nodes", "orphan")).getAllByRole("cell");
    expect(within(orphan[1] as HTMLElement).getByText("Undetermined")).not.toHaveClass("font-mono");
    expect(orphan[3]).toHaveTextContent("UnavailableUnknown");
    const second = within(rowOf("Nodes", "pool-a-2")).getAllByRole("cell");
    expect(second[3]).toHaveTextContent("Not ready · Unschedulable");
    const first = within(rowOf("Nodes", firstNode)).getAllByRole("cell");
    expect(first[4]).toHaveTextContent("95%");
    expect(first[4]).toHaveTextContent("25% requested");
  });

  it("node opened", async () => {
    const user = userEvent.setup();
    await open("nodes");
    const rows = within(screen.getByRole("table", { name: "Nodes" })).getAllByRole("row");
    expect(rows[1]).toHaveTextContent(firstNode);
    await user.click(screen.getByRole("button", { name: `Show the details of ${firstNode}` }));
    expect(screen.getAllByRole("button", { name: `Copy ${firstNode}` })).toHaveLength(1);
    expect(screen.getAllByText("CPU: peak of 95% utilization").filter((node) => node.closest("td[colspan]") !== null)).toHaveLength(0);
    expect(screen.getAllByText("CPU: peak of 95% utilization")).toHaveLength(1);
    const facts = screen.getByText("CPU capacity", { selector: "dt" }).closest("dl") as HTMLElement;
    expect(within(facts).getAllByRole("term").map((term) => term.textContent)).toEqual([
      "CPU capacity",
      "CPU allocatable",
      "Memory capacity",
      "Memory allocatable",
    ]);
    const grid = screen.getByRole("table", { name: "Utilization" });
    expect(within(grid).getAllByRole("rowheader").map((cell) => cell.textContent)).toEqual([
      "CPU %",
      "Memory %",
    ]);
    const links = within(screen.getByRole("list", { name: "Findings" })).getAllByRole("link");
    expect(links.map((link) => link.getAttribute("href"))).toEqual([0, 10].map((n) => `/reports/${id}/findings?finding=${String(n)}`));
  });

  it("the facts of a node with no allocatable show the muted placeholder", async () => {
    await open("nodes", "?open=orphan");
    const facts = screen.getByText("CPU allocatable", { selector: "dt" }).closest("dl") as HTMLElement;
    const placeholders = within(facts).getAllByText("Unavailable");
    expect(placeholders.length).toBeGreaterThan(0);
    for (const placeholder of placeholders) {
      expect(placeholder).toHaveClass("text-muted-fg");
    }
  });

  it("a peak with no number is its placeholder word alone, muted and upright", async () => {
    await open("nodes");
    const cpu = within(rowOf("Nodes", "orphan")).getAllByRole("cell")[4] as HTMLElement;
    expect(cpu).toHaveTextContent(/^Unavailable$/);
    const word = within(cpu).getByText("Unavailable");
    expect(word).toHaveClass("text-muted-fg");
    expect(cpu.querySelector(".font-mono")).toBeNull();
  });

  it("a peak cell takes the tint of the status the view gives its peak", async () => {
    const node = english.nodes[0] as View["nodes"][number];
    const peak = (raw: number, status: "healthy" | "attention" | "critical") => ({ raw, text: `${String(raw)}%`, status });
    const nodes: View["nodes"] = [
      { ...node, name: "n-attention", cpu_usage_percent: { ...node.cpu_usage_percent, peak: peak(40, "attention") } },
      { ...node, name: "n-critical", cpu_usage_percent: { ...node.cpu_usage_percent, peak: peak(40, "critical") } },
      { ...node, name: "n-healthy", cpu_usage_percent: { ...node.cpu_usage_percent, peak: peak(99, "healthy") } },
    ];
    mock.views.set(`${id}:en`, { ...english, nodes });
    await open("nodes");
    const cpu = (name: string) => within(rowOf("Nodes", name)).getAllByRole("cell")[4] as HTMLElement;
    expect(cpu("n-attention")).toHaveClass("bg-attention-surface");
    expect(cpu("n-attention")).not.toHaveClass("bg-critical-surface");
    expect(cpu("n-critical")).toHaveClass("bg-critical-surface");
    expect(cpu("n-critical")).not.toHaveClass("bg-attention-surface");
    expect(cpu("n-healthy")).not.toHaveClass("bg-attention-surface");
    expect(cpu("n-healthy")).not.toHaveClass("bg-critical-surface");
  });

  it("a peak cell is tinted critical and its share marked critical by the status the view gives the share", async () => {
    await open("nodes");
    const cpu = within(rowOf("Nodes", "pool-a-1")).getAllByRole("cell")[4] as HTMLElement;
    expect(cpu).toHaveClass("bg-critical-surface");
    const share = within(cpu).getByText("90% requested");
    expect(share).toHaveTextContent(/^Critical90% requested$/);
    const memory = within(rowOf("Nodes", "pool-a-1")).getAllByRole("cell")[5] as HTMLElement;
    expect(memory).not.toHaveClass("bg-critical-surface");
  });

  it("the state of a node", async () => {
    const node = english.nodes[0] as View["nodes"][number];
    const nodes: View["nodes"] = [
      { ...node, name: "n-unknown", ready: null, schedulable: null, state_status: "unavailable" },
      { ...node, name: "n-ready", ready: true, schedulable: null, state_status: null },
      { ...node, name: "n-down", ready: false, schedulable: null, state_status: "attention" },
      { ...node, name: "n-cordoned", ready: true, schedulable: false, state_status: "attention" },
      { ...node, name: "n-down-unmarked", ready: false, schedulable: true, state_status: null },
      { ...node, name: "n-unknown-ready", ready: null, schedulable: true, state_status: "unavailable" },
    ];
    mock.views.set(`${id}:en`, { ...english, nodes });
    await open("nodes");
    const state = (name: string) => within(rowOf("Nodes", name)).getAllByRole("cell")[3] as HTMLElement;
    expect(state("n-unknown")).toHaveTextContent(/^UnavailableUnknown$/);
    expect(state("n-ready")).toHaveTextContent(/^Ready$/);
    expect(state("n-down")).toHaveTextContent(/^AttentionNot ready$/);
    expect(state("n-cordoned")).toHaveTextContent(/^AttentionReady · Unschedulable$/);
    expect(state("n-cordoned").querySelector("svg")).toHaveClass("text-attention");
    expect(state("n-down-unmarked")).toHaveTextContent(/^Not ready · Schedulable$/);
    expect(state("n-down-unmarked").querySelector("svg")).toBeNull();
    expect(state("n-unknown-ready")).toHaveTextContent(/^UnavailableUnknown$/);
  });

  it("a node with no finding says so and ranks last", async () => {
    await open("nodes");
    const orphan = within(rowOf("Nodes", "orphan")).getAllByRole("cell");
    expect(orphan[2]).toHaveTextContent(/^No finding$/);
    expect(orphan[2]?.querySelector("svg")).toBeNull();
    expect(within(orphan[2] as HTMLElement).getByText("No finding")).toHaveClass("text-muted-fg");
    const second = within(rowOf("Nodes", "pool-a-2")).getAllByRole("cell");
    expect(second[2]).toHaveTextContent(/^AttentionThe node was not ready at the end of the period; The node was unschedulable at the end of the period$/);
    const names = within(screen.getByRole("table", { name: "Nodes" }))
      .getAllByRole("row")
      .slice(1)
      .map((row) => within(row).getAllByRole("cell")[1]?.textContent ?? "");
    expect(names[names.length - 1]).toContain("orphan");
  });

  it("the status cell of a node says why under the severity, and none for no finding", async () => {
    await open("nodes");
    const status = (name: string) => within(rowOf("Nodes", name)).getAllByRole("cell")[2] as HTMLElement;
    expect(status("pool-a-1")).toHaveTextContent("CPU: 90% of the allocatable requested");
    const line = within(status("pool-a-1")).getByText("CPU: 90% of the allocatable requested");
    expect(line).toHaveClass("text-xs");
    expect(line.querySelector("svg")).toBeNull();
    expect(status("orphan")).toHaveTextContent(/^No finding$/);
  });

  it("the state of a node carries a shape", async () => {
    await open("nodes");
    const down = within(rowOf("Nodes", "pool-a-2")).getAllByRole("cell")[3] as HTMLElement;
    expect(down.querySelector("svg")).toHaveClass("text-attention");
    expect(down).toHaveTextContent("AttentionNot ready · Unschedulable");
    expect(within(down).getByText("Attention")).toHaveClass("sr-only");
    const unknown = within(rowOf("Nodes", "orphan")).getAllByRole("cell")[3] as HTMLElement;
    expect(unknown.querySelector("svg")).toHaveClass("text-neutral");
    expect(unknown).toHaveTextContent(/^UnavailableUnknown$/);
  });

  it("no finding is worded in Portuguese", async () => {
    await open("nodes");
    await act(async () => {
      await i18next.changeLanguage("pt-BR");
    });
    expect((await screen.findAllByText("Sem achado")).length).toBeGreaterThan(1);
  });

  it("the search of nodes finds the shown pool word", async () => {
    const user = userEvent.setup();
    await open("nodes");
    await user.type(screen.getByRole("searchbox", { name: "Search nodes" }), "undetermined");
    const rows = within(screen.getByRole("table", { name: "Nodes" })).getAllByRole("row").slice(1);
    expect(rows).toHaveLength(1);
    expect(rows[0]).toHaveTextContent("orphan");
  });

  it("no pool leaves the empty word", async () => {
    mock.views.set(`${id}:en`, { ...english, nodepools: [] });
    await open("nodes");
    const heading = screen.getByRole("heading", { name: "Node pools" });
    expect(within(heading.parentElement as HTMLElement).getByText("No rows in this report.")).toBeInTheDocument();
    expect(screen.queryByRole("table", { name: "Node pools" })).toBeNull();
  });

  it("copy", async () => {
    const user = userEvent.setup();
    const writeText = vi.fn<(text: string) => Promise<void>>().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    await open("nodes", `?open=${firstNode}`);
    await user.click(screen.getByRole("button", { name: `Copy ${firstNode}` }));
    expect(writeText).toHaveBeenCalledWith(firstNode);
    expect(await within(screen.getByRole("status")).findByText(`Copied: ${firstNode}`)).toBeInTheDocument();
    writeText.mockRejectedValue(new Error("refused"));
    await user.click(screen.getByRole("button", { name: `Copy ${firstNode}` }));
    await waitFor(() => {
      expect(screen.getByRole("status")).toHaveTextContent("The name could not be copied.");
    });
  });

  it("pools", async () => {
    await open("nodes");
    const table = screen.getByRole("table", { name: "Node pools" });
    expect(screen.getByRole("heading", { name: "Node pools" })).toBeInTheDocument();
    const rows = within(table).getAllByRole("row").slice(1);
    expect(rows).toHaveLength(3);
    expect(within(rows[0] as HTMLElement).getByRole("button", { name: "Copy agentpool" })).toBeInTheDocument();
    expect(within(rows[1] as HTMLElement).getByRole("button", { name: "Copy pool-a" })).toBeInTheDocument();
    expect(within(rows[2] as HTMLElement).queryByRole("button")).not.toBeInTheDocument();
    expect(within(rows[2] as HTMLElement).getByText("Undetermined")).toBeInTheDocument();
    const counts = rows.map((row) => within(row).getAllByRole("cell")[3] as HTMLElement);
    expect(counts.map((cell) => cell.querySelector("span")?.textContent)).toEqual(["1", "2", "1"]);
    expect(counts.map((cell) => within(cell).getAllByRole("listitem").map((item) => item.textContent))).toEqual([
      [firstNode],
      ["pool-a-1", "pool-a-2"],
      ["orphan"],
    ]);
    expect(within(counts[1] as HTMLElement).getAllByRole("listitem")[0]).toHaveClass("font-mono");
  });

  it("the pools carry the status of their worst finding, or say none", async () => {
    await open("nodes");
    const table = screen.getByRole("table", { name: "Node pools" });
    expect(within(table).getAllByRole("columnheader").map((cell) => cell.textContent)).toEqual([
      "Node pool",
      "Status",
      "Confidence",
      "Nodes",
    ]);
    const status = within(table)
      .getAllByRole("row")
      .slice(1)
      .map((row) => within(row).getAllByRole("cell")[1]?.textContent);
    expect(status).toEqual(["No finding", "Attention", "Attention"]);
  });
});

describe("words of the autoscaling and nodes tabs in portuguese", () => {
  it("name the replicas and the coverage in the masculine", () => {
    expect(i18next.t("report.stat.desiredReplicas", { lng: "pt-BR" })).toBe("Replicas desejados");
    expect(i18next.t("hpaCoverage.unknown", { lng: "pt-BR" })).toBe("Desconhecido");
    expect(i18next.t("report.stat.replicas", { lng: "pt-BR" })).toBe("Replicas");
    expect(i18next.t("report.stat.utilization", { lng: "pt-BR" })).toBe("Utilização");
  });
});

describe("copy without a clipboard", () => {
  it("announces the failure and throws nothing", async () => {
    const user = userEvent.setup();
    await open("nodes", `?open=${firstNode}`);
    Object.defineProperty(navigator, "clipboard", { value: undefined, configurable: true });
    await user.click(screen.getByRole("button", { name: `Copy ${firstNode}` }));
    await waitFor(() => {
      expect(screen.getByRole("status")).toHaveTextContent("The name could not be copied.");
    });
  });
});

describe("replica range", () => {
  it("an unavailable bound is a muted sans placeholder, never mono", async () => {
    const hpa = english.hpas[0] as components["schemas"]["ViewHPA"];
    const gone = { raw: null, text: "Unavailable", status: null };
    mock.views.set(`${id}:en`, { ...english, hpas: [{ ...hpa, spec_min: gone, spec_max: gone }] });
    await open("autoscaling");
    const cell = within(screen.getByRole("table", { name: "HPAs" })).getAllByRole("cell").find((c) => /^Unavailable \/ Unavailable$/.test(c.textContent));
    expect(cell).toBeDefined();
    for (const word of within(cell as HTMLElement).getAllByText("Unavailable")) {
      expect(word).toHaveClass("text-muted-fg");
      expect(word.closest(".font-mono")).toBeNull();
    }
  });
});

describe("autoscaling tab verdict", () => {
  it("opens on the verdict of the cluster and the evidence warning", async () => {
    await open("autoscaling");
    const section = screen.getByRole("heading", { level: 2, name: "Cluster autoscaling" }).parentElement?.parentElement as HTMLElement;
    const sentence = within(section).getByText("Node autoscaling is configured as disabled.");
    expect(within(sentence.parentElement as HTMLElement).getByText("Critical")).toBeInTheDocument();
    expect(within(section).getByText("The capacity evidence of the nodes is incomplete.").parentElement?.querySelector("svg")).toHaveClass("text-attention");
  });

  it("shows the cluster finding alone when the evidence is complete", async () => {
    mock.views.set(`${id}:en`, directComplete as View);
    await open("autoscaling");
    expect(screen.getByText("Node autoscaling enabled with observed headroom.")).toBeInTheDocument();
    expect(screen.queryByText("The capacity evidence of the nodes is incomplete.")).not.toBeInTheDocument();
  });
});
