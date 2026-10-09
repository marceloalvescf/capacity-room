import { useId, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { StatusMark } from "@/components/status-mark";
import type { ViewWorkload } from "./api";
import { DataTable, severityRank, type Column } from "@/components/data-table";
import { Facts, matchWorkload, PeakCell, RowFindings, UsageGrids, ValueText, WorkloadName } from "./pieces";
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

function Unobserved() {
  const { t } = useTranslation();
  const view = useView();
  const rows = useMemo(() => view.workloads.filter((workload) => !workload.observed_usage), [view.workloads]);
  const heading = useId();
  if (rows.length === 0) {
    return null;
  }
  return (
    <section className="flex flex-col gap-3">
      <h2 id={heading} className="text-base font-semibold">
        {t("report.unobserved.title")}
      </h2>
      <p className="text-sm text-muted-fg">{t("report.unobserved.hint")}</p>
      <div className="rounded-md border border-border bg-surface text-surface-fg">
        <table aria-labelledby={heading} className="w-full text-sm">
          <thead className="border-b border-border text-muted-fg">
            <tr>
              <th scope="col" className="px-3 py-2 text-left font-medium">
                {t("report.column.workload")}
              </th>
              <th scope="col" className="px-3 py-2 text-left font-medium">
                {t("report.column.status")}
              </th>
              <th scope="col" className="px-3 py-2 text-left font-medium">
                {t("report.facts.hpa")}
              </th>
            </tr>
          </thead>
          <tbody className="[&>tr:not(:last-child)>td]:border-b [&>tr>td]:border-border">
            {rows.map((row) => (
              <tr key={rowKey(row)}>
                <td className="px-3 py-2 align-top">
                  <WorkloadName row={row} />
                </td>
                <td className="px-3 py-2 align-top">
                  <StatusMark status={row.status} />
                </td>
                <td className="px-3 py-2 align-top">{t(`hpaCoverage.${row.hpa_coverage}`)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

export function WorkloadsTab() {
  const { t } = useTranslation();
  const view = useView();
  const workloads = useMemo(() => view.workloads.filter((workload) => workload.observed_usage), [view.workloads]);
  const columns = useMemo<Column<ViewWorkload>[]>(
    () => [
      {
        id: "name",
        header: t("report.column.workload"),
        cell: (row) => <WorkloadName row={row} />,
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
    <div className="flex flex-col gap-6">
      <DataTable
      label={t("report.tab.workloads")}
      empty={t("report.table.empty")}
      search={{ label: t("report.search.workloads"), text: searchText }}
      rows={workloads}
      rowKey={rowKey}
      rowName={rowName}
      columns={columns}
      renderOpened={renderOpened}
      status={status}
      />
      <Unobserved />
    </div>
  );
}
