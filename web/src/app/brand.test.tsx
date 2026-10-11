import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import "@/i18n/i18n";
import { Brand } from "./brand";

describe("brand", () => {
  it("names the product beside a disc hidden from assistive technology", () => {
    const { container } = render(<Brand />);
    expect(screen.getByText("CapacityRoom")).toHaveClass("font-bold");
    const svg = container.querySelector("svg");
    expect(svg).toHaveAttribute("aria-hidden", "true");
    expect(svg?.querySelector("circle")).toHaveAttribute("stroke", "currentColor");
    expect(svg?.querySelector("line")).toHaveClass("stroke-accent");
  });
});
