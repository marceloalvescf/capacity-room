import { describe, expect, it } from "vitest";
import { i18next } from "@/i18n/i18n";
import {
  calendarRanges,
  checkRange,
  parsePeriod,
  parseWallTime,
  periodDays,
  periodText,
  resolveCalendar,
  wallNow,
  type CalendarRange,
} from "./period";

const zone = "America/Sao_Paulo";
const wed = new Date("2026-10-07T21:00:42Z");

function len(period: string) {
  return { kind: "length", period } as const;
}

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
    expect(periodText(i18next.t, "en", len("7d"))).toBe("Last 7 days");
    expect(periodText(i18next.t, "en", len("36h"))).toBe("Last 36 hours");
    await i18next.changeLanguage("pt-BR");
    expect(periodText(i18next.t, "pt-BR", len("36h"))).toBe("36 horas até agora");
    expect(periodText(i18next.t, "pt-BR", len("1h"))).toBe("Última hora");
  });

  it("a period outside the grammar is shown as it came", async () => {
    await i18next.changeLanguage("en");
    for (const odd of ["P1D", "0h", "", "36"]) {
      expect(periodText(i18next.t, "en", len(odd))).toBe(odd);
    }
  });

  it("a period's length in days", () => {
    expect(periodDays(len("90m"))).toBe(0.0625);
    expect(periodDays(len("36h"))).toBe(1.5);
    expect(periodDays(len("15d"))).toBe(15);
    expect(periodDays(len("360h"))).toBe(15);
    expect(periodDays(len("361h"))).toBeGreaterThan(15);
    expect(periodDays(len("1.5h"))).toBeNull();
  });

  it("wall time grammar", () => {
    expect(parseWallTime("2026-09-01 00:00")).toBe("2026-09-01 00:00");
    expect(parseWallTime(" 2026-09-01 00:00 ")).toBe("2026-09-01 00:00");
    for (const bad of [
      "2026-9-01 00:00",
      "2026-09-01T00:00",
      "2026-09-01 24:00",
      "2026-09-01 00:60",
      "2026-02-30 00:00",
      "2026-09-01 00:00:00",
      "",
    ]) {
      expect({ bad, parsed: parseWallTime(bad) }).toEqual({ bad, parsed: null });
    }
  });

  it("wall now floors the minute", () => {
    expect(wallNow(zone, wed)).toBe("2026-10-07 18:00");
  });

  it("each calendar range", () => {
    const want: Record<CalendarRange, { start: string; end: string }> = {
      today: { start: "2026-10-07 00:00", end: "2026-10-07 18:00" },
      yesterday: { start: "2026-10-06 00:00", end: "2026-10-07 00:00" },
      thisWeek: { start: "2026-10-05 00:00", end: "2026-10-07 18:00" },
      previousWeek: { start: "2026-09-28 00:00", end: "2026-10-05 00:00" },
      thisMonth: { start: "2026-10-01 00:00", end: "2026-10-07 18:00" },
      previousMonth: { start: "2026-09-01 00:00", end: "2026-10-01 00:00" },
    };
    for (const name of calendarRanges) {
      expect({ name, range: resolveCalendar(name, zone, wed) }).toEqual({ name, range: want[name] });
    }
  });

  it("a Monday and a Sunday", () => {
    for (const now of [new Date("2026-10-05T15:00:00Z"), new Date("2026-10-11T15:00:00Z")]) {
      expect(resolveCalendar("thisWeek", zone, now).start).toBe("2026-10-05 00:00");
      expect(resolveCalendar("previousWeek", zone, now)).toEqual({
        start: "2026-09-28 00:00",
        end: "2026-10-05 00:00",
      });
    }
  });

  it("year boundary", () => {
    const now = new Date("2026-01-03T13:00:00Z");
    expect(wallNow(zone, now)).toBe("2026-01-03 10:00");
    expect(resolveCalendar("previousMonth", zone, now)).toEqual({
      start: "2025-12-01 00:00",
      end: "2026-01-01 00:00",
    });
    expect(resolveCalendar("previousWeek", zone, now)).toEqual({
      start: "2025-12-22 00:00",
      end: "2025-12-29 00:00",
    });
  });

  it("resolve in the report's zone", () => {
    expect(new Date("2026-10-08T01:30:00Z").getDate()).toBe(8);
    expect(resolveCalendar("yesterday", zone, new Date("2026-10-08T01:30:00Z"))).toEqual({
      start: "2026-10-06 00:00",
      end: "2026-10-07 00:00",
    });
  });

  it("check a range", () => {
    const a = "2026-09-01 00:00";
    expect(checkRange(a, "2026-09-02 00:00", zone, wed)).toEqual({});
    expect(checkRange("2026-09-02 00:00", a, zone, wed)).toEqual({ start: "order" });
    expect(checkRange(a, a, zone, wed)).toEqual({ start: "order" });
    expect(checkRange(a, "2026-10-07 18:01", zone, wed)).toEqual({ end: "future" });
    expect(checkRange(a, "2026-10-07 18:00", zone, wed)).toEqual({});
    expect(checkRange("bad", "2026-09-02 00:00", zone, wed)).toEqual({ start: "invalid" });
    expect(checkRange(a, "bad", zone, wed)).toEqual({ end: "invalid" });
  });

  it("period text", async () => {
    const range = { kind: "range", start: "2026-09-01 00:00", end: "2026-10-01 00:00" } as const;
    await i18next.changeLanguage("en");
    expect(periodText(i18next.t, "en", len("7d"))).toBe("Last 7 days");
    expect(periodText(i18next.t, "en", len("36h"))).toBe("Last 36 hours");
    expect(periodText(i18next.t, "en", { kind: "calendar", name: "thisWeek" })).toBe("This week so far");
    expect(periodText(i18next.t, "en", range)).toBe("2026-09-01 00:00 to 2026-10-01 00:00");
    await i18next.changeLanguage("pt-BR");
    expect(periodText(i18next.t, "pt-BR", { kind: "calendar", name: "previousMonth" })).toBe("Mês anterior");
    expect(periodText(i18next.t, "pt-BR", range)).toBe("2026-09-01 00:00 a 2026-10-01 00:00");
  });

  it("period days", () => {
    expect(periodDays(len("30d"))).toBe(30);
    expect(periodDays({ kind: "range", start: "2026-09-01 00:00", end: "2026-10-01 00:00" })).toBe(30);
    const { start, end } = resolveCalendar("previousMonth", zone, wed);
    expect(periodDays({ kind: "range", start, end })).toBe(30);
    expect(periodDays({ kind: "calendar", name: "previousMonth" })).toBeNull();
  });
});
