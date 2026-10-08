import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { StatusMark } from "@/components/status-mark";
import { cn } from "@/lib/cn";
import type { ViewNode } from "./api";
import { CopyName } from "./copy-name";
import { DataTable, severityRank, type ReportColumn } from "./data-table";
import { Facts, matchNode, PeakCell, RowFindings, StatGrid } from "./pieces";
import { useView } from "./report-page";

const rowKey = (row: ViewNode) => row.name;
const rowName = (row: ViewNode) => row.name;
const searchText = (row: ViewNode) => `${row.name} ${row.pool}`;
const status = (row: ViewNode) => row.status;

function Opened({ row }: { row: ViewNode }) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-4">
      <p className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm">
        <CopyName name={row.name} />
        {row.reason_status === null ? (
          <span>{row.reason}</span>
        ) : (
          <StatusMark status={row.reason_status} label={row.reason} />
        )}
      </p>
      <Facts
        items={[
          { label: t("report.facts.cpuCapacity"), value: <span className="font-mono">{row.cpu_capacity.text}</span> },
          { label: t("report.facts.cpuAllocatable"), value: <span className="font-mono">{row.cpu_allocatable.text}</span> },
          { label: t("report.facts.memoryCapacity"), value: <span className="font-mono">{row.memory_capacity.text}</span> },
          {
            label: t("report.facts.memoryAllocatable"),
            value: <span className="font-mono">{row.memory_allocatable.text}</span>,
          },
        ]}
      />
      <StatGrid
        rows={[
          { label: t("report.stat.cpuPercent"), stat: row.cpu_usage_percent },
          { label: t("report.stat.memoryPercent"), stat: row.memory_usage_percent },
        ]}
      />
      <RowFindings match={matchNode(row)} />
    </div>
  );
}

const renderOpened = (row: ViewNode) => <Opened row={row} />;

export function NodesTab() {
  const { t } = useTranslation();
  const { nodes, nodepools } = useView();
  const columns = useMemo<ReportColumn<ViewNode>[]>(
    () => [
      {
        id: "name",
        header: t("report.column.node"),
        cell: (row) => (
          <div className="flex flex-col gap-0.5">
            <span className="font-mono">{row.name}</span>
            <span className="text-xs text-muted-fg">
              {row.pool === "unknown" ? t("report.node.unknownPool") : <span className="font-mono">{row.pool}</span>}
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
        id: "state",
        header: t("report.column.state"),
        cell: (row) => (
          <span className={cn(row.ready === false && "text-attention")}>
            {row.ready === null || row.schedulable === null
              ? t("report.node.unknownState")
              : `${t(row.ready ? "report.node.ready" : "report.node.notReady")} · ${t(row.schedulable ? "report.node.schedulable" : "report.node.unschedulable")}`}
          </span>
        ),
      },
      {
        id: "cpu",
        header: t("report.column.cpuPeak"),
        cell: (row) => <PeakCell peak={row.cpu_usage_percent.peak} share={row.cpu_request_percent} word="requested" />,
        sortValue: (row) => row.cpu_usage_percent.peak.raw,
      },
      {
        id: "memory",
        header: t("report.column.memoryPeak"),
        cell: (row) => (
          <PeakCell peak={row.memory_usage_percent.peak} share={row.memory_request_percent} word="requested" />
        ),
        sortValue: (row) => row.memory_usage_percent.peak.raw,
      },
    ],
    [t],
  );
  return (
    <div className="flex flex-col gap-6">
      <DataTable
        label={t("report.tab.nodes")}
        searchLabel={t("report.search.nodes")}
        rows={nodes}
        rowKey={rowKey}
        rowName={rowName}
        columns={columns}
        searchText={searchText}
        renderOpened={renderOpened}
        status={status}
      />
      <section className="flex flex-col gap-3">
        <h2 className="text-base font-semibold">{t("report.pools.title")}</h2>
        <div className="rounded-md border border-border bg-surface text-surface-fg">
          <table aria-label={t("report.pools.title")} className="w-full text-sm">
            <thead className="border-b border-border text-muted-fg">
              <tr>
                <th scope="col" className="px-3 py-2 text-left font-medium">
                  {t("report.column.nodepool")}
                </th>
                <th scope="col" className="px-3 py-2 text-left font-medium">
                  {t("report.column.confidence")}
                </th>
                <th scope="col" className="px-3 py-2 text-left font-medium">
                  {t("report.column.nodes")}
                </th>
              </tr>
            </thead>
            <tbody className="[&>tr:not(:last-child)>td]:border-b [&>tr>td]:border-border">
              {nodepools.map((pool) => (
                <tr key={pool.name}>
                  <td className="px-3 py-2 align-top">
                    {pool.name === "unknown" ? t("report.node.unknownPool") : <CopyName name={pool.name} />}
                  </td>
                  <td className="px-3 py-2 align-top">{t(`confidence.${pool.confidence}`)}</td>
                  <td className="px-3 py-2 align-top">
                    <span className="font-mono">{pool.nodes.length}</span>
                    <span className="ml-2 font-mono text-xs text-muted-fg">{pool.nodes.join(", ")}</span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
