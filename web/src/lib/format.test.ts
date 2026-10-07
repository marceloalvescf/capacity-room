import { afterEach, describe, expect, it } from "vitest";
import { formatDate, formatElapsed, formatTime } from "./format";

const zone = process.env.TZ;
afterEach(() => {
  if (zone === undefined) {
    delete process.env.TZ;
  } else {
    process.env.TZ = zone;
  }
});

describe("format", () => {
  it("shows the time in the browser's time zone and the elapsed time as m:ss or h:mm:ss", () => {
    process.env.TZ = "America/Sao_Paulo";
    expect(formatTime("2026-10-07T17:02:00Z")).toBe("2026-10-07 14:02");
    expect(formatDate("2026-11-06T12:00:00Z")).toBe("2026-11-06");
    expect(formatElapsed(42_000)).toBe("0:42");
    expect(formatElapsed(3_725_000)).toBe("1:02:05");
  });

  it("clamps an elapsed time that is negative or not a number to 0:00", () => {
    expect(formatElapsed(-1500)).toBe("0:00");
    expect(formatElapsed(Number.NaN)).toBe("0:00");
  });
});
