import { z } from "zod";

const codePoints = (value: string) => Array.from(value).length;

// A new password: 12 to 1024 code points, worded by locale keys.
export const newPassword = z
  .string()
  .min(1, "validation.required")
  .refine((value) => codePoints(value) >= 12, "validation.passwordShort")
  .refine((value) => codePoints(value) <= 1024, "validation.passwordLong");
