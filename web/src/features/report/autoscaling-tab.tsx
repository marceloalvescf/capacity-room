import { useCallback, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { StatusMark } from "@/components/status-mark";
import type { ViewHPA } from "./api";
import { DataTable, severityRank, type Column } from "@/components/data-table";
import { Counts, FindingStatus, matchHPA, RowFindings, StatGrid, ValueText, worstSeverity } from "./pieces";
import { useView } from "./report-page";

const rowKey = (row: ViewHPA) => `${row.namespace}/${row.name}`;
const rowName = (row: ViewHPA) => row.name;
const searchText = (row: ViewHPA) => `${row.name} ${row.target_kind} ${row.target_name} ${row.namespace}`;

function Opened({ row }: { row: ViewHPA }) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-4">
      <StatGrid
        label={t("report.stat.replicas")}
        rows={[
          { label: t("report.stat.currentReplicas"), stat: row.current },
          { label: t("report.stat.desiredReplicas"), stat: row.desired },
        ]}
      />
      <RowFindings match={matchHPA(row)} />
    </div>
  );
}

const renderOpened = (row: ViewHPA) => <Opened row={row} />;

export function AutoscalingTab() {
  const { t } = useTranslation();
  const { summary, hpas, findings } = useView();
  const status = useCallback((row: ViewHPA) => worstSeverity(findings, matchHPA(row)) ?? "", [findings]);
  const { autoscaling } = summary;
  const columns = useMemo<Column<ViewHPA>[]>(
    () => [
      {
        id: "name",
        header: t("report.column.hpa"),
        cell: (row) => (
          <div className="flex flex-col gap-0.5">
            <span className="font-mono [overflow-wrap:anywhere]">{row.name}</span>
            <span className="text-xs text-muted-fg">
              {row.target_kind} <span className="font-mono [overflow-wrap:anywhere]">{row.target_name}</span> ·{" "}
              <span className="font-mono [overflow-wrap:anywhere]">{row.namespace}</span>
            </span>
          </div>
        ),
        sortValue: (row) => row.name,
      },
      {
        id: "status",
        header: t("report.column.status"),
        cell: (row) => {
          const worst = status(row);
          return <FindingStatus status={worst === "" ? "healthy" : worst} hasFinding={worst !== ""} />;
        },
        sortValue: (row) => severityRank[status(row)] ?? 3,
      },
      {
        id: "situation",
        header: t("report.column.situation"),
        cell: (row) =>
          row.situation_status === null ? (
            row.situation
          ) : (
            <StatusMark status={row.situation_status} label={row.situation} withStatus />
          ),
        sortValue: (row) => severityRank[row.situation_status ?? ""] ?? 3,
      },
      {
        id: "range",
        header: t("report.column.replicaRange"),
        cell: (row) => (
          <span className="font-mono">
            <ValueText value={row.spec_min} /> / <ValueText value={row.spec_max} />
          </span>
        ),
        sortValue: (row) => row.spec_min.raw,
        align: "end",
      },
      {
        id: "peak",
        header: t("report.column.replicasPeak"),
        cell: (row) => (
          <span className="font-mono">
            <ValueText value={row.current.peak} />
          </span>
        ),
        sortValue: (row) => row.current.peak.raw,
        align: "end",
      },
    ],
    [t, status],
  );
  return (
    <div className="flex flex-col gap-6">
      <section className="flex flex-col gap-3">
        <div className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
          <h2 className="text-base font-semibold">{t("report.autoscaling.title")}</h2>
          <span className="text-sm">{t(`autoscalingStatus.${autoscaling.status}`)}</span>
          <span className="text-sm text-muted-fg">
            {t("report.autoscaling.source", { value: t(`strategy.${autoscaling.source}`) })}
          </span>
          <span className="text-sm text-muted-fg">
            {t("report.confidence", { value: t(`confidence.${autoscaling.confidence}`) })}
          </span>
        </div>
        <Counts
          label={t("report.autoscaling.title")}
          items={[
            { label: t("report.autoscaling.currentNodes"), value: autoscaling.current_nodes },
            { label: t("report.autoscaling.minNodes"), value: autoscaling.min_nodes },
            { label: t("report.autoscaling.maxNodes"), value: autoscaling.max_nodes },
            { label: t("report.autoscaling.managedPools"), value: autoscaling.managed_node_groups },
            { label: t("report.autoscaling.scaleEvents"), value: autoscaling.scale_events },
            { label: t("report.autoscaling.unschedulablePods"), value: autoscaling.unschedulable_pods },
            { label: t("report.autoscaling.workloadsWithoutHpa"), value: summary.workloads_without_hpa },
          ]}
        />
      </section>
      <section className="flex flex-col gap-3">
        <h2 className="text-base font-semibold">{t("report.hpas.title")}</h2>
        <DataTable
          label={t("report.hpas.title")}
          empty={t("report.table.empty")}
          search={{ label: t("report.search.hpas"), text: searchText }}
          rows={hpas}
          rowKey={rowKey}
          rowName={rowName}
          columns={columns}
          renderOpened={renderOpened}
          status={status}
        />
      </section>
    </div>
  );
}
