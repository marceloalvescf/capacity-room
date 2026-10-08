import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { StatusMark } from "@/components/status-mark";
import type { ViewNamespace } from "./api";
import { DataTable, severityRank, type ReportColumn } from "./data-table";
import { matchNamespace, PeakCell, RowFindings, UsageGrids, ValueText } from "./pieces";
import { useView } from "./report-page";

const rowKey = (row: ViewNamespace) => row.name;
const searchText = (row: ViewNamespace) => row.name;
const status = (row: ViewNamespace) => row.status;

function Opened({ row }: { row: ViewNamespace }) {
  return (
    <div className="flex flex-col gap-4">
      <UsageGrids row={row} />
      <RowFindings match={matchNamespace(row)} />
    </div>
  );
}

const renderOpened = (row: ViewNamespace) => <Opened row={row} />;

export function NamespacesTab() {
  const { t } = useTranslation();
  const { namespaces } = useView();
  const columns = useMemo<ReportColumn<ViewNamespace>[]>(
    () => [
      {
        id: "name",
        header: t("report.column.namespace"),
        cell: (row) => <span className="font-mono">{row.name}</span>,
        sortValue: (row) => row.name,
      },
      {
        id: "status",
        header: t("report.column.status"),
        cell: (row) => <StatusMark status={row.status} />,
        sortValue: (row) => severityRank[row.status] ?? 3,
      },
      {
        id: "pods",
        header: t("report.column.pods"),
        cell: (row) => (
          <span className="font-mono">
            <ValueText value={row.pods} />
          </span>
        ),
        sortValue: (row) => row.pods.raw,
        align: "end",
      },
      {
        id: "cpu",
        header: t("report.column.cpuPeak"),
        cell: (row) => <PeakCell peak={row.cpu_usage.peak} share={row.cpu_request_percent.peak} word="request" />,
        sortValue: (row) => row.cpu_usage.peak.raw,
      },
      {
        id: "memory",
        header: t("report.column.memoryPeak"),
        cell: (row) => <PeakCell peak={row.memory_usage.peak} share={row.memory_request_percent.peak} word="request" />,
        sortValue: (row) => row.memory_usage.peak.raw,
      },
    ],
    [t],
  );
  return (
    <DataTable
      label={t("report.tab.namespaces")}
      searchLabel={t("report.search.namespaces")}
      rows={namespaces}
      rowKey={rowKey}
      rowName={rowKey}
      columns={columns}
      searchText={searchText}
      renderOpened={renderOpened}
      status={status}
    />
  );
}
