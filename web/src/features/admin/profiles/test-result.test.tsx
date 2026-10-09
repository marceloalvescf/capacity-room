import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { ApiFailure } from "@/api/client";
import { i18next } from "@/i18n/i18n";
import { TestResultLine } from "./test-result";

afterEach(async () => {
  await i18next.changeLanguage("en");
});

describe("the test result line", () => {
  it("shows nothing without an outcome", () => {
    const { container } = render(<TestResultLine outcome={null} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("words a refused draft, muted", () => {
    render(<TestResultLine outcome={{ kind: "refused" }} />);
    expect(screen.getByText("Correct the marked fields, then test again.")).toHaveClass("text-muted-fg");
  });

  it("words a failed call in the language of the moment", async () => {
    const outcome = { kind: "failed", error: new ApiFailure(500, "INTERNAL_ERROR", null) } as const;
    const { rerender } = render(<TestResultLine outcome={outcome} />);
    expect(screen.getByText(i18next.t("error.INTERNAL_ERROR"))).toBeInTheDocument();
    await i18next.changeLanguage("pt-BR");
    rerender(<TestResultLine outcome={outcome} />);
    expect(screen.getByText(i18next.t("error.INTERNAL_ERROR", { lng: "pt-BR" }))).toBeInTheDocument();
    expect(i18next.t("error.INTERNAL_ERROR", { lng: "pt-BR" })).not.toBe(i18next.t("error.INTERNAL_ERROR", { lng: "en" }));
  });
});
