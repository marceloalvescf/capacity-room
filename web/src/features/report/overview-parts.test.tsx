import { render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";
import "@/i18n/i18n";
import view from "../../../tests/fixtures/view/direct-complete.en.json";
import type { ReportView } from "./api";
import { FindingsChart } from "./findings-chart";
import { AnswerStrip, Plan } from "./overview-parts";

const golden = view as ReportView;
const base = "/reports/abc";

function hrefs(): string[] {
  return screen.getAllByRole("link").map((link) => link.getAttribute("href") ?? "");
}

describe("the overview's parts", () => {
  it("AnswerStrip links a severity under the base", () => {
    render(
      <MemoryRouter>
        <AnswerStrip view={golden} base={base} />
      </MemoryRouter>,
    );
    expect(hrefs()).toContain("/reports/abc/findings?severity=critical");
    expect(hrefs()).toContain("/reports/abc/findings");
  });

  it("Plan with a limit shows that many rows and a link to the overview", () => {
    render(
      <MemoryRouter>
        <Plan view={golden} base={base} limit={3} />
      </MemoryRouter>,
    );
    expect(within(screen.getByRole("table")).getAllByRole("row")).toHaveLength(4);
    expect(screen.getByRole("link", { name: "See the action plan" }).getAttribute("href")).toBe("/reports/abc/overview");
  });

  it("FindingsChart links a scope's sentence under the base", () => {
    render(
      <MemoryRouter>
        <FindingsChart view={golden} base={base} linkSegments={false} />
      </MemoryRouter>,
    );
    expect(hrefs()).toContain("/reports/abc/findings?scope=namespace&finding=0");
  });
});
