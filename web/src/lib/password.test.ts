import { describe, expect, it } from "vitest";
import { newPassword } from "./password";

function message(value: string): string | undefined {
  const parsed = newPassword.safeParse(value);
  return parsed.success ? undefined : parsed.error.issues[0]?.message;
}

describe("a new password", () => {
  it("counts code points, as the server counts runes", () => {
    expect(message("😀".repeat(11))).toBe("validation.passwordShort");
    expect(message("😀".repeat(12))).toBeUndefined();
    expect(message("😀".repeat(1024))).toBeUndefined();
    expect(message("😀".repeat(1025))).toBe("validation.passwordLong");
  });

  it("asks for a value first", () => {
    expect(message("")).toBe("validation.required");
  });
});
