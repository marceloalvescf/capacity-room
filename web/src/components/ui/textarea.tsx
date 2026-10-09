import { useId, type ComponentProps } from "react";
import { cn } from "@/lib/cn";

interface TextAreaProps extends Omit<ComponentProps<"textarea">, "id"> {
  label: string;
  hint?: string | undefined;
  error?: string | undefined;
}

export function TextArea({ label, hint, error, className, ...rest }: TextAreaProps) {
  const id = useId();
  const hintId = `${id}-hint`;
  const errorId = `${id}-error`;
  const described = [hint === undefined ? null : hintId, error === undefined ? null : errorId]
    .filter((value) => value !== null)
    .join(" ");
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={id} className="text-sm font-medium">
        {label}
      </label>
      <textarea
        id={id}
        aria-invalid={error === undefined ? undefined : true}
        aria-describedby={described === "" ? undefined : described}
        className={cn(
          "rounded-md border border-input-border bg-surface px-3 py-2 font-mono text-sm text-surface-fg",
          "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent",
          className,
        )}
        {...rest}
      />
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
