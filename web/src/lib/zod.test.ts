import { expect, it } from "vitest";
import { z } from "zod";
import "@/lib/zod";

it("turns the schema compiler off", () => {
  expect(z.config().jitless).toBe(true);
});
