import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { StatusMark } from "@/components/status-mark";
import type { ViewWorkload } from "./api";
import { DataTable, severityRank, type ReportColumn } from "./data-table";
import { Facts, matchWorkload, PeakCell, RowFindings, UsageGrids, ValueText } from "./pieces";
import { useView } from "./report-page";

const rowKey = (row: ViewWorkload) => `${row.namespace}/${row.kind}/${row.name}`;
const rowName = (row: ViewWorkload) => row.name;
const searchText = (row: ViewWorkload) => `${row.name} ${row.kind} ${row.namespace}`;
const status = (row: ViewWorkload) => row.status;

function Opened({ row }: { row: ViewWorkload }) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-4">
      <Facts
        items={[
          {
            label: t("report.facts.pods"),
            value: (
              <span className="font-mono">
                <ValueText value={row.pods} />
              </span>
            ),
          },
          { label: t("report.facts.hpa"), value: t(`hpaCoverage.${row.hpa_coverage}`) },
          {
            label: t("report.facts.values"),
            value: row.approximate ? t("report.facts.approximate") : t("report.facts.measured"),
          },
        ]}
      />
      <UsageGrids row={row} />
      <RowFindings match={matchWorkload(row)} />
    </div>
  );
}

const renderOpened = (row: ViewWorkload) => <Opened row={row} />;

export function WorkloadsTab() {
  const { t } = useTranslation();
  const view = useView();
  const workloads = useMemo(() => view.workloads.filter((workload) => workload.observed_usage), [view.workloads]);
  const columns = useMemo<ReportColumn<ViewWorkload>[]>(
    () => [
      {
        id: "name",
        header: t("report.column.workload"),
        cell: (row) => (
          <div className="flex flex-col gap-0.5">
            <span className="font-mono">{row.name}</span>
            <span className="flex flex-wrap gap-x-2 text-xs text-muted-fg">
              <span className="font-mono">{row.kind}</span>
              <span className="font-mono">{row.namespace}</span>
            </span>
          </div>
        ),
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
      label={t("report.tab.workloads")}
      searchLabel={t("report.search.workloads")}
      rows={workloads}
      rowKey={rowKey}
      rowName={rowName}
      columns={columns}
      searchText={searchText}
      renderOpened={renderOpened}
      status={status}
    />
  );
}
