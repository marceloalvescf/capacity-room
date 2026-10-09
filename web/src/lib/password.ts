import { z } from "zod";

// A new password: 12 to 1024 characters, worded by locale keys.
export const newPassword = z
  .string()
  .min(1, "validation.required")
  .min(12, "validation.passwordShort")
  .max(1024, "validation.passwordLong");
