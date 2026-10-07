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

const units = { m: "minute", h: "hour", d: "day" } as const;

export function parsePeriod(input: string): string | null {
  const trimmed = input.trim();
  return /^[1-9][0-9]*[mhd]$/.test(trimmed) ? trimmed : null;
}

const unitDays = { m: 1 / 1440, h: 1 / 24, d: 1 } as const;

// The length of a period in days, or null when it is outside the grammar.
export function periodDays(period: string): number | null {
  if (parsePeriod(period) !== period) {
    return null;
  }
  return Number(period.slice(0, -1)) * unitDays[period.slice(-1) as keyof typeof unitDays];
}

function isQuick(period: string): period is QuickRange {
  return quickRanges.some((range) => range === period);
}

export function periodText(t: TFunction, language: Language, period: string): string {
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
