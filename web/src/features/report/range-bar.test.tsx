import { render } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { i18next } from "@/i18n/i18n";
import { RangeBar } from "./range-bar";
import type { ViewFinding } from "./api";

type Evidence = ViewFinding["evidence"][number];

const evidence = (name: Evidence["name"], raw: number): Evidence & { raw: number } => ({ name, raw, text: `${String(raw)}%` });

function draw(low: number, high: number, spread: number | null = high - low) {
  const { container } = render(
    <RangeBar
      low={evidence("minimum_peak_percent", low)}
      high={evidence("maximum_peak_percent", high)}
      spread={spread === null ? null : evidence("spread_percent", spread)}
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

  it("puts an open dot at the lowest and a filled dot at the highest, on a track of 0 to 100", () => {
    const svg = draw(20, 60);
    const track = svg?.querySelector("[data-track]");
    const x = Number(track?.getAttribute("x"));
    const width = Number(track?.getAttribute("width"));
    const open = svg?.querySelector("[data-low]");
    const filled = svg?.querySelector("[data-high]");
    expect(Number(open?.getAttribute("cx"))).toBeCloseTo(x + width * 0.2);
    expect(Number(filled?.getAttribute("cx"))).toBeCloseTo(x + width * 0.6);
    expect(open).toHaveClass("fill-surface", "stroke-attention");
    expect(filled).toHaveClass("fill-attention");
  });

  it("draws the spread between the dots at a lower opacity", () => {
    const span = draw(20, 60)?.querySelector("[data-span]");
    expect(span).toHaveClass("fill-attention", "opacity-35");
  });

  it("labels the dots and the spread with the words and texts of the evidence", () => {
    const text = draw(20, 60)?.textContent ?? "";
    for (const label of ["Lowest peak 20%", "Highest peak 60%", "Spread 40%", "0%", "25%", "50%", "75%", "100%"]) {
      expect(text).toContain(label);
    }
  });

  it("draws a lowest of 0 as a range with both dots, no fill from zero", () => {
    const svg = draw(0, 60);
    const track = svg?.querySelector("[data-track]");
    expect(svg?.querySelector("[data-low]")?.getAttribute("cx")).toBe(track?.getAttribute("x"));
    expect(svg?.querySelector("[data-high]")).not.toBeNull();
    expect(svg?.querySelector("[data-fill]")).toBeNull();
  });

  it("omits the spread label when the evidence has none", () => {
    expect(draw(20, 60, null)?.textContent).not.toContain("Spread");
  });
});
