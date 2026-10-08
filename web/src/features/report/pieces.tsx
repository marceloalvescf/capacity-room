import { useId, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { StatusMark, StatusShape } from "@/components/status-mark";
import { cn } from "@/lib/cn";
import type { ViewFinding, ViewHPA, ViewNamespace, ViewNode, ViewStatistic, ViewValue, ViewWorkload } from "./api";
import { useView } from "./report-page";

// A value of the view: the shape of its status, the status word for screen readers, then its text.
export function ValueText({ value }: { value: ViewValue }) {
  const { t } = useTranslation();
  return (
    <span className={cn("inline-flex items-center gap-1.5", value.raw === null && "text-muted-fg")}>
      {value.status !== null && (
        <>
          <StatusShape status={value.status} />
          <span className="sr-only">{t(`status.${value.status}`)}</span>
        </>
      )}
      {value.text}
    </span>
  );
}

// A row of counts, each number above its name.
export function Counts({ label, items }: { label: string; items: { label: string; value: ViewValue }[] }) {
  return (
    <dl aria-label={label} className="flex flex-wrap gap-x-8 gap-y-3 sm:flex-nowrap">
      {items.map((item) => (
        <div key={item.label} className="flex flex-col-reverse justify-end gap-0.5">
          <dt className="text-sm leading-snug text-muted-fg">{item.label}</dt>
          <dd className="font-mono text-xl font-semibold leading-tight">
            <ValueText value={item.value} />
          </dd>
        </div>
      ))}
    </dl>
  );
}

// The peak of a row and, under it, the share it makes of the request, each beside the shape of its status.
export function PeakCell({ peak, share, word }: { peak: ViewValue; share: ViewValue; word: "request" | "requested" }) {
  const { t } = useTranslation();
  const text =
    word === "request"
      ? t("report.share.request", { value: share.text })
      : t("report.share.requested", { value: share.text });
  return (
    <div className="flex flex-col gap-0.5">
      <span className="font-mono">
        <ValueText value={peak} />
      </span>
      <span className="font-mono text-xs text-muted-fg">
        {share.raw === null ? share.text : <ValueText value={{ ...share, text }} />}
      </span>
    </div>
  );
}

const statistics = ["average", "minimum", "peak", "current"] as const;

// The four statistics of each measure, one measure to a row.
export function StatGrid({ title, rows }: { title?: string; rows: { label: string; stat: ViewStatistic }[] }) {
  const { t } = useTranslation();
  return (
    <table aria-label={title} className="w-full text-sm">
      <thead className="border-b border-border text-muted-fg">
        <tr>
          {title === undefined ? (
            <td />
          ) : (
            <th scope="col" className="px-3 py-1.5 text-left font-semibold text-fg">
              {title}
            </th>
          )}
          <th scope="col" className="px-3 py-1.5 text-right font-medium">
            {t("report.stat.average")}
          </th>
          <th scope="col" className="px-3 py-1.5 text-right font-medium">
            {t("report.stat.minimum")}
          </th>
          <th scope="col" className="px-3 py-1.5 text-right font-medium">
            {t("report.stat.peak")}
          </th>
          <th scope="col" className="px-3 py-1.5 text-right font-medium">
            {t("report.stat.current")}
          </th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => (
          <tr key={row.label} className="border-b border-border last:border-0">
            <th scope="row" className="px-3 py-1.5 text-left font-medium text-muted-fg">
              {row.label}
            </th>
            {statistics.map((name) => (
              <td key={name} className="px-3 py-1.5 text-right font-mono">
                <ValueText value={row.stat[name]} />
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

// Labelled facts, each label over its value.
export function Facts({ items }: { items: { label: string; value: ReactNode }[] }) {
  return (
    <dl className="grid grid-cols-2 gap-x-6 gap-y-3 sm:grid-cols-4">
      {items.map((item) => (
        <div key={item.label} className="flex flex-col gap-0.5">
          <dt className="text-sm text-muted-fg">{item.label}</dt>
          <dd className="text-sm">{item.value}</dd>
        </div>
      ))}
    </dl>
  );
}

// The findings of the view that a row owns, each a link to the finding.
export function RowFindings({ match }: { match: (finding: ViewFinding) => boolean }) {
  const { t } = useTranslation();
  const headingId = useId();
  const { findings } = useView();
  const kept = findings.flatMap((finding, index) => (match(finding) ? [{ finding, index }] : []));
  if (kept.length === 0) {
    return null;
  }
  return (
    <section className="flex flex-col gap-2">
      <h2 id={headingId} className="text-sm font-semibold">
        {t("report.tab.findings")}
      </h2>
      <ul aria-labelledby={headingId} className="flex flex-col gap-1">
        {kept.map(({ finding, index }) => (
          <li key={index}>
            <Link
              to={`../findings?finding=${String(index)}`}
              className="flex flex-wrap items-baseline gap-x-3 rounded-md px-2 py-1 text-sm hover:underline focus-visible:outline-2 focus-visible:outline-accent"
            >
              <StatusMark status={finding.severity} />
              <span>{finding.sentence}</span>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}

export const matchNamespace = (row: ViewNamespace) => (f: ViewFinding) =>
  f.scope_type === "namespace" && f.scope_name === row.name;

export const matchWorkload = (row: ViewWorkload) => (f: ViewFinding) =>
  f.scope_type === "workload" && f.scope_kind === row.kind && f.scope_name === `${row.namespace}/${row.name}`;

export const matchHPA = (row: ViewHPA) => (f: ViewFinding) =>
  f.scope_type === "hpa" && f.scope_name === `${row.namespace}/${row.name}`;

export const matchNode = (row: ViewNode) => (f: ViewFinding) => f.scope_type === "node" && f.scope_name === row.name;

// The CPU and memory grids of a namespace or a workload.
export function UsageGrids({ row }: { row: ViewNamespace | ViewWorkload }) {
  const { t } = useTranslation();
  return (
    <>
      <StatGrid
        title={t("report.stat.cpu")}
        rows={[
          { label: t("report.stat.usage"), stat: row.cpu_usage },
          { label: t("report.stat.request"), stat: row.cpu_requests },
          { label: t("report.stat.limit"), stat: row.cpu_limits },
          { label: t("report.stat.requestPercent"), stat: row.cpu_request_percent },
          { label: t("report.stat.limitPercent"), stat: row.cpu_limit_percent },
        ]}
      />
      <StatGrid
        title={t("report.stat.memory")}
        rows={[
          { label: t("report.stat.usage"), stat: row.memory_usage },
          { label: t("report.stat.request"), stat: row.memory_requests },
          { label: t("report.stat.limit"), stat: row.memory_limits },
          { label: t("report.stat.requestPercent"), stat: row.memory_request_percent },
          { label: t("report.stat.limitPercent"), stat: row.memory_limit_percent },
        ]}
      />
    </>
  );
}
