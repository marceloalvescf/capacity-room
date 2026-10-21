import { render } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { i18next } from "@/i18n/i18n";
import { RangeBar } from "./range-bar";
import type { ViewFinding } from "./api";

type Evidence = ViewFinding["evidence"][number];

const evidence = (name: Evidence["name"], raw: number): Evidence & { raw: number } => ({ name, raw, text: `${String(raw)}%` });

function draw(low: number, high: number) {
  const { container } = render(
    <RangeBar
      low={evidence("minimum_peak_percent", low)}
      high={evidence("maximum_peak_percent", high)}
      status="attention"
    />,
  );
  return container.querySelector("svg");
}

beforeEach(async () => {
  await i18next.changeLanguage("en");
});

describe("range bar", () => {
  it("is a drawing hidden from assistive technology, with no inline style", () => {
    const svg = draw(20, 60);
    expect(svg).toHaveAttribute("aria-hidden", "true");
    expect(svg?.querySelector("[style]")).toBeNull();
  });

  it("puts an open dot at the lowest and a filled dot at the highest, by percent of the width", () => {
    const svg = draw(20, 60);
    const open = svg?.querySelector("[data-low]");
    const filled = svg?.querySelector("[data-high]");
    expect(svg?.querySelector("[data-track]")).toHaveAttribute("width", "100%");
    expect(open).toHaveAttribute("cx", "20%");
    expect(filled).toHaveAttribute("cx", "60%");
    expect(svg?.querySelector("[data-span]")).toHaveAttribute("x", "20%");
    expect(svg?.querySelector("[data-span]")).toHaveAttribute("width", "40%");
    expect(open).toHaveClass("fill-surface", "stroke-attention");
    expect(filled).toHaveClass("fill-attention");
  });

  it("hangs the lowest label above the track and the highest below it, each toward the centre", () => {
    const cases: [number, number, string, string][] = [
      [0, 100, "start", "end"],
      [0, 10, "start", "start"],
      [90, 95, "end", "end"],
      [20, 50, "start", "start"],
      [20, 60, "start", "end"],
    ];
    for (const [low, high, lowAnchor, highAnchor] of cases) {
      const { container, unmount } = render(
        <RangeBar low={evidence("minimum_peak_percent", low)} high={evidence("maximum_peak_percent", high)} status="attention" />,
      );
      const track = Number(container.querySelector("[data-track]")?.getAttribute("y"));
      const lowLabel = container.querySelector("[data-label=low]");
      const highLabel = container.querySelector("[data-label=high]");
      expect(Number(lowLabel?.getAttribute("y")), String(low)).toBeLessThan(track);
      expect(Number(highLabel?.getAttribute("y")), String(high)).toBeGreaterThan(track);
      expect([lowLabel?.getAttribute("text-anchor"), highLabel?.getAttribute("text-anchor")]).toEqual([lowAnchor, highAnchor]);
      expect([lowLabel?.getAttribute("x"), highLabel?.getAttribute("x")]).toEqual([`${String(low)}%`, `${String(high)}%`]);
      unmount();
    }
  });

  it("draws the spread between the dots at a lower opacity", () => {
    const span = draw(20, 60)?.querySelector("[data-span]");
    expect(span).toHaveClass("fill-attention", "opacity-35");
  });

  it("labels the dots with the words and texts of the evidence, and the scale", () => {
    const text = draw(20, 60)?.textContent ?? "";
    for (const label of ["Lowest peak 20%", "Highest peak 60%", "0%", "25%", "50%", "75%", "100%"]) {
      expect(text).toContain(label);
    }
    expect(text).not.toContain("Spread");
  });

  it("draws a lowest of 0 as a range with both dots, no fill from zero", () => {
    const svg = draw(0, 60);
    expect(svg?.querySelector("[data-low]")).toHaveAttribute("cx", "0%");
    expect(svg?.querySelector("[data-high]")).not.toBeNull();
    expect(svg?.querySelector("[data-fill]")).toBeNull();
  });
});
