import { describe, expect, it } from "vitest";
import { safeNext } from "./session";

describe("safeNext", () => {
  it("keeps a local path", () => {
    expect(safeNext("/account")).toBe("/account");
    expect(safeNext("/account?x=1#y")).toBe("/account?x=1#y");
  });

  it("sends everything else to the start page", () => {
    for (const raw of [
      "//evil.example",
      "/\\evil.example",
      "https://evil.example",
      "javascript:alert(1)",
      "%2F%2Fevil.example",
      "/\t/evil.example",
      "/\n/evil.example",
      "",
      null,
    ]) {
      expect(safeNext(raw)).toBe("/");
    }
  });
});
