import { useEffect, useRef, type ComponentProps } from "react";
import { cn } from "@/lib/cn";

interface CheckboxProps extends ComponentProps<"input"> {
  label: string;
  indeterminate?: boolean;
}

export function Checkbox({ label, indeterminate = false, className, ...rest }: CheckboxProps) {
  const input = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (input.current !== null) {
      input.current.indeterminate = indeterminate;
    }
  }, [indeterminate]);
  return (
    <label className="inline-flex min-h-9 cursor-pointer items-center gap-2 text-sm">
      <input
        ref={input}
        type="checkbox"
        className={cn(
          "size-4 cursor-pointer accent-accent",
          "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent",
          className,
        )}
        {...rest}
      />
      {label}
    </label>
  );
}
