import { cn } from "@/lib/cn";

interface SegmentedProps<T extends string> {
  legend: string;
  hideLegend?: boolean;
  name: string;
  value: T;
  options: readonly { value: T; label: string; lang?: string }[];
  onChange: (value: T) => void;
}

export function Segmented<T extends string>({ legend, hideLegend = false, name, value, options, onChange }: SegmentedProps<T>) {
  return (
    <fieldset className="flex">
      <legend className={hideLegend ? "sr-only" : "mb-1 text-sm font-medium"}>{legend}</legend>
      {options.map((option) => (
        <label
          key={option.value}
          className={cn(
            "relative inline-flex min-h-9 cursor-pointer items-center border border-input-border bg-surface px-4 text-sm font-medium text-surface-fg",
            "first:rounded-l-md last:rounded-r-md not-first:-ml-px",
            "has-checked:z-10 has-checked:border-action has-checked:bg-action has-checked:text-action-fg",
            "has-focus-visible:z-20 has-focus-visible:outline-2 has-focus-visible:outline-offset-2 has-focus-visible:outline-accent",
          )}
        >
          <input
            className="absolute inset-0 m-0 cursor-pointer appearance-none opacity-0"
            type="radio"
            name={name}
            value={option.value}
            checked={value === option.value}
            onChange={() => {
              onChange(option.value);
            }}
          />
          <span lang={option.lang}>{option.label}</span>
        </label>
      ))}
    </fieldset>
  );
}
