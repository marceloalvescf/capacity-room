import type { ComponentProps } from "react";
import { cn } from "@/lib/cn";

type Variant = "primary" | "secondary" | "danger";

const variants: Record<Variant, string> = {
  primary: "bg-action text-action-fg border-action not-disabled:hover:bg-action-hover not-disabled:hover:border-action-hover",
  secondary: "bg-surface text-surface-fg border-input-border not-disabled:hover:bg-hover",
  danger: "bg-critical text-surface border-critical not-disabled:hover:bg-critical-hover not-disabled:hover:border-critical-hover",
};

// The look of a button, for a link that leads somewhere. The colour of a hover changes at once; a press sinks the
// button by a pixel, which only moves under motion-safe.
export function buttonStyles(variant: Variant = "secondary"): string {
  return cn(
    "inline-flex min-h-9 items-center justify-center gap-2 rounded-md border px-3 text-sm font-medium",
    "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent",
    "not-disabled:active:translate-y-px motion-safe:transition-transform motion-safe:duration-100 motion-safe:ease-out",
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
