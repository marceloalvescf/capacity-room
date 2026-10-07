import type { TFunction } from "i18next";
import type { Language } from "@/i18n/language";

export const quickRanges = ["1h", "6h", "24h", "7d", "15d", "30d"] as const;
type QuickRange = (typeof quickRanges)[number];

const quickKeys: Record<QuickRange, string> = {
  "1h": "period.quick.1h",
  "6h": "period.quick.6h",
  "24h": "period.quick.24h",
  "7d": "period.quick.7d",
  "15d": "period.quick.15d",
  "30d": "period.quick.30d",
};

export const calendarRanges = ["today", "yesterday", "thisWeek", "previousWeek", "thisMonth", "previousMonth"] as const;
export type CalendarRange = (typeof calendarRanges)[number];

export type PeriodValue =
  | { kind: "length"; period: string }
  | { kind: "calendar"; name: CalendarRange }
  | { kind: "range"; start: string; end: string };

export type RangeProblem = "invalid" | "order" | "future";

const units = { m: "minute", h: "hour", d: "day" } as const;

export function parsePeriod(input: string): string | null {
  const trimmed = input.trim();
  return /^[1-9][0-9]*[mhd]$/.test(trimmed) ? trimmed : null;
}

const unitDays = { m: 1 / 1440, h: 1 / 24, d: 1 } as const;

function lengthDays(period: string): number | null {
  if (parsePeriod(period) !== period) {
    return null;
  }
  return Number(period.slice(0, -1)) * unitDays[period.slice(-1) as keyof typeof unitDays];
}

const wallShape = /^(\d{4})-(\d{2})-(\d{2}) (\d{2}):(\d{2})$/;

function pad(n: number, width = 2): string {
  return String(n).padStart(width, "0");
}

function wallText(ms: number): string {
  const d = new Date(ms);
  return `${pad(d.getUTCFullYear(), 4)}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())} ${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}`;
}

// A wall string read as if it were UTC, in milliseconds; null outside the grammar.
function wallMs(wall: string): number | null {
  const m = wallShape.exec(wall);
  if (m === null) {
    return null;
  }
  const [year, month, day, hour, minute] = m.slice(1).map(Number) as [number, number, number, number, number];
  const ms = Date.UTC(year, month - 1, day, hour, minute);
  return wallText(ms) === wall ? ms : null;
}

// The wall time of the grammar, trimmed, or null when it is not a real date and time.
export function parseWallTime(input: string): string | null {
  const trimmed = input.trim();
  return wallMs(trimmed) === null ? null : trimmed;
}

// The zone's wall clock at an instant, floored to the minute.
export function wallNow(zone: string, now: Date): string {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: zone,
    hourCycle: "h23",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).formatToParts(now);
  const get = (type: string) => parts.find((part) => part.type === type)?.value ?? "";
  return `${get("year")}-${get("month")}-${get("day")} ${get("hour")}:${get("minute")}`;
}

const dayMs = 86_400_000;

export function resolveCalendar(name: CalendarRange, zone: string, now: Date): { start: string; end: string } {
  const end = wallNow(zone, now);
  const nowMs = wallMs(end) as number;
  const today = Math.floor(nowMs / dayMs) * dayMs;
  const sinceMonday = (new Date(today).getUTCDay() + 6) % 7;
  const monday = today - sinceMonday * dayMs;
  const first = new Date(today);
  const thisMonth = Date.UTC(first.getUTCFullYear(), first.getUTCMonth(), 1);
  const previousMonth = Date.UTC(first.getUTCFullYear(), first.getUTCMonth() - 1, 1);
  const bounds: Record<CalendarRange, [number, number]> = {
    today: [today, nowMs],
    yesterday: [today - dayMs, today],
    thisWeek: [monday, nowMs],
    previousWeek: [monday - 7 * dayMs, monday],
    thisMonth: [thisMonth, nowMs],
    previousMonth: [previousMonth, thisMonth],
  };
  const [from, to] = bounds[name];
  return { start: wallText(from), end: wallText(to) };
}

// Wall strings of the grammar compare as text.
export function checkRange(
  start: string,
  end: string,
  zone: string,
  now: Date,
): { start?: RangeProblem; end?: RangeProblem } {
  const problems: { start?: RangeProblem; end?: RangeProblem } = {};
  if (parseWallTime(start) === null) {
    problems.start = "invalid";
  }
  if (parseWallTime(end) === null) {
    problems.end = "invalid";
  }
  if (problems.start !== undefined || problems.end !== undefined) {
    return problems;
  }
  if (start >= end) {
    return { start: "order" };
  }
  if (end > wallNow(zone, now)) {
    return { end: "future" };
  }
  return {};
}

// The length of a period in days, or null when it has none: a length outside
// the grammar, or a calendar name, whose span depends on the day it is read.
export function periodDays(value: PeriodValue): number | null {
  if (value.kind === "length") {
    return lengthDays(value.period);
  }
  if (value.kind === "calendar") {
    return null;
  }
  const start = wallMs(value.start);
  const end = wallMs(value.end);
  return start === null || end === null ? null : (end - start) / dayMs;
}

function isQuick(period: string): period is QuickRange {
  return quickRanges.some((range) => range === period);
}

function lengthText(t: TFunction, language: Language, period: string): string {
  if (isQuick(period)) {
    return t(quickKeys[period], { lng: language });
  }
  if (parsePeriod(period) !== period) {
    return period;
  }
  const unit = units[period.slice(-1) as keyof typeof units];
  const duration = new Intl.NumberFormat(language, { style: "unit", unit, unitDisplay: "long" }).format(
    Number(period.slice(0, -1)),
  );
  return t("period.custom.value", { lng: language, duration });
}

export function periodText(t: TFunction, language: Language, value: PeriodValue): string {
  switch (value.kind) {
    case "length":
      return lengthText(t, language, value.period);
    case "calendar":
      return t(`period.calendar.${value.name}`, { lng: language });
    case "range":
      return t("period.range.value", { lng: language, start: value.start, end: value.end });
  }
}
