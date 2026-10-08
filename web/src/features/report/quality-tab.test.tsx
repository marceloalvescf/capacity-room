import { cleanup, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";
import type { components } from "@/api/schema.gen";
import { i18next } from "@/i18n/i18n";
import { installMockApi, profileSummary, report, signedIn } from "../../../tests/msw";
import { renderApp } from "../../../tests/render";
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

async function open(tab = "quality", search = "") {
  const view = await renderApp(`/reports/${id}/${tab}${search}`, "en");
  await screen.findByRole("heading", { level: 1, name: i18next.t("progress.title", { profile: "Production" }) });
  return view;
}

describe("data quality tab", () => {
  it("collection line", async () => {
    await open();
    expect(await screen.findByText("Collected from 2026-07-15 14:30 to 2026-07-15 14:30, in 0:42")).toBeInTheDocument();
  });

  it("collection line in the zone of the report", async () => {
    mock.views.set(`${id}:en`, { ...english, report: { ...english.report, timezone: "Asia/Tokyo" } });
    await open();
    expect(await screen.findByText("Collected from 2026-07-16 02:30 to 2026-07-16 02:30, in 0:42")).toBeInTheDocument();
  });

  it("an unavailable capability is critical", async () => {
    mock.views.set(`${id}:en`, {
      ...english,
      quality: {
        ...english.quality,
        capabilities: english.quality.capabilities.map((c) => (c.name === "node_autoscaling" ? { ...c, state: "unavailable" as const } : c)),
      },
    });
    await open();
    const table = await screen.findByRole("table", { name: "Data quality" });
    const row = within(table).getAllByRole("row").slice(1)[8] as HTMLElement;
    const cell = within(row).getAllByRole("cell")[2] as HTMLElement;
    expect(within(cell).getByText("Unavailable").closest("span.inline-flex")).toHaveClass("text-critical");
    expect(cell.querySelector("svg polygon")).not.toBeNull();
  });

  it("capabilities in view order", async () => {
    await open();
    const table = await screen.findByRole("table", { name: "Data quality" });
    const rows = within(table).getAllByRole("row").slice(1);
    expect(rows).toHaveLength(9);
    expect(rows.map((row) => within(row).getAllByRole("cell")[1]?.textContent)).toEqual([
      "Usage by namespace",
      "Usage by workload",
      "Requests and limits",
      "HPA history",
      "Coverage of workloads without HPA",
      "Mapping between address and node",
      "Identification of node pools",
      "Node utilization",
      "Node autoscaling",
    ]);
    const hpa = within(rows[3] as HTMLElement).getAllByRole("cell");
    const state = within(hpa[2] as HTMLElement).getByText("Partial");
    expect(state.closest("span.inline-flex")).toHaveClass("text-attention");
    expect(hpa[2]?.querySelector("svg polygon")).not.toBeNull();
    expect(hpa[3]).toHaveTextContent("Raw metrics");
    expect(hpa[4]).toHaveTextContent("Ambiguous HPA configuration or target values were omitted.");
  });

  it("metric names only here", async () => {
    const user = userEvent.setup();
    await open();
    await user.click(await screen.findByRole("button", { name: "Show the details of HPA history" }));
    expect(screen.getByText("Metrics read")).toBeInTheDocument();
    const names = [
      "kube_horizontalpodautoscaler_info",
      "kube_horizontalpodautoscaler_spec_min_replicas",
      "kube_horizontalpodautoscaler_spec_max_replicas",
      "kube_horizontalpodautoscaler_status_current_replicas",
      "kube_horizontalpodautoscaler_status_desired_replicas",
    ];
    for (const name of names) {
      expect(screen.getByText(name)).toHaveClass("font-mono");
    }
    for (const tab of ["overview", "findings", "namespaces", "workloads", "autoscaling", "nodes"]) {
      cleanup();
      await open(tab);
      expect(document.body.textContent).not.toContain("kube_");
    }
  });

  it("warnings and notices", async () => {
    await open();
    const warnings = await screen.findByRole("list", { name: "Warnings" });
    const items = within(warnings).getAllByRole("listitem");
    expect(items.map((item) => item.textContent)).toEqual([
      "AttentionAmbiguous HPA configuration or target values were omitted.",
      "AttentionThe capacity evidence of the nodes is incomplete.",
    ]);
    for (const item of items) {
      expect(item.querySelector("span.inline-flex")).toHaveClass("text-attention");
      expect(within(item).getByText("Attention")).toHaveClass("sr-only");
    }
    const notices = within(screen.getByRole("list", { name: "Notices" })).getAllByRole("listitem");
    expect(notices.map((item) => item.textContent)).toEqual([
      "Attention1 workload was left out of the coverage inventory because it is scaled to zero.",
    ]);
    expect(notices[0]?.querySelector("span.inline-flex")).toHaveClass("text-attention");
  });

  it("two messages of one code both show", async () => {
    const [first] = english.quality.warnings;
    mock.views.set(`${id}:en`, {
      ...english,
      quality: { ...english.quality, warnings: [first as (typeof english.quality.warnings)[number], { ...(first as (typeof english.quality.warnings)[number]), text: "A second warning." }] },
    });
    await open();
    const warnings = await screen.findByRole("list", { name: "Warnings" });
    expect(within(warnings).getAllByRole("listitem")).toHaveLength(2);
  });

  it("no warnings", async () => {
    mock.views.set(`${id}:en`, { ...english, quality: { ...english.quality, warnings: [], notices: [] } });
    await open();
    expect(await screen.findByText("No warning.")).toBeInTheDocument();
    expect(screen.getByText("No notice.")).toBeInTheDocument();
    expect(screen.queryByRole("list", { name: "Warnings" })).not.toBeInTheDocument();
    expect(screen.queryByRole("list", { name: "Notices" })).not.toBeInTheDocument();
  });
});
