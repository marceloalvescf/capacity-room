import { act, screen, within } from "@testing-library/react";
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

const t = (key: string): string => i18next.t(key);

type View = components["schemas"]["ReportView"];

const id = "0b6c3b52-2f0e-4d0a-8d57-6f6f6c1f0a01";
const english = viewEn as View;
const portuguese = viewPt as View;
const workloadKey = "payments%2FDeployment%2Fpayments-web";

beforeEach(() => {
  window.localStorage.clear();
  mockColourScheme(false);
  mock.profiles = [profileSummary()];
  mock.reports = [report()];
  mock.views.set(`${id}:en`, english);
  mock.views.set(`${id}:pt-BR`, portuguese);
});

async function open(tab: "namespaces" | "workloads", search = "", language: "en" | "pt-BR" = "en") {
  const view = await renderApp(`/reports/${id}/${tab}${search}`, language);
  await screen.findByRole("searchbox", { name: i18next.t(`report.search.${tab}`) });
  return view;
}

function bodyRows(): HTMLElement[] {
  return within(screen.getByRole("table", { name: /^(Namespaces|Workloads|Namespace|Workload)/ }))
    .getAllByRole("row")
    .slice(1);
}

function hrefEnding(index: number): unknown {
  return expect.stringMatching(new RegExp(`findings\\?finding=${String(index)}$`));
}

function textHolding(sentence: string): unknown {
  return expect.stringContaining(sentence);
}

function shapeOf(node: HTMLElement): string | undefined {
  return ["text-healthy", "text-attention", "text-critical", "text-neutral"].find((name) =>
    node.querySelector("svg")?.classList.contains(name),
  );
}

describe("workloads tab", () => {
  it("workloads columns", async () => {
    await open("workloads");
    const rows = bodyRows();
    expect(rows).toHaveLength(1);
    expect(screen.getByText("Rows: 1")).toBeInTheDocument();
    const cells = within(rows[0] as HTMLElement).getAllByRole("cell");
    const name = cells[1] as HTMLElement;
    expect(within(name).getByText("payments-web")).toHaveClass("font-mono");
    expect(within(name).getByText("Deployment")).toHaveClass("font-mono");
    expect(within(name).getByText("payments")).toHaveClass("font-mono");
    expect(cells[2]).toHaveTextContent("Critical");
    expect(cells[3]).toHaveTextContent("1");
    expect(cells[3]?.querySelector(".font-mono")).not.toBeNull();
    const cpu = cells[4] as HTMLElement;
    expect(within(cpu).getByText("400 mCPU")).toBeInTheDocument();
    const cpuShare = within(cpu).getByText("80% of request");
    expect(shapeOf(cpuShare)).toBe("text-attention");
    const memory = cells[5] as HTMLElement;
    expect(within(memory).getByText("120 B")).toBeInTheDocument();
    expect(shapeOf(within(memory).getByText("24000% of request"))).toBe("text-critical");
  });

  it("workload opened", async () => {
    const user = userEvent.setup();
    const { router } = await open("workloads");
    await user.click(screen.getByRole("button", { name: "Show the details of payments-web" }));
    expect(location(router)).toBe(`/reports/${id}/workloads?open=${workloadKey}`);
    const facts = screen.getByText("HPA", { selector: "dt" }).closest("dl") as HTMLElement;
    expect(facts).toHaveTextContent("Pods1");
    expect(facts).toHaveTextContent("HPACovered by an HPA");
    expect(facts).toHaveTextContent("ValuesMeasured");
    const grid = screen.getByRole("table", { name: "CPU" });
    const gridRows = within(grid).getAllByRole("row");
    expect(within(gridRows[0] as HTMLElement).getAllByRole("columnheader").map((cell) => cell.textContent)).toEqual([
      "CPU",
      "Average",
      "Minimum",
      "Peak",
      "Current",
    ]);
    expect(gridRows.slice(1).map((row) => within(row).getAllByRole("rowheader")[0]?.textContent)).toEqual([
      "Usage",
      "Request",
      "Limit",
      "% Request",
      "% Limit",
    ]);
    const requestPercent = within(gridRows[4] as HTMLElement).getAllByRole("cell");
    expect(requestPercent.map((cell) => cell.textContent)).toEqual(["Healthy60%", "Healthy40%", "Attention80%", "Attention80%"]);
    expect(shapeOf(requestPercent[2] as HTMLElement)).toBe("text-attention");
    expect(requestPercent[2]).toHaveClass("font-mono", "text-right");
    expect(screen.getByRole("table", { name: "Memory" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Findings" })).toBeInTheDocument();
    const links = within(screen.getByRole("list", { name: "Findings" })).getAllByRole("link");
    expect(links.map((link) => link.getAttribute("href"))).toEqual([5, 6, 13, 20].map(hrefEnding));
    expect(links.map((link) => link.textContent)).toEqual(
      [5, 6, 13, 20].map((index) => textHolding(english.findings[index]?.sentence ?? "")),
    );
  });

  it("a share with no number keeps its mark and status word", async () => {
    mock.views.set(`${id}:en`, {
      ...english,
      workloads: english.workloads.map((w) => ({
        ...w,
        cpu_request_percent: { ...w.cpu_request_percent, peak: { raw: null, text: "Not defined", status: "undefined" } },
      })),
    });
    await open("workloads");
    const cpu = within(bodyRows()[0] as HTMLElement).getAllByRole("cell")[4] as HTMLElement;
    const share = within(cpu).getByText("Not defined");
    expect(share).toHaveClass("text-muted-fg");
    expect(shapeOf(share)).toBe("text-neutral");
    expect(share.parentElement).toHaveTextContent(`${t("status.undefined")}Not defined`);
    expect(cpu).not.toHaveTextContent("of request");
  });

  it("language switch keeps the opened row", async () => {
    const { router } = await open("workloads", `?q=pay&open=${workloadKey}`);
    const before = location(router);
    expect(screen.getByRole("table", { name: "CPU" })).toBeInTheDocument();
    await act(async () => {
      await i18next.changeLanguage("pt-BR");
    });
    expect(await screen.findAllByRole("columnheader", { name: "Média" })).toHaveLength(2);
    expect(location(router)).toBe(before);
    expect(screen.getByRole("table", { name: "CPU" })).toBeInTheDocument();
    expect(screen.getAllByRole("rowheader", { name: "Uso" })).toHaveLength(2);
    expect(screen.getByRole("searchbox")).toHaveValue("pay");
  });

  it("workloads with no observed usage leave no row", async () => {
    mock.views.set(`${id}:en`, { ...english, workloads: english.workloads.map((w) => ({ ...w, observed_usage: false })) });
    await renderApp(`/reports/${id}/workloads`, "en");
    expect(await screen.findByText("No rows in this report.")).toBeInTheDocument();
  });
});

describe("namespaces tab", () => {
  it("namespaces", async () => {
    const user = userEvent.setup();
    await open("namespaces");
    const rows = bodyRows();
    expect(rows).toHaveLength(1);
    const cells = within(rows[0] as HTMLElement).getAllByRole("cell");
    expect(within(cells[1] as HTMLElement).getByText("payments")).toHaveClass("font-mono");
    expect(cells[2]).toHaveTextContent("Critical");
    expect(shapeOf(within(cells[4] as HTMLElement).getByText("660% of request"))).toBe("text-critical");
    await user.click(screen.getByRole("button", { name: "Show the details of payments" }));
    expect(screen.getByRole("table", { name: "CPU" })).toBeInTheDocument();
    expect(screen.getByRole("table", { name: "Memory" })).toBeInTheDocument();
    expect(screen.queryByText("HPA", { selector: "dt" })).not.toBeInTheDocument();
    const links = within(screen.getByRole("list", { name: "Findings" })).getAllByRole("link");
    expect(links.map((link) => link.getAttribute("href"))).toEqual([1, 2, 3, 4].map(hrefEnding));
  });

  it("a missing number shows its placeholder muted", async () => {
    mock.views.set(`${id}:en`, {
      ...english,
      namespaces: english.namespaces.map((n) => ({ ...n, cpu_usage: { ...n.cpu_usage, peak: { raw: null, text: "Unavailable", status: null } } })),
    });
    await open("namespaces");
    const cpu = within(bodyRows()[0] as HTMLElement).getAllByRole("cell")[4] as HTMLElement;
    expect(within(cpu).getByText("Unavailable")).toHaveClass("text-muted-fg");
  });
});
