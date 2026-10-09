import { z } from "zod";

// A new password: 12 to 1024 code points, worded by locale keys.
export const newPassword = z
  .string()
  .min(1, "validation.required")
  .refine((value) => [...value].length >= 12, "validation.passwordShort")
  .refine((value) => [...value].length <= 1024, "validation.passwordLong");
