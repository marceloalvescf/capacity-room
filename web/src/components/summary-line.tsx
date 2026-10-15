import { Fragment } from "react";
import { cn } from "@/lib/cn";
import { Separator } from "./separator";

// A summary line: entries parted by a gap (a middle dot for screen readers), each made of parts parted by a space. A value is set in the
// monospaced font and a word is not.
export interface SummaryPart {
  text: string;
  value: boolean;
}

export type Summary = readonly (readonly SummaryPart[])[];

export const wordPart = (text: string): SummaryPart[] => [{ text, value: false }];
export const valuePart = (text: string): SummaryPart[] => [{ text, value: true }];

export function summaryText(summary: Summary): string {
  return summary.map((entry) => entry.map((part) => part.text).join(" ")).join(" · ");
}

export function SummaryLine({ summary, className }: { summary: Summary; className?: string }) {
  return (
    <span className={className}>
      {summary.map((entry, index) => (
        <Fragment key={index}>
          {index > 0 ? <Separator /> : null}
          {entry.map((part, at) => (
            <Fragment key={at}>
              {at > 0 ? " " : null}
              <span className={cn(part.value && "font-mono")}>{part.text}</span>
            </Fragment>
          ))}
        </Fragment>
      ))}
    </span>
  );
}
