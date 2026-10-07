import { describe, expect, it } from "vitest";
import { pdfMark, reportMark } from "./marks";

describe("marks", () => {
  it("maps the statuses as R194 says", () => {
    expect(reportMark({ status: "queued", expired: false })).toBe("undefined");
    expect(reportMark({ status: "running", expired: false })).toBe("undefined");
    expect(reportMark({ status: "collected", expired: false })).toBe("healthy");
    expect(reportMark({ status: "failed", expired: false })).toBe("critical");
    expect(reportMark({ status: "collected", expired: true })).toBe("unavailable");
    expect(pdfMark("pending")).toBe("undefined");
    expect(pdfMark("ready")).toBe("healthy");
    expect(pdfMark("failed")).toBe("critical");
  });
});
