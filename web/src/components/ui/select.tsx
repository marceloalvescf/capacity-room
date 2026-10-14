import { useId, type ComponentProps } from "react";
import { cn } from "@/lib/cn";

interface SelectProps extends Omit<ComponentProps<"select">, "id"> {
  label: string;
  hint?: string | undefined;
  error?: string | undefined;
}

export function Select({
  label,
  hint,
  error,
  className,
  children,
  "aria-describedby": describedBy,
  ...rest
}: SelectProps) {
  const id = useId();
  const hintId = `${id}-hint`;
  const errorId = `${id}-error`;
  const described = [describedBy ?? null, hint === undefined ? null : hintId, error === undefined ? null : errorId]
    .filter((value) => value !== null && value !== "")
    .join(" ");
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className="text-sm font-medium">
        {label}
      </label>
      <select
        {...rest}
        id={id}
        aria-invalid={error === undefined ? undefined : true}
        aria-describedby={described === "" ? undefined : described}
        className={cn(
          "min-h-9 rounded-md border border-input-border bg-surface px-3 text-sm text-surface-fg",
          "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent",
          className,
        )}
      >
        {children}
      </select>
      {hint === undefined ? null : (
        <p id={hintId} className="text-sm text-muted-fg">
          {hint}
        </p>
      )}
      {error === undefined ? null : (
        <p id={errorId} className="text-sm text-critical">
          {error}
        </p>
      )}
    </div>
  );
}
