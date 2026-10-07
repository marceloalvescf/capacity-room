import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { StatusMark, statusValues, type Status } from "@/components/status-mark";
import { i18next } from "@/i18n/i18n";
import en from "@/locales/en.json";
import ptBR from "@/locales/pt-BR.json";

const shapes: Record<Status, (svg: SVGSVGElement) => boolean> = {
  healthy: (svg) => svg.querySelector("circle")?.getAttribute("fill") === "currentColor",
  attention: (svg) => svg.querySelector("polygon")?.getAttribute("points")?.split(" ").length === 3,
  critical: (svg) => svg.querySelector("polygon")?.getAttribute("points")?.split(" ").length === 4,
  undefined: (svg) => svg.querySelector("circle")?.getAttribute("fill") === "none",
  unavailable: (svg) => svg.querySelector("line") !== null,
};

describe("status mark", () => {
  for (const [lang, file] of [
    ["en", en],
    ["pt-BR", ptBR],
  ] as const) {
    for (const status of statusValues) {
      it(`${status} in ${lang}`, async () => {
        await i18next.changeLanguage(lang);
        const { container } = render(<StatusMark status={status} />);
        const svg = container.querySelector("svg");
        expect(svg).not.toBeNull();
        expect(svg?.getAttribute("aria-hidden")).toBe("true");
        expect(shapes[status](svg as SVGSVGElement)).toBe(true);
        const label = container.querySelector("svg + span");
        expect(label?.textContent).toBe(file.status[status]);
        expect(label?.classList.contains("font-bold")).toBe(status === "critical");
      });
    }
  }

  it("takes an explicit label", () => {
    const { container } = render(<StatusMark status="healthy" label="42" />);
    expect(container.querySelector("svg + span")?.textContent).toBe("42");
  });
});
