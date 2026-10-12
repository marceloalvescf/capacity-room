import { DatedText } from "@/components/dated-text";
import { useCallback, useId, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { StatusMark, type Status } from "@/components/status-mark";
import { formatElapsed, formatTimeIn } from "@/lib/format";
import type { ViewCapability, ViewMessage } from "./api";
import { DataTable, type Column } from "@/components/data-table";
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
          <li key={metric} className="font-mono text-sm [overflow-wrap:anywhere]">
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
  marked,
}: {
  title: string;
  empty: string;
  messages: ViewMessage[];
  marked: boolean;
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
          {messages.map((message, index) => (
            <li key={index}>
              {marked ? <StatusMark status="attention" label={message.text} withStatus /> : message.text}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

export function QualityTab() {
  const { t } = useTranslation();
  const { quality, report } = useView();
  const elapsed = new Date(quality.collection_completed_at).getTime() - new Date(quality.collection_started_at).getTime();
  const searchText = useCallback(
    (row: ViewCapability) => `${t(`capability.${row.name}`)} ${t(`capabilityState.${row.state}`)} ${t(`strategy.${row.strategy}`)} ${row.remark}`,
    [t],
  );
  const rowName = useCallback((row: ViewCapability) => t(`capability.${row.name}`), [t]);
  const columns = useMemo<Column<ViewCapability>[]>(
    () => [
      {
        id: "capability",
        header: t("report.column.capability"),
        cell: (row) => t(`capability.${row.name}`),
      },
      {
        id: "state",
        header: t("report.column.state"),
        cell: (row) => <StatusMark status={stateStatus[row.state]} label={t(`capabilityState.${row.state}`)} withStatus />,
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
        <DatedText
          i18nKey="report.quality.collected"
          values={{ start: formatTimeIn(quality.collection_started_at, report.timezone), end: formatTimeIn(quality.collection_completed_at, report.timezone), elapsed: formatElapsed(elapsed) }}
        />
      </p>
      <DataTable
        label={t("report.tab.quality")}
        empty={t("report.table.empty")}
        search={{ label: t("report.search.capabilities"), text: searchText }}
        rows={quality.capabilities}
        rowKey={rowKey}
        rowName={rowName}
        columns={columns}
        renderOpened={renderOpened}
        defaultOrder="view"
      />
      <div className="grid gap-6 md:grid-cols-2">
        <Messages title={t("report.quality.warnings")} empty={t("report.quality.noWarnings")} messages={quality.warnings} marked />
        <Messages title={t("report.quality.notices")} empty={t("report.quality.noNotices")} messages={quality.notices} marked={false} />
      </div>
    </div>
  );
}
