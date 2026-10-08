import { useTranslation } from "react-i18next";
import { StatusShape } from "@/components/status-mark";
import { cn } from "@/lib/cn";
import type { ViewValue } from "./api";

// A value of the view: the shape of its status, the status word for screen readers, then its text.
export function ValueText({ value }: { value: ViewValue }) {
  const { t } = useTranslation();
  return (
    <span className={cn("inline-flex items-center gap-1.5", value.raw === null && "text-muted-fg")}>
      {value.status !== null && (
        <>
          <StatusShape status={value.status} />
          <span className="sr-only">{t(`status.${value.status}`)}</span>
        </>
      )}
      {value.text}
    </span>
  );
}

// A row of counts, each number above its name.
export function Counts({ label, items }: { label: string; items: { label: string; value: ViewValue }[] }) {
  return (
    <dl aria-label={label} className="flex flex-wrap gap-x-8 gap-y-3 sm:flex-nowrap">
      {items.map((item) => (
        <div key={item.label} className="flex flex-col-reverse justify-end gap-0.5">
          <dt className="text-sm leading-snug text-muted-fg">{item.label}</dt>
          <dd className="font-mono text-xl font-semibold leading-tight">
            <ValueText value={item.value} />
          </dd>
        </div>
      ))}
    </dl>
  );
}
