import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
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
    expect(screen.getByText("Enabled")).toBeInTheDocument();
    expect(screen.getByText("Source: Raw metrics")).toBeInTheDocument();
    expect(screen.getByText("Confidence: Inferred")).toBeInTheDocument();
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
    expect(list.getAllByRole("definition").map((value) => value.textContent)).toEqual(["3", "2", "3", "2", "1", "2", "2"]);
  });

  it("hpa rows", async () => {
    await open("autoscaling");
    const table = screen.getByRole("table", { name: "HPAs" });
    expect(within(table).getAllByRole("row")).toHaveLength(5);
    const web = within(rowOf("HPAs", "web")).getAllByRole("cell");
    expect(web[1]).toHaveTextContent("Deployment payments-web · payments");
    expect(web[2]).toHaveTextContent("Maximum reached");
    expect(within(web[2] as HTMLElement).getByText("Maximum reached").closest("span.inline-flex")).toHaveClass("text-critical");
    expect(web[3]).toHaveTextContent("2 / 4");
    expect(web[3]?.querySelector(".font-mono")).not.toBeNull();
    expect(web[3]).toHaveClass("text-right");
    expect(web[4]).toHaveTextContent("4");
    const dup = within(rowOf("HPAs", "dup")).getAllByRole("cell");
    expect(dup[3]).toHaveTextContent("1 / Unavailable");
  });

  it("hpa opened", async () => {
    const user = userEvent.setup();
    const { router } = await open("autoscaling");
    await user.click(screen.getByRole("button", { name: "Show the details of web" }));
    expect(location(router)).toBe(`/reports/${id}/autoscaling?open=payments%2Fweb`);
    const grid = screen.getByRole("table", { name: "" });
    expect(within(grid).getAllByRole("rowheader").map((cell) => cell.textContent)).toEqual([
      "Current replicas",
      "Desired replicas",
    ]);
    const links = within(screen.getByRole("list", { name: "Findings" })).getAllByRole("link");
    expect(links.map((link) => link.getAttribute("href"))).toEqual([`/reports/${id}/findings?finding=7`]);
    expect(links[0]).toHaveTextContent(english.findings[7]?.sentence ?? "");
  });
});

describe("nodes tab", () => {
  it("node rows", async () => {
    await open("nodes");
    const orphan = within(rowOf("Nodes", "orphan")).getAllByRole("cell");
    expect(within(orphan[1] as HTMLElement).getByText("Unknown pool")).not.toHaveClass("font-mono");
    expect(orphan[3]).toHaveTextContent("Unknown");
    const second = within(rowOf("Nodes", "pool-a-2")).getAllByRole("cell");
    expect(second[3]).toHaveTextContent("Not ready · Unschedulable");
    expect(within(second[3] as HTMLElement).getByText("Not ready · Unschedulable")).toHaveClass("text-attention");
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
    const reason = screen.getAllByText("CPU: peak of 95% utilization").find((node) => node.closest("td[colspan]") !== null);
    expect(reason).toBeDefined();
    expect((reason as HTMLElement).closest("span.inline-flex")).toHaveClass("text-critical");
    const facts = screen.getByText("CPU capacity", { selector: "dt" }).closest("dl") as HTMLElement;
    expect(within(facts).getAllByRole("term").map((term) => term.textContent)).toEqual([
      "CPU capacity",
      "CPU allocatable",
      "Memory capacity",
      "Memory allocatable",
    ]);
    const grids = screen.getAllByRole("table", { name: "" });
    expect(within(grids[0] as HTMLElement).getAllByRole("rowheader").map((cell) => cell.textContent)).toEqual([
      "CPU %",
      "Memory %",
    ]);
    const links = within(screen.getByRole("list", { name: "Findings" })).getAllByRole("link");
    expect(links.map((link) => link.getAttribute("href"))).toEqual([0, 9].map((n) => `/reports/${id}/findings?finding=${String(n)}`));
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
    expect(within(rows[2] as HTMLElement).getByText("Unknown pool")).toBeInTheDocument();
    const counts = rows.map((row) => within(row).getAllByRole("cell")[2]);
    expect(counts.map((cell) => cell?.textContent)).toEqual([
      `1${firstNode}`,
      "2pool-a-1, pool-a-2",
      "1orphan",
    ]);
    expect(counts[1]?.querySelector(".font-mono")).not.toBeNull();
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
