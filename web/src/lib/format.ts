const pad = (value: number, width = 2): string => String(value).padStart(width, "0");

const none = "-";

export function formatDate(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return none;
  }
  return `${String(date.getFullYear())}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

export function formatTime(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return none;
  }
  return `${formatDate(iso)} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

export function formatTimeSeconds(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return none;
  }
  return `${formatTime(iso)}:${pad(date.getSeconds())}`;
}

// The time as formatTime shows it, in a named zone instead of the browser's.
export function formatTimeIn(iso: string, timeZone: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return none;
  }
  const parts = new Map(
    new Intl.DateTimeFormat("en-US", {
      timeZone,
      hourCycle: "h23",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    })
      .formatToParts(date)
      .map((part) => [part.type, part.value]),
  );
  return `${parts.get("year") ?? ""}-${parts.get("month") ?? ""}-${parts.get("day") ?? ""} ${parts.get("hour") ?? ""}:${parts.get("minute") ?? ""}`;
}

export function formatElapsed(ms: number): string {
  // A clock ahead of the browser's gives a negative span; it shows as zero.
  const total = Number.isFinite(ms) ? Math.max(0, Math.floor(ms / 1000)) : 0;
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = total % 60;
  return hours > 0 ? `${String(hours)}:${pad(minutes)}:${pad(seconds)}` : `${String(minutes)}:${pad(seconds)}`;
}

const ageUnits = [
  ["week", 7 * 24 * 60 * 60 * 1000],
  ["day", 24 * 60 * 60 * 1000],
  ["hour", 60 * 60 * 1000],
  ["minute", 60 * 1000],
] as const;

// The age of a moment as the largest whole unit among minute, hour, day, and week.
export function formatAge(iso: string, now: number, language: string): string {
  const span = Math.max(0, now - Date.parse(iso));
  const formatter = new Intl.RelativeTimeFormat(language, { numeric: "always", style: "short" });
  for (const [unit, ms] of ageUnits) {
    if (span >= ms) {
      return formatter.format(-Math.floor(span / ms), unit);
    }
  }
  return formatter.format(-0, "minute");
}
