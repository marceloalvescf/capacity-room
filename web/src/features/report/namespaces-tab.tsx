import { useCallback, useMemo } from "react";
import { useTranslation } from "react-i18next";
import type { ViewNamespace } from "./api";
import { DataTable, severityRank, type Column } from "@/components/data-table";
import { FindingStatus, matchNamespace, PeakCell, peakTint, RowFindings, UsageGrids, ValueText } from "./pieces";
import { useView } from "./report-page";

const rowKey = (row: ViewNamespace) => row.name;
const searchText = (row: ViewNamespace) => row.name;

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
  const { namespaces, findings } = useView();
  const hasFinding = useCallback((row: ViewNamespace) => findings.some(matchNamespace(row)), [findings]);
  const status = useCallback((row: ViewNamespace) => (hasFinding(row) ? row.status : ""), [hasFinding]);
  const columns = useMemo<Column<ViewNamespace>[]>(
    () => [
      {
        id: "name",
        header: t("report.column.namespace"),
        cell: (row) => <span className="font-mono [overflow-wrap:anywhere]">{row.name}</span>,
        sortValue: (row) => row.name,
      },
      {
        id: "status",
        header: t("report.column.status"),
        cell: (row) => <FindingStatus status={row.status} hasFinding={hasFinding(row)} />,
        sortValue: (row) => severityRank[status(row)] ?? 3,
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
        cellClassName: (row) => peakTint(row.cpu_usage.peak, row.cpu_request_percent.peak),
      },
      {
        id: "memory",
        header: t("report.column.memoryPeak"),
        cell: (row) => <PeakCell peak={row.memory_usage.peak} share={row.memory_request_percent.peak} word="request" />,
        sortValue: (row) => row.memory_usage.peak.raw,
        cellClassName: (row) => peakTint(row.memory_usage.peak, row.memory_request_percent.peak),
      },
    ],
    [t, hasFinding, status],
  );
  return (
    <DataTable
      label={t("report.tab.namespaces")}
      empty={t("report.table.empty")}
      search={{ label: t("report.search.namespaces"), text: searchText }}
      rows={namespaces}
      rowKey={rowKey}
      rowName={rowKey}
      columns={columns}
      renderOpened={renderOpened}
      status={status}
    />
  );
}
