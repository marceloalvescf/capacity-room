import { CaretDownIcon, CopyIcon } from "@phosphor-icons/react";
import { useCallback, useMemo, useRef } from "react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { StatusMark } from "@/components/status-mark";
import type { ViewNode, ViewNodepool, ViewValue } from "./api";
import { IconButton } from "@/components/ui/icon-button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { CopyName, useCopy } from "./copy-name";
import { DataTable, severityRank, type Column } from "@/components/data-table";
import { Facts, FindingStatus, matchNode, nodeKey, PeakCell, peakTint, RowFindings, StatGrid, useFindingIndex, ValueText } from "./pieces";
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

// The state is marked with the status the view gives it.
function NodeState({ row }: { row: ViewNode }) {
  const { t } = useTranslation();
  const text = stateText(row, t);
  if (row.state_status) {
    return <StatusMark status={row.state_status} label={text} withStatus />;
  }
  return <span>{text}</span>;
}

const poolKey = (pool: ViewNodepool) => pool.name;
const poolName = (pool: ViewNodepool) => pool.name;

// The count of a pool's nodes as a button that opens the names, each to copy, and a control that copies them all.
function PoolNodes({ pool }: { pool: ViewNodepool }) {
  const { t } = useTranslation();
  const copy = useCopy();
  const name = pool.name === "unknown" ? t("report.node.unknownPool") : pool.name;
  const heading = t("report.pools.heading", { pool: name });
  const list = useRef<HTMLUListElement>(null);
  return (
    <Popover>
      <PopoverTrigger asChild>
        <button
          type="button"
          aria-label={t("report.pools.nodesOf", { pool: name, count: pool.nodes.length })}
          className="inline-flex items-center gap-1 rounded-md px-1 font-mono focus-visible:outline-2 focus-visible:outline-accent"
        >
          {pool.nodes.length}
          <CaretDownIcon aria-hidden="true" className="size-3 text-accent" />
        </button>
      </PopoverTrigger>
      <PopoverContent
        align="start"
        aria-label={heading}
        className="w-80 max-w-[calc(100vw-2rem)]"
        onOpenAutoFocus={(event) => {
          event.preventDefault();
          list.current?.focus();
        }}
      >
        <div className="flex flex-col gap-2">
          <div className="flex items-center justify-between gap-2">
            <h3 className="label">{heading}</h3>
            <IconButton
              label={t("report.pools.copyAll")}
              variant="secondary"
              className="h-7 min-h-7 w-7 border-transparent bg-transparent"
              onClick={() => {
                void copy(pool.nodes.join("\n"), t("report.pools.copiedAll", { pool: name }));
              }}
            >
              <CopyIcon aria-hidden="true" className="size-4" />
            </IconButton>
          </div>
          <ul ref={list} tabIndex={-1} aria-label={heading} className="flex max-h-56 flex-col gap-1 overflow-y-auto text-sm outline-none">
            {pool.nodes.map((node) => (
              <li key={node}>
                <CopyName name={node} />
              </li>
            ))}
          </ul>
        </div>
      </PopoverContent>
    </Popover>
  );
}

function Pools({ pools }: { pools: ViewNodepool[] }) {
  const { t } = useTranslation();
  const columns = useMemo<Column<ViewNodepool>[]>(
    () => [
      {
        id: "name",
        header: t("report.column.nodepool"),
        cell: (pool) => (pool.name === "unknown" ? t("report.node.unknownPool") : <CopyName name={pool.name} />),
      },
      {
        id: "status",
        header: t("report.column.status"),
        cell: (pool) => <FindingStatus status={pool.status ?? "healthy"} hasFinding={pool.status !== null} />,
      },
      {
        id: "confidence",
        header: t("report.column.confidence"),
        cell: (pool) => t(`confidence.${pool.confidence}`),
      },
      {
        id: "nodes",
        header: t("report.column.nodes"),
        cell: (pool) => <PoolNodes pool={pool} />,
      },
    ],
    [t],
  );
  return (
    <DataTable
      label={t("report.pools.title")}
      empty={t("report.table.empty")}
      rows={pools}
      rowKey={poolKey}
      rowName={poolName}
      columns={columns}
      defaultOrder="view"
    />
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
        <h2 className="text-lg font-semibold">{t("report.pools.title")}</h2>
        <Pools pools={nodepools} />
      </section>
    </div>
  );
}
