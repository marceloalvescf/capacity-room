import { useId, useMemo, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { StatusMark, StatusShape, type Status } from "@/components/status-mark";
import { cn } from "@/lib/cn";
import type {
  ViewFinding,
  ViewHPA,
  ViewNamespace,
  ViewNode,
  ViewNodepool,
  ViewStatistic,
  ViewValue,
  ViewWorkload,
} from "./api";
import { FindingProof } from "./proof-bar";
import { useView } from "./report-page";

// A workload's name, with its kind and namespace beneath.
export function WorkloadName({ row }: { row: ViewWorkload }) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="font-mono [overflow-wrap:anywhere]">{row.name}</span>
      <span className="flex flex-wrap gap-x-2 text-xs text-muted-fg">
        <span>{row.kind}</span>
        <span className="font-mono [overflow-wrap:anywhere]">{row.namespace}</span>
      </span>
    </div>
  );
}

// The node pool the report could not name, which reads as a word, not as a name of the cluster.
export function isUnknownPool(finding: ViewFinding): boolean {
  return finding.scope_type === "nodepool" && finding.scope_name === "unknown";
}

// The scope name as the reader sees it: none for the cluster, a word for a node pool the report could not name.
export function scopeName(finding: ViewFinding, unknownPool: string): string | null {
  if (finding.scope_type === "cluster") {
    return null;
  }
  return isUnknownPool(finding) ? unknownPool : finding.scope_name;
}

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

// The peak of a row and, under it, the share it makes of the request, each beside the shape of its status. A peak
// with no number is its placeholder word alone, muted, with no share under it.
export function PeakCell({ peak, share, word }: { peak: ViewValue; share: ViewValue; word: "request" | "requested" }) {
  const { t } = useTranslation();
  if (peak.raw === null) {
    return <span className="text-muted-fg">{peak.text}</span>;
  }
  const text =
    share.raw === null ? share.text : t(word === "request" ? "report.share.request" : "report.share.requested", { value: share.text });
  return (
    <div className="flex flex-col gap-0.5">
      <span className="font-mono">
        <ValueText value={peak} />
      </span>
      <span className="font-mono text-xs text-muted-fg">
        <ValueText value={{ ...share, text }} />
      </span>
    </div>
  );
}

// The tint of a peak cell follows the worse of the statuses the view gives the peak and its share, never a
// threshold of its own: the critical surface, the attention surface, or none.
export function peakTint(peak: ViewValue, share: ViewValue): string {
  const statuses = [peak.status, share.status];
  return statuses.includes("critical") ? "bg-critical-surface" : statuses.includes("attention") ? "bg-attention-surface" : "";
}

const statistics = ["average", "minimum", "peak", "current"] as const;

// The four statistics of each measure, one measure to a row. A grid with no visible title is named by its label.
export function StatGrid({
  title,
  label,
  rows,
}: {
  title?: string;
  label?: string;
  rows: { label: string; stat: ViewStatistic }[];
}) {
  const { t } = useTranslation();
  return (
    <table aria-label={title ?? label} className="w-full text-sm">
      <thead className="border-b border-border text-muted-fg">
        <tr>
          {title === undefined ? (
            <td />
          ) : (
            <th scope="col" className="px-3 py-1.5 text-left font-semibold text-fg">
              {title}
            </th>
          )}
          <th scope="col" className="label px-3 py-1.5 text-right">
            {t("report.stat.average")}
          </th>
          <th scope="col" className="label px-3 py-1.5 text-right">
            {t("report.stat.minimum")}
          </th>
          <th scope="col" className="label px-3 py-1.5 text-right">
            {t("report.stat.peak")}
          </th>
          <th scope="col" className="label px-3 py-1.5 text-right">
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

// The findings of the view that a row owns, each a link to the finding beside the proof its evidence draws.
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
      <h2 id={headingId} className="text-lg font-semibold">
        {t("report.tab.findings")}
      </h2>
      <ul aria-labelledby={headingId} className="flex flex-col">
        {kept.map(({ finding, index }) => (
          <li key={index} className="grid items-center gap-x-6 gap-y-1 border-b border-border py-2 last:border-0 lg:grid-cols-[1fr_20rem]">
            <Link
              to={`../findings?finding=${String(index)}`}
              className="flex flex-wrap items-baseline gap-x-3 self-start rounded-md text-sm hover:underline focus-visible:outline-2 focus-visible:outline-accent"
            >
              <StatusMark status={finding.severity} />
              <span>{finding.sentence}</span>
            </Link>
            <FindingProof finding={finding} />
          </li>
        ))}
      </ul>
    </section>
  );
}

// The scope a finding names, as one key: its type, its kind for a workload alone, and its name. A row owns the
// findings whose key equals its own, so the keys below and the matchers built on them are one definition.
function scopeKey(type: ViewFinding["scope_type"], kind: string, name: string): string {
  return JSON.stringify([type, type === "workload" ? kind : "", name]);
}

export const findingKey = (f: ViewFinding): string => scopeKey(f.scope_type, f.scope_kind, f.scope_name);
export const namespaceKey = (row: ViewNamespace): string => scopeKey("namespace", "", row.name);
export const workloadKey = (row: ViewWorkload): string => scopeKey("workload", row.kind, `${row.namespace}/${row.name}`);
export const hpaKey = (row: ViewHPA): string => scopeKey("hpa", "", `${row.namespace}/${row.name}`);
export const nodeKey = (row: ViewNode): string => scopeKey("node", "", row.name);
export const nodepoolKey = (pool: ViewNodepool): string => scopeKey("nodepool", "", pool.name);

const matchKey = (key: string) => (f: ViewFinding) => findingKey(f) === key;

export const matchNamespace = (row: ViewNamespace) => matchKey(namespaceKey(row));
export const matchWorkload = (row: ViewWorkload) => matchKey(workloadKey(row));
export const matchHPA = (row: ViewHPA) => matchKey(hpaKey(row));
export const matchNode = (row: ViewNode) => matchKey(nodeKey(row));

const severityOrder: readonly ViewFinding["severity"][] = ["critical", "attention", "healthy"];

// The worst finding of each scope key, once per view: the first of the worst severity, in the view's order. A key
// is present when its scope has a finding.
export function indexWorstFindings(findings: readonly ViewFinding[]): ReadonlyMap<string, ViewFinding> {
  const worst = new Map<string, ViewFinding>();
  for (const finding of findings) {
    const key = findingKey(finding);
    const held = worst.get(key);
    if (held === undefined || severityOrder.indexOf(finding.severity) < severityOrder.indexOf(held.severity)) {
      worst.set(key, finding);
    }
  }
  return worst;
}

// The worst severity of the findings of each scope key.
export function indexFindings(findings: readonly ViewFinding[]): ReadonlyMap<string, ViewFinding["severity"]> {
  return new Map([...indexWorstFindings(findings)].map(([key, finding]) => [key, finding.severity]));
}

export function useWorstFindingIndex(): ReadonlyMap<string, ViewFinding> {
  const { findings } = useView();
  return useMemo(() => indexWorstFindings(findings), [findings]);
}

export function useFindingIndex(): ReadonlyMap<string, ViewFinding["severity"]> {
  const { findings } = useView();
  return useMemo(() => indexFindings(findings), [findings]);
}

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

// The status of a row that has a finding is drawn as a mark; a row with none says so in plain text.
export function FindingStatus({ status, hasFinding }: { status: Status; hasFinding: boolean }) {
  const { t } = useTranslation();
  return hasFinding ? <StatusMark status={status} /> : <span className="text-muted-fg">{t("report.noFinding")}</span>;
}

// The sentence that gives a confidence and what it means.
export function ConfidenceText({ confidence }: { confidence: ViewFinding["confidence"] }) {
  const { t } = useTranslation();
  return (
    <>
      {t("report.confidence", { value: t(`confidence.${confidence}`) })} {t(`confidenceMeaning.${confidence}`)}
    </>
  );
}
