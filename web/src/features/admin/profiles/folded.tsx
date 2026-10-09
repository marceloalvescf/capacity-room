import { useId, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";

// A section of the form that starts folded: its title and a line that sums it up, with the button that opens it
// in the same box.
export function Folded({
  title,
  summary,
  open,
  onToggle,
  children,
}: {
  title: string;
  summary: string;
  open: boolean;
  onToggle: () => void;
  children: ReactNode;
}) {
  const { t } = useTranslation();
  const body = useId();
  const titleId = `${body}-title`;
  const buttonId = `${body}-button`;
  return (
    <section className="rounded-md border border-border bg-surface p-3">
      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0">
          <h2 id={titleId} className="font-semibold">{title}</h2>
          <p className="font-mono text-sm break-words text-muted-fg">{summary}</p>
        </div>
        <Button
          id={buttonId}
          aria-labelledby={`${buttonId} ${titleId}`}
          aria-expanded={open}
          aria-controls={body}
          onClick={onToggle}
        >
          {open ? t("admin.profile.fold") : t("admin.profile.change")}
        </Button>
      </div>
      <div id={body} hidden={!open} className="mt-4 flex flex-col gap-4">
        {children}
      </div>
    </section>
  );
}
