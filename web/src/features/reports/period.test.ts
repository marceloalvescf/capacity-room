import { describe, expect, it } from "vitest";
import { i18next } from "@/i18n/i18n";
import { parsePeriod, periodDays, periodText } from "./period";

describe("period", () => {
  it("period grammar", () => {
    expect(parsePeriod("36h")).toBe("36h");
    expect(parsePeriod(" 36h ")).toBe("36h");
    for (const bad of ["0h", "01h", "1H", "1.5h", "36", "h", ""]) {
      expect({ bad, parsed: parsePeriod(bad) }).toEqual({ bad, parsed: null });
    }
  });

  it("period words", async () => {
    await i18next.changeLanguage("en");
    expect(periodText(i18next.t, "en", "7d")).toBe("Last 7 days");
    expect(periodText(i18next.t, "en", "36h")).toBe("Last 36 hours");
    await i18next.changeLanguage("pt-BR");
    expect(periodText(i18next.t, "pt-BR", "36h")).toBe("36 horas até agora");
    expect(periodText(i18next.t, "pt-BR", "1h")).toBe("Última hora");
  });

  it("a period outside the grammar is shown as it came", async () => {
    await i18next.changeLanguage("en");
    for (const odd of ["P1D", "0h", "", "36"]) {
      expect(periodText(i18next.t, "en", odd)).toBe(odd);
    }
  });

  it("a period's length in days", () => {
    expect(periodDays("90m")).toBe(0.0625);
    expect(periodDays("36h")).toBe(1.5);
    expect(periodDays("15d")).toBe(15);
    expect(periodDays("360h")).toBe(15);
    expect(periodDays("361h")).toBeGreaterThan(15);
    expect(periodDays("1.5h")).toBeNull();
  });
});
