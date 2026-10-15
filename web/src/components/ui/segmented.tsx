import { cn } from "@/lib/cn";

interface SegmentedProps<T extends string> {
  legend: string;
  hideLegend?: boolean;
  name: string;
  value: T;
  options: readonly { value: T; label: string; lang?: string }[];
  onChange: (value: T) => void;
}

export function Segmented<T extends string>({
  legend,
  hideLegend = false,
  name,
  value,
  options,
  onChange,
}: SegmentedProps<T>) {
  return (
    <fieldset>
      <legend className={hideLegend ? "sr-only" : "mb-1 text-sm font-medium"}>
        {legend}
      </legend>
      <div className="flex w-fit flex-wrap rounded-md bg-hover p-0.5">
        {options.map((option) => (
          <label
            key={option.value}
            className={cn(
              "relative inline-flex min-h-8 cursor-pointer items-center rounded-[5px] border border-transparent px-3.5 text-sm font-medium text-muted-fg",
              "not-has-checked:hover:text-fg",
              "has-checked:z-10 has-checked:border-input-border has-checked:bg-surface has-checked:text-surface-fg",
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
      </div>
    </fieldset>
  );
}
