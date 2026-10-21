import { useTranslation } from "react-i18next";
import { DatedText } from "@/components/dated-text";
import { formatTime } from "@/lib/format";
import type { ReportView } from "./api";

// The facts of a report on one line: its cluster, its period, the namespaces it covers when the caller names them,
// its report profile, and who requested it and when.
export function ReportMeta({
  view,
  createdAt,
  owner,
  namespaces,
}: {
  view: ReportView;
  createdAt: string;
  owner: string;
  namespaces?: string[];
}) {
  const { t } = useTranslation();
  return (
    <p className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-muted-fg">
      <span>
        {t("report.cluster")} <span className="font-mono text-fg">{view.report.cluster_name}</span>
      </span>
      <span className="whitespace-nowrap">{view.report.period_text}</span>
      {namespaces === undefined ? null : namespaces.length === 0 ? (
        <span>{t("home.meta.namespaces")}</span>
      ) : (
        <span className="font-mono text-fg">{namespaces.join(", ")}</span>
      )}
      <span>{t(`reportProfile.${view.report.profile}`)}</span>
      <span>
        <DatedText i18nKey="progress.requested" values={{ time: formatTime(createdAt), username: owner }} />
      </span>
    </p>
  );
}
