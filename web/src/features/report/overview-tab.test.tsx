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
  it("severity tiles", async () => {
    await open();
    const links = screen.getAllByRole("link", { name: "See findings" });
    expect(links).toHaveLength(3);
    const tiles = links.map((link) => link.parentElement as HTMLElement);
    expect(tiles.map((tile) => tile.textContent)).toEqual([
      expect.stringMatching(/^Critical.*9See findings$/),
      expect.stringMatching(/^Attention.*10See findings$/),
      expect.stringMatching(/^Healthy.*7See findings$/),
    ]);
    expect(links.map((link) => link.getAttribute("href"))).toEqual([
      expect.stringMatching(/findings\?severity=critical$/),
      expect.stringMatching(/findings\?severity=attention$/),
      expect.stringMatching(/findings\?severity=healthy$/),
    ]);
    expect(within(tiles[0] as HTMLElement).getByText("9")).toHaveClass("font-mono");
  });

  it("a tile below the minimum severity keeps its count and no link", async () => {
    mock.views.set(`${id}:en`, { ...english, min_severity: "attention" });
    await open();
    const links = screen.getAllByRole("link", { name: "See findings" });
    expect(links.map((link) => link.getAttribute("href"))).toEqual([
      expect.stringMatching(/findings\?severity=critical$/),
      expect.stringMatching(/findings\?severity=attention$/),
    ]);
    const tiles = [...((links[0] as HTMLElement).parentElement?.parentElement?.children ?? [])] as HTMLElement[];
    expect(tiles).toHaveLength(3);
    const healthy = tiles[2] as HTMLElement;
    expect(healthy).toHaveTextContent(/^Healthy.*7$/);
    expect(within(healthy).getByText("7")).toHaveClass("font-mono");
    expect(within(healthy).queryByRole("link")).toBeNull();

    mock.views.set(`${id}:en`, { ...english, min_severity: "critical" });
    cleanup();
    await open();
    const only = screen.getAllByRole("link", { name: "See findings" });
    expect(only).toHaveLength(1);
    const kept = [...((only[0] as HTMLElement).parentElement?.parentElement?.children ?? [])] as HTMLElement[];
    expect(kept[1]).toHaveTextContent(/^Attention.*10$/);
  });

  it("inventory", async () => {
    await open();
    const list = screen.getByLabelText("Inventory");
    const items = [...list.children] as HTMLElement[];
    expect(items).toHaveLength(6);
    const names = ["Namespaces", "Workloads", "HPAs", "Workloads without HPA", "Nodes", "Node pools"];
    const numbers = ["1", "1", "4", "2", "4", "3"];
    items.forEach((item, index) => {
      const number = item.querySelector("dd") as HTMLElement;
      const name = item.querySelector("dt") as HTMLElement;
      expect(number).toHaveTextContent(numbers[index] ?? "");
      expect(name).toHaveTextContent(names[index] ?? "");
      expect(name.compareDocumentPosition(number) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
      expect(item).toHaveClass("flex-col-reverse");
    });
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
