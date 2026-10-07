import { describe, expect, it } from "vitest";
import { i18next } from "@/i18n/i18n";
import { languageValues } from "@/api/schema.gen";
import { schema as accountSchema } from "./account/account-page";
import { schema as loginSchema } from "./auth/login-page";

function messages(result: { success: boolean; error?: { issues: { message: string }[] } }): string[] {
  return result.error?.issues.map((issue) => issue.message) ?? [];
}

describe("form schemas", () => {
  it("every issue a form can produce is a catalog key in both languages", () => {
    const long = "x".repeat(1025);
    const issues = [
      ...messages(loginSchema.safeParse({ username: "", password: "" })),
      ...messages(loginSchema.safeParse({ username: "a", password: long })),
      ...messages(accountSchema.safeParse({ current: "", next: "", repeat: "" })),
      ...messages(accountSchema.safeParse({ current: long, next: "short", repeat: "other" })),
      ...messages(accountSchema.safeParse({ current: "a", next: long, repeat: long })),
      ...messages(accountSchema.safeParse({ current: "a", next: "twelve-chars-x", repeat: "twelve-chars-y" })),
    ];
    expect(issues.length).toBeGreaterThan(0);
    for (const message of issues) {
      for (const lng of languageValues) {
        expect({ message, lng, found: i18next.exists(message, { lng }) }).toEqual({ message, lng, found: true });
      }
    }
  });
});
