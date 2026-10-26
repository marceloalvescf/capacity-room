import { render } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";
import "@/i18n/i18n";
import type { ViewScope } from "./api";
import { Donut, lift, minSweep, minWidth, ScopeBar } from "./findings-chart";

vi.mock("./report-page", () => ({ useView: () => ({ findings: [] }) }));

describe("lift", () => {
  it("raises a small count and takes the difference from the largest", () => {
    const lifted = lift([0.9, 0, 359.1], 6);
    expect(lifted).toEqual([6, 0, 354]);
  });
  it("leaves values that already fit", () => {
    expect(lift([100, 0, 260], 6)).toEqual([100, 0, 260]);
  });
});

function degrees(d: string): number {
  const [from, to] = [...d.matchAll(/(-?\d+\.\d+) (-?\d+\.\d+)/g)].filter((_, index) => index === 0 || index === 1).map((match) => Math.atan2(Number(match[2]) - 100, Number(match[1]) - 100));
  return (((to ?? 0) - (from ?? 0)) * 180) / Math.PI;
}

const threeTotals = [
  { severity: "critical", count: 1 },
  { severity: "attention", count: 0 },
  { severity: "healthy", count: 400 },
] as const;

describe("Donut", () => {
  it("draws a critical slice of at least the least sweep and fills the ring", () => {
    const { container } = render(<Donut totals={threeTotals} />);
    const paths = [...container.querySelectorAll("path")];
    expect(paths).toHaveLength(2);
    const critical = degrees(paths[0]?.getAttribute("d") ?? "");
    const healthy = degrees(paths[1]?.getAttribute("d") ?? "");
    expect(critical).toBeGreaterThanOrEqual(minSweep - 0.01);
    expect(Math.abs(critical) + Math.abs(healthy > 0 ? healthy : healthy + 360)).toBeCloseTo(360, 0);
  });
});

describe("ScopeBar", () => {
  it("draws a critical segment of at least the least width and keeps the total", () => {
    const scope = { scope_type: "node", severity: "critical", critical: 1, attention: 0, healthy: 400, finding: 0 } as ViewScope;
    const { container } = render(
      <MemoryRouter>
        <ul>
          <ScopeBar scope={scope} totals={threeTotals} scale={401} link={false} />
        </ul>
      </MemoryRouter>,
    );
    const rects = [...container.querySelectorAll("svg rect")].slice(1);
    expect(rects).toHaveLength(2);
    const widths = rects.map((rect) => Number(rect.getAttribute("width")) + 2);
    expect(widths[0]).toBeGreaterThanOrEqual(minWidth);
    const last = rects[1];
    expect(Number(last?.getAttribute("x")) + Number(last?.getAttribute("width")) + 2).toBeCloseTo(1000, 3);
  });
});
