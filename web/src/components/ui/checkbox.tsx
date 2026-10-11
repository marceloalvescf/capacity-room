import { useEffect, useId, useRef, type ComponentProps } from "react";
import { cn } from "@/lib/cn";

interface CheckboxProps extends ComponentProps<"input"> {
  label: string;
  indeterminate?: boolean;
  hideLabel?: boolean;
  hint?: string | undefined;
  error?: string | undefined;
}

export function Checkbox({
  label,
  indeterminate = false,
  hideLabel = false,
  hint,
  error,
  className,
  ...rest
}: CheckboxProps) {
  const input = useRef<HTMLInputElement>(null);
  const id = useId();
  const hintId = `${id}-hint`;
  const errorId = `${id}-error`;
  const described = [hint === undefined ? null : hintId, error === undefined ? null : errorId]
    .filter((value) => value !== null)
    .join(" ");
  useEffect(() => {
    if (input.current !== null) {
      input.current.indeterminate = indeterminate;
    }
  });
  const box = (
    <label className="inline-flex min-h-9 cursor-pointer items-center gap-2 text-sm">
      <input
        ref={input}
        type="checkbox"
        aria-invalid={error === undefined ? undefined : true}
        aria-describedby={described === "" ? undefined : described}
        className={cn(
          "size-4 cursor-pointer accent-action",
          "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent",
          className,
        )}
        {...rest}
      />
      <span className={cn(hideLabel && "sr-only")}>{label}</span>
    </label>
  );
  if (hint === undefined && error === undefined) {
    return box;
  }
  return (
    <div className="flex flex-col">
      {box}
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
