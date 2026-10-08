import { useCallback, useId, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { StatusMark, type Status } from "@/components/status-mark";
import { formatElapsed, formatTime } from "@/lib/format";
import type { ViewCapability, ViewMessage } from "./api";
import { DataTable, type ReportColumn } from "./data-table";
import { useView } from "./report-page";

const stateStatus: Record<ViewCapability["state"], Status> = {
  available: "healthy",
  partial: "attention",
  unavailable: "unavailable",
};

const rowKey = (row: ViewCapability) => row.name;

function Opened({ row }: { row: ViewCapability }) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-2">
      <p className="text-sm font-medium">{t("report.quality.metrics")}</p>
      <ul className="flex flex-col gap-1">
        {row.evidence.map((metric) => (
          <li key={metric} className="font-mono text-sm">
            {metric}
          </li>
        ))}
      </ul>
    </div>
  );
}

const renderOpened = (row: ViewCapability) => <Opened row={row} />;

function Messages({
  title,
  empty,
  messages,
}: {
  title: string;
  empty: string;
  messages: ViewMessage[];
}) {
  const heading = useId();
  return (
    <section className="flex flex-col gap-3">
      <h2 id={heading} className="text-base font-semibold">
        {title}
      </h2>
      {messages.length === 0 ? (
        <p className="text-sm text-muted-fg">{empty}</p>
      ) : (
        <ul aria-labelledby={heading} className="flex flex-col gap-2 text-sm">
          {messages.map((message) => (
            <li key={message.code}><StatusMark status="attention" label={message.text} /></li>
          ))}
        </ul>
      )}
    </section>
  );
}

export function QualityTab() {
  const { t } = useTranslation();
  const { quality } = useView();
  const elapsed = new Date(quality.collection_completed_at).getTime() - new Date(quality.collection_started_at).getTime();
  const searchText = useCallback(
    (row: ViewCapability) => `${t(`capability.${row.name}`)} ${t(`capabilityState.${row.state}`)} ${t(`strategy.${row.strategy}`)} ${row.remark}`,
    [t],
  );
  const rowName = useCallback((row: ViewCapability) => t(`capability.${row.name}`), [t]);
  const columns = useMemo<ReportColumn<ViewCapability>[]>(
    () => [
      {
        id: "capability",
        header: t("report.column.capability"),
        cell: (row) => t(`capability.${row.name}`),
      },
      {
        id: "state",
        header: t("report.column.state"),
        cell: (row) => <StatusMark status={stateStatus[row.state]} label={t(`capabilityState.${row.state}`)} />,
      },
      {
        id: "source",
        header: t("report.column.source"),
        cell: (row) => t(`strategy.${row.strategy}`),
      },
      {
        id: "remark",
        header: t("report.column.remark"),
        cell: (row) => <span className="text-muted-fg">{row.remark}</span>,
      },
    ],
    [t],
  );
  return (
    <div className="flex flex-col gap-6">
      <p className="text-sm">
        {t("report.quality.collected", {
          start: formatTime(quality.collection_started_at),
          end: formatTime(quality.collection_completed_at),
          elapsed: formatElapsed(elapsed),
        })}
      </p>
      <DataTable
        label={t("report.tab.quality")}
        searchLabel={t("report.search.capabilities")}
        rows={quality.capabilities}
        rowKey={rowKey}
        rowName={rowName}
        columns={columns}
        searchText={searchText}
        renderOpened={renderOpened}
        defaultOrder="view"
      />
      <div className="grid gap-6 md:grid-cols-2">
        <Messages title={t("report.quality.warnings")} empty={t("report.quality.noWarnings")} messages={quality.warnings} />
        <Messages title={t("report.quality.notices")} empty={t("report.quality.noNotices")} messages={quality.notices} />
      </div>
    </div>
  );
}
