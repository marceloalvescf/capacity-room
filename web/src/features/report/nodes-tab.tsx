import { useCallback, useMemo } from "react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { StatusMark } from "@/components/status-mark";
import type { ViewNode, ViewNodepool, ViewValue } from "./api";
import { CopyName } from "./copy-name";
import { DataTable, severityRank, type Column } from "@/components/data-table";
import { Facts, FindingStatus, matchNode, nodeKey, nodepoolKey, PeakCell, peakTint, RowFindings, StatGrid, useFindingIndex, useWorstFindingIndex, ValueText } from "./pieces";
import { useView } from "./report-page";

const rowKey = (row: ViewNode) => row.name;
const rowName = (row: ViewNode) => row.name;

function FactValue({ value }: { value: ViewValue }) {
  return (
    <span className="font-mono">
      <ValueText value={value} />
    </span>
  );
}

function Opened({ row }: { row: ViewNode }) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-4">
      <p className="text-sm">
        <CopyName name={row.name} />
      </p>
      <Facts
        items={[
          { label: t("report.facts.cpuCapacity"), value: <FactValue value={row.cpu_capacity} /> },
          { label: t("report.facts.cpuAllocatable"), value: <FactValue value={row.cpu_allocatable} /> },
          { label: t("report.facts.memoryCapacity"), value: <FactValue value={row.memory_capacity} /> },
          { label: t("report.facts.memoryAllocatable"), value: <FactValue value={row.memory_allocatable} /> },
        ]}
      />
      <StatGrid
        label={t("report.stat.utilization")}
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

// The readiness of a node and, when it is known, whether it takes new pods.
function stateText(row: ViewNode, t: TFunction): string {
  if (row.ready === null) {
    return t("report.node.unknownState");
  }
  const ready = t(row.ready ? "report.node.ready" : "report.node.notReady");
  if (row.schedulable === null) {
    return ready;
  }
  return `${ready} · ${t(row.schedulable ? "report.node.schedulable" : "report.node.unschedulable")}`;
}

// A node that is not ready is marked for attention; an unknown readiness is marked unavailable.
function NodeState({ row }: { row: ViewNode }) {
  const { t } = useTranslation();
  const text = stateText(row, t);
  if (row.ready === false) {
    return <StatusMark status="attention" label={text} withStatus />;
  }
  if (row.ready === null) {
    return <StatusMark status="unavailable" label={text} withStatus />;
  }
  return <span>{text}</span>;
}

function Pools({ pools }: { pools: ViewNodepool[] }) {
  const { t } = useTranslation();
  const index = useWorstFindingIndex();
  if (pools.length === 0) {
    return <p className="text-sm text-muted-fg">{t("report.table.empty")}</p>;
  }
  return (
    <div className="rounded-md border border-border bg-surface text-surface-fg">
      <table aria-label={t("report.pools.title")} className="w-full text-sm">
        <thead className="border-b border-border">
          <tr>
            <th scope="col" className="label px-3 py-2 text-left">
              {t("report.column.nodepool")}
            </th>
            <th scope="col" className="label px-3 py-2 text-left">
              {t("report.column.status")}
            </th>
            <th scope="col" className="label px-3 py-2 text-left">
              {t("report.column.confidence")}
            </th>
            <th scope="col" className="label px-3 py-2 text-left">
              {t("report.column.nodes")}
            </th>
          </tr>
        </thead>
        <tbody className="[&>tr:not(:last-child)>td]:border-b [&>tr>td]:border-border">
          {pools.map((pool) => (
            <tr key={pool.name}>
              <td className="px-3 py-2 align-top">
                {pool.name === "unknown" ? t("report.node.unknownPool") : <CopyName name={pool.name} />}
              </td>
              <td className="px-3 py-2 align-top">
                <FindingStatus
                  status={index.get(nodepoolKey(pool))?.severity ?? "healthy"}
                  hasFinding={index.has(nodepoolKey(pool))}
                />
              </td>
              <td className="px-3 py-2 align-top">{t(`confidence.${pool.confidence}`)}</td>
              <td className="px-3 py-2 align-top">
                <span className="font-mono">{pool.nodes.length}</span>
                <ul className="mt-0.5 flex flex-wrap gap-x-3 text-xs text-muted-fg">
                  {pool.nodes.map((name) => (
                    <li key={name} className="font-mono [overflow-wrap:anywhere]">
                      {name}
                    </li>
                  ))}
                </ul>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function NodesTab() {
  const { t } = useTranslation();
  const { nodes, nodepools } = useView();
  const index = useFindingIndex();
  const hasFinding = useCallback((row: ViewNode) => index.has(nodeKey(row)), [index]);
  const status = useCallback((row: ViewNode) => (hasFinding(row) ? row.status : ""), [hasFinding]);
  const unknownPool = t("report.node.unknownPool");
  const searchText = useCallback(
    (row: ViewNode) => `${row.name} ${row.pool === "unknown" ? unknownPool : row.pool}`,
    [unknownPool],
  );
  const columns = useMemo<Column<ViewNode>[]>(
    () => [
      {
        id: "name",
        header: t("report.column.node"),
        cell: (row) => (
          <div className="flex flex-col gap-0.5">
            <span className="font-mono [overflow-wrap:anywhere]">{row.name}</span>
            <span className="text-xs text-muted-fg">
              {row.pool === "unknown" ? t("report.node.unknownPool") : <span className="font-mono [overflow-wrap:anywhere]">{row.pool}</span>}
            </span>
          </div>
        ),
        sortValue: (row) => row.name,
      },
      {
        id: "status",
        header: t("report.column.status"),
        cell: (row) => (
          <div className="flex flex-col gap-0.5">
            <FindingStatus status={row.status} hasFinding={hasFinding(row)} />
            {row.reason_status !== null && <span className="max-w-56 text-xs leading-snug">{row.reason}</span>}
          </div>
        ),
        sortValue: (row) => severityRank[status(row)] ?? 3,
      },
      {
        id: "state",
        header: t("report.column.state"),
        cell: (row) => <NodeState row={row} />,
      },
      {
        id: "cpu",
        header: t("report.column.cpuPeak"),
        cell: (row) => <PeakCell peak={row.cpu_usage_percent.peak} share={row.cpu_request_percent} word="requested" />,
        sortValue: (row) => row.cpu_usage_percent.peak.raw,
        cellClassName: (row) => peakTint(row.cpu_usage_percent.peak, row.cpu_request_percent),
      },
      {
        id: "memory",
        header: t("report.column.memoryPeak"),
        cell: (row) => (
          <PeakCell peak={row.memory_usage_percent.peak} share={row.memory_request_percent} word="requested" />
        ),
        sortValue: (row) => row.memory_usage_percent.peak.raw,
        cellClassName: (row) => peakTint(row.memory_usage_percent.peak, row.memory_request_percent),
      },
    ],
    [t, hasFinding, status],
  );
  return (
    <div className="flex flex-col gap-6">
      <DataTable
        label={t("report.tab.nodes")}
        empty={t("report.table.empty")}
        search={{ label: t("report.search.nodes"), text: searchText }}
        rows={nodes}
        rowKey={rowKey}
        rowName={rowName}
        columns={columns}
        renderOpened={renderOpened}
        status={status}
      />
      <section className="flex flex-col gap-3">
        <h2 className="text-base font-semibold">{t("report.pools.title")}</h2>
        <Pools pools={nodepools} />
      </section>
    </div>
  );
}
