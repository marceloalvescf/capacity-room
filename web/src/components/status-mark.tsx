import { useTranslation } from "react-i18next";
import { cn } from "@/lib/cn";

export const statusValues = ["healthy", "attention", "critical", "undefined", "unavailable"] as const;
export type Status = (typeof statusValues)[number];

const colour: Record<Status, string> = {
  healthy: "text-healthy",
  attention: "text-attention",
  critical: "text-critical",
  undefined: "text-neutral",
  unavailable: "text-neutral",
};

function Shape({ status }: { status: Status }) {
  switch (status) {
    case "healthy":
      return <circle cx="6" cy="6" r="5" fill="currentColor" />;
    case "attention":
      return <polygon points="6,1 11,11 1,11" fill="currentColor" />;
    case "critical":
      return <polygon points="6,0.5 11.5,6 6,11.5 0.5,6" fill="currentColor" />;
    case "undefined":
      return <circle cx="6" cy="6" r="4.25" fill="none" stroke="currentColor" strokeWidth="1.5" />;
    case "unavailable":
      return <line x1="1.5" y1="6" x2="10.5" y2="6" stroke="currentColor" strokeWidth="2" />;
  }
}

// The shape of a status alone, for a place where the word is given separately. It is 12px unless a size is given.
export function StatusShape({ status, className }: { status: Status; className?: string }) {
  return (
    <svg aria-hidden="true" viewBox="0 0 12 12" className={cn("size-3 shrink-0", colour[status], className)}>
      <Shape status={status} />
    </svg>
  );
}

// With a label of its own, `withStatus` adds the status word for screen readers, as the values of the report view do.
// The colour of the status is the shape's alone; the label keeps the colour of the text around it. `plain` draws the
// shape in that colour too, for a control that mutes it; the shape and the word still name the status.
export function StatusMark({
  status,
  label,
  withStatus = false,
  plain = false,
}: {
  status: Status;
  label?: string;
  withStatus?: boolean;
  plain?: boolean;
}) {
  const { t } = useTranslation();
  return (
    <span className="inline-flex items-center gap-1.5">
      <svg aria-hidden="true" viewBox="0 0 12 12" className={cn("size-3 shrink-0", !plain && colour[status])}>
        <Shape status={status} />
      </svg>
      {label === undefined || !withStatus ? null : <span className="sr-only">{t(`status.${status}`)}</span>}
      <span className={cn("whitespace-nowrap", status === "critical" && "font-semibold")}>{label ?? t(`status.${status}`)}</span>
    </span>
  );
}
