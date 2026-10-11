import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DepthGauge, type GaugeSeverity } from "./depth-gauge";

function gauge(current: number, max: number, min: number | null = null, severity: GaugeSeverity = "critical") {
  const { container } = render(<DepthGauge current={current} max={max} min={min} severity={severity} />);
  const svg = container.querySelector("svg") as SVGSVGElement;
  return {
    svg,
    bands: [...svg.querySelectorAll("[data-band]")],
    labels: [...svg.querySelectorAll("text")].map((node) => node.textContent),
  };
}

describe("depth gauge", () => {
  it("draws one band per node up to the maximum, the bands in use tinted deeper toward it", () => {
    const { svg, bands, labels } = gauge(4, 6);
    expect(svg).toHaveAttribute("aria-hidden", "true");
    expect(bands).toHaveLength(6);
    expect(bands.map((band) => band.getAttribute("class"))).toEqual([
      "fill-depth-1",
      "fill-depth-1",
      "fill-depth-2",
      "fill-depth-2",
      "fill-transparent",
      "fill-transparent",
    ]);
    expect(labels).toEqual(["0", "1", "2", "3", "4", "5", "6"]);
    expect(svg.querySelector("[data-hazard]")).toBeNull();
  });

  it("a full scale under a critical cluster finding hatches the last band in the critical colour", () => {
    const { svg, bands } = gauge(3, 3, null, "critical");
    expect(bands).toHaveLength(3);
    const hazard = svg.querySelectorAll("[data-hazard]");
    expect(hazard).toHaveLength(1);
    expect(hazard[0]).toBe(bands[2]);
    expect(hazard[0]?.getAttribute("fill")).toMatch(/^url\(#hazard-[a-zA-Z0-9]+\)$/);
    expect(svg.querySelector("pattern line")).toHaveClass("stroke-critical");
    expect(svg.querySelector("[data-closing]")).toHaveClass("stroke-critical");
  });

  it("a full scale under an attention cluster finding hatches the last band in the attention colour", () => {
    const { svg } = gauge(3, 3, null, "attention");
    expect(svg.querySelectorAll("[data-hazard]")).toHaveLength(1);
    expect(svg.querySelector("pattern line")).toHaveClass("stroke-attention");
    expect(svg.querySelector("[data-closing]")).toHaveClass("stroke-attention");
    expect(svg.querySelector(".stroke-critical")).toBeNull();
  });

  it("a full scale with no cluster finding is plain depth closed by a neat-line, never hatched by the numbers", () => {
    const { svg, bands } = gauge(3, 3, null, null);
    expect(svg.querySelector("[data-hazard]")).toBeNull();
    expect(svg.querySelector("pattern")).toBeNull();
    expect(bands.map((band) => band.getAttribute("class"))).toEqual(["fill-depth-1", "fill-depth-2", "fill-depth-3"]);
    expect(svg.querySelector("[data-closing]")).toHaveClass("stroke-neatline");
  });

  it("room left draws no closing line and no hatch, whatever the severity", () => {
    const { svg } = gauge(2, 3, null, "critical");
    expect(svg.querySelector("[data-hazard]")).toBeNull();
    expect(svg.querySelector("[data-closing]")).toBeNull();
  });

  it("sounds once under motion-safe: the level rises, then the hazard shows", () => {
    const { svg, bands } = gauge(3, 3);
    const rise = bands[0]?.parentElement;
    expect(rise).toHaveClass("motion-safe:animate-sounding", "origin-left");
    expect(bands[1]?.parentElement).toBe(rise);
    expect(svg.querySelector("[data-hazard]")?.parentElement).toHaveClass("motion-safe:animate-hazard-in");
    expect(svg.querySelector(".stroke-critical:not(pattern *)")).toHaveClass("motion-safe:animate-hazard-in");
  });

  it("marks the minimum with a dashed contour when it is known", () => {
    expect(gauge(2, 5, 2).svg.querySelector("[data-contour]")).toHaveAttribute("stroke-dasharray", "2 2");
    expect(gauge(2, 5, null).svg.querySelector("[data-contour]")).toBeNull();
  });

  it("labels at most eleven soundings above twelve nodes, the maximum always among them", () => {
    expect(gauge(10, 40).labels).toEqual(["0", "4", "8", "12", "16", "20", "24", "28", "32", "36", "40"]);
    expect(gauge(10, 25).labels).toEqual(["0", "3", "6", "9", "12", "15", "18", "21", "25"]);
  });
});
