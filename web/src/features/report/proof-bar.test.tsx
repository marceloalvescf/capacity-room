import { render } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import type { components } from "@/api/schema.gen";
import { i18next } from "@/i18n/i18n";
import viewEn from "../../../tests/fixtures/view/edge-scaling.en.json";
import type { ViewFinding } from "./api";
import { FindingProof, ProofBar } from "./proof-bar";

type View = components["schemas"]["ReportView"];

function finding(code: string): ViewFinding {
  const found = (viewEn as View).findings.find((item) => item.code === code);
  if (found === undefined) {
    throw new Error(`no finding ${code} in the golden view`);
  }
  return found;
}

function bar(element: Parameters<typeof ProofBar>[0]) {
  const { container } = render(<ProofBar {...element} />);
  return container.querySelector("svg");
}

beforeEach(async () => {
  await i18next.changeLanguage("en");
});

describe("proof bar", () => {
  it("is a drawing hidden from assistive technology, its fill growing under motion-safe only", () => {
    const svg = bar({ value: 95, reference: 100, status: "critical" });
    expect(svg).toHaveAttribute("aria-hidden", "true");
    expect(svg?.querySelector("[data-fill]")?.parentElement).toHaveClass("motion-safe:animate-proof");
    expect(svg?.querySelector("[data-fill]")?.parentElement?.getAttribute("class")).not.toMatch(/(^|\s)animate-proof/);
  });

  it("fills to its value on a scale that ends at the reference, with no tick when the value stays within it", () => {
    const svg = bar({ value: 95, reference: 100, status: "critical" });
    const fill = svg?.querySelector("[data-fill]");
    expect(fill).toHaveAttribute("x", "0");
    expect(fill).toHaveAttribute("width", "950");
    expect(svg?.querySelector("[data-reference]")).toBeNull();
  });

  it("runs past its reference at half height, the fill ending at the tick of the reference", () => {
    const svg = bar({ value: 4, reference: 3, status: "critical" });
    expect(svg?.querySelector("[data-fill]")).toHaveAttribute("width", "750");
    const overrun = svg?.querySelector("[data-overrun]");
    expect(overrun).toHaveAttribute("x", "750");
    expect(overrun).toHaveAttribute("width", "250");
    expect(overrun).toHaveAttribute("y", "6");
    expect(overrun).toHaveAttribute("height", "4");
    expect(overrun).toHaveClass("fill-critical");
    expect(svg?.querySelector("[data-reference]")).toHaveAttribute("x1", "750");
    expect(svg?.querySelector("[data-break]")).toBeNull();
  });

  it("draws the tick of the reference when the value reaches it, with no overrun", () => {
    const svg = bar({ value: 3, reference: 3, status: "attention" });
    expect(svg?.querySelector("[data-reference]")).toHaveAttribute("x1", "1000");
    expect(svg?.querySelector("[data-fill]")).toHaveAttribute("width", "1000");
    expect(svg?.querySelector("[data-overrun]")).toBeNull();
    expect(svg?.querySelector("[data-break]")).toBeNull();
  });

  it("scales to the value up to four times the reference, with no break within it", () => {
    const svg = bar({ value: 330, reference: 100, status: "critical" });
    expect(svg?.querySelector("[data-fill]")).toHaveAttribute("width", "303.030303030303");
    expect(svg?.querySelector("[data-reference]")).toHaveAttribute("x1", "303.030303030303");
    const overrun = svg?.querySelector("[data-overrun]");
    expect(overrun).toHaveAttribute("x", "303.030303030303");
    expect(overrun).toHaveAttribute("width", "696.969696969697");
    expect(svg?.querySelector("[data-break]")).toBeNull();
    expect(bar({ value: 400, reference: 100, status: "critical" })?.querySelector("[data-break]")).toBeNull();
  });

  it("ends the scale at four times the reference and breaks the bar past it", () => {
    const svg = bar({ value: 12000, reference: 100, status: "critical" });
    expect(svg?.querySelector("[data-fill]")).toHaveAttribute("width", "250");
    expect(svg?.querySelector("[data-reference]")).toHaveAttribute("x1", "250");
    const overrun = svg?.querySelector("[data-overrun]");
    expect(overrun).toHaveAttribute("x", "250");
    expect(overrun).toHaveAttribute("width", "750");
    expect(svg?.querySelector("[data-break]")).not.toBeNull();
  });

  it("takes the colour of the status it is given, and ink for any other or none", () => {
    const fillOf = (status: Parameters<typeof ProofBar>[0]["status"]) =>
      bar({ value: 1, reference: 2, status })?.querySelector("[data-fill]")?.getAttribute("class");
    expect(fillOf("critical")).toBe("fill-critical");
    expect(fillOf("attention")).toBe("fill-attention");
    expect(fillOf("healthy")).toBe("fill-healthy");
    expect(fillOf("unavailable")).toBe("fill-neatline");
    expect(fillOf(null)).toBe("fill-neatline");
  });

  it("divides a count per unit and marks its floor; a long count is not divided", () => {
    const svg = bar({ value: 3, reference: 3, status: "attention", units: true, floor: 2 });
    expect(svg?.querySelectorAll("[data-unit]")).toHaveLength(2);
    expect(svg?.querySelector("[data-floor]")).toHaveAttribute("x1", "666.6666666666666");
    expect(bar({ value: 41, reference: 41, status: null, units: true })?.querySelectorAll("[data-unit]")).toHaveLength(0);
    expect(bar({ value: 3, reference: 3, status: null })?.querySelectorAll("[data-unit]")).toHaveLength(0);
  });

  it("marks no floor at none or at the end of the scale", () => {
    expect(bar({ value: 2, reference: 3, status: null, units: true, floor: 0 })?.querySelector("[data-floor]")).toBeNull();
    expect(bar({ value: 2, reference: 3, status: null, units: true, floor: 3 })?.querySelector("[data-floor]")).toBeNull();
    expect(bar({ value: 2, reference: 3, status: null, units: true, floor: null })?.querySelector("[data-floor]")).toBeNull();
  });

  it("draws nothing on an empty scale", () => {
    expect(bar({ value: 0, reference: 0, status: null })).toBeNull();
  });
});

describe("finding proof", () => {
  it("prints the leading evidence, then the rest, beside the bar of the finding's severity", () => {
    const { container } = render(<FindingProof finding={finding("NAMESPACE_CPU_LIMIT_CRITICAL")} />);
    expect(container.querySelector("p")).toHaveTextContent("Peak330% · Average220%");
    expect(container.querySelector("[data-fill]")).toHaveClass("fill-critical");
    expect(container.querySelector("[data-reference]")).not.toBeNull();
    expect(container.querySelector("[data-overrun]")).not.toBeNull();
    expect(container.querySelector("[data-break]")).toBeNull();
    expect(container.querySelectorAll("[data-unit]")).toHaveLength(0);
  });

  it("divides the bar of a count per unit and not that of a percent", () => {
    const { container } = render(<FindingProof finding={finding("HPA_MAX_REPLICAS_REACHED")} />);
    expect(container.querySelectorAll("[data-unit]")).toHaveLength(3);
    expect(container.querySelector("[data-reference]")).toHaveAttribute("x1", "1000");
  });

  it("draws an uneven pool as a range, its numbers printed above and no fill from zero", () => {
    const { container } = render(<FindingProof finding={finding("NODEPOOL_CPU_UNEVEN")} />);
    expect(container.querySelector("[data-low]")).not.toBeNull();
    expect(container.querySelector("[data-high]")).not.toBeNull();
    expect(container.querySelector("[data-fill]")).toBeNull();
    expect(container.querySelector("p")?.textContent).toContain("Highest peak60%");
  });

  it("shows nothing for a finding with nothing to draw", () => {
    const { container } = render(<FindingProof finding={finding("DEPLOYMENT_WITHOUT_HPA")} />);
    expect(container).toBeEmptyDOMElement();
  });
});
