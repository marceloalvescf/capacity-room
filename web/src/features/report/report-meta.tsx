import { useTranslation } from "react-i18next";
import { DatedText } from "@/components/dated-text";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { formatTime } from "@/lib/format";
import type { ReportView } from "./api";

// The facts of a report on one line: its cluster, its period, the namespaces it covers when the caller gives the
// selection (all_namespaces: all of them, listed in a popover), its report profile, and who requested it and when.
export function ReportMeta({
  view,
  createdAt,
  owner,
  selection,
}: {
  view: ReportView;
  createdAt: string;
  owner: string;
  selection?: { namespaces: string[]; all_namespaces: boolean } | undefined;
}) {
  const { t } = useTranslation();
  return (
    <p className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-muted-fg">
      <span>
        {t("report.cluster")} <span className="font-mono text-fg">{view.report.cluster_name}</span>
      </span>
      <span className="whitespace-nowrap">{view.report.period_text}</span>
      {selection === undefined ? null : selection.all_namespaces ? (
        <Popover>
          <PopoverTrigger asChild>
            <button
              type="button"
              className="rounded-sm underline decoration-dotted underline-offset-4 hover:text-fg focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
            >
              {t("namespaces.allCount", { count: view.report.namespaces.length })}
            </button>
          </PopoverTrigger>
          <PopoverContent side="bottom" align="start" collisionPadding={24} className="w-72 p-2">
            <ul className="flex max-h-56 flex-col overflow-y-auto px-1 font-mono text-sm">
              {view.report.namespaces.map((name) => (
                <li key={name} className="py-0.5">
                  {name}
                </li>
              ))}
            </ul>
          </PopoverContent>
        </Popover>
      ) : (
        <span className="font-mono text-fg">{selection.namespaces.join(", ")}</span>
      )}
      <span>{t(`reportProfile.${view.report.profile}`)}</span>
      <span>
        <DatedText i18nKey="progress.requested" values={{ time: formatTime(createdAt), username: owner }} />
      </span>
    </p>
  );
}
