import type { ComponentProps } from "react";
import { cn } from "@/lib/cn";

type Variant = "primary" | "secondary" | "danger";

const variants: Record<Variant, string> = {
  primary: "bg-accent text-accent-fg border-accent",
  secondary: "bg-surface text-surface-fg border-input-border",
  danger: "bg-critical text-surface border-critical",
};

// The look of a button, for a link that leads somewhere.
export function buttonStyles(variant: Variant = "secondary"): string {
  return cn(
    "inline-flex min-h-9 items-center justify-center gap-2 rounded-md border px-3 text-sm font-medium",
    "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent",
    "disabled:cursor-not-allowed disabled:opacity-60",
    variants[variant],
  );
}

export function Button({
  variant = "secondary",
  className,
  type = "button",
  ...rest
}: ComponentProps<"button"> & { variant?: Variant }) {
  return (
    <button
      type={type}
      className={cn(buttonStyles(variant), className)}
      {...rest}
    />
  );
}
