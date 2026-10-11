import { CaretRightIcon } from "@phosphor-icons/react";
import { Fragment, useId } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { StatusMark, StatusShape } from "@/components/status-mark";
import { cn } from "@/lib/cn";
import type { ViewFinding, ViewRecommendation } from "./api";
import { DepthGauge, type GaugeSeverity } from "./depth-gauge";
import { findingKey, isUnknownPool, matchRecommendation, scopeName, ValueText } from "./pieces";
import { useView } from "./report-page";

const severities = ["critical", "attention", "healthy"] as const;

const minimum: Record<string, number> = { all: 0, healthy: 0, attention: 1, critical: 2 };
const rank: Record<(typeof severities)[number], number> = { healthy: 0, attention: 1, critical: 2 };

const scopeTypes = ["node", "nodepool", "namespace", "workload", "hpa"] as const;
type Severity = (typeof severities)[number];

function worstOf(list: Severity[]): Severity {
  return severities.find((severity) => list.includes(severity)) ?? "healthy";
}

// Whether the cluster can grow: its nodes against the autoscaler's maximum, and the worst finding of the cluster.
function Growth() {
  const { t } = useTranslation();
  const { summary, findings } = useView();
  const { autoscaling } = summary;
  const enabled = autoscaling.status === "enabled";
  const current = autoscaling.current_nodes.raw;
  const max = autoscaling.max_nodes.raw;
  const worst = findings
    .filter((finding) => finding.scope_type === "cluster")
    .reduce<ViewFinding | null>(
      (best, finding) => (best === null || severities.indexOf(finding.severity) < severities.indexOf(best.severity) ? finding : best),
      null,
    );
  // A full scale is a hazard only when the view gives the cluster a finding above healthy.
  const severity: GaugeSeverity = worst === null || worst.severity === "healthy" ? null : worst.severity;
  return (
    <div className="flex flex-col gap-3">
      <h2 className="label">{t("report.autoscaling.title")}</h2>
      {enabled ? (
        <div className="flex flex-col gap-1">
          <p className="font-mono text-display font-semibold">
            <ValueText value={autoscaling.current_nodes} />
            <span className="mx-2 font-sans font-normal text-muted-fg">/</span>
            <ValueText value={autoscaling.max_nodes} />
          </p>
          {current !== null && max !== null && max > 0 ? (
            <DepthGauge current={current} max={max} min={autoscaling.min_nodes.raw} severity={severity} />
          ) : null}
          <p className="text-sm text-muted-fg">{t("report.overview.nodesCaption")}</p>
        </div>
      ) : (
        <p className="text-2xl font-semibold">{t(`autoscalingStatus.${autoscaling.status}`)}</p>
      )}
      {worst === null ? null : (
        <div className="flex flex-col gap-1">
          <StatusMark status={worst.severity} />
          <p className="text-sm">{worst.sentence}</p>
        </div>
      )}
      {enabled ? (
        <p className="text-sm text-muted-fg">
          <span className="whitespace-nowrap">
            {t("report.autoscaling.minNodes")}{" "}
            <span className="font-mono italic">
              <ValueText value={autoscaling.min_nodes} />
            </span>
          </span>
          {" · "}
          <span className="whitespace-nowrap">
            {t("report.autoscaling.unschedulablePods")}{" "}
            <span className="font-mono italic">
              <ValueText value={autoscaling.unschedulable_pods} />
            </span>
          </span>
          {" · "}
          <span className="whitespace-nowrap">
            {t("report.autoscaling.scaleEvents")}{" "}
            <span className="font-mono italic">
              <ValueText value={autoscaling.scale_events} />
            </span>
          </span>
        </p>
      ) : null}
      <Link to="../autoscaling" className="inline-flex items-center gap-1 self-start text-sm text-accent underline">
        {t("report.overview.seeAutoscaling")}
        <CaretRightIcon aria-hidden="true" className="size-3" />
      </Link>
    </div>
  );
}

// One scope: its worst severity, the first finding of that severity, and how many of its rows reach each severity.
function PressureLine({ type }: { type: (typeof scopeTypes)[number] }) {
  const { t } = useTranslation();
  const { findings } = useView();
  const indexed = findings.map((finding, index) => ({ finding, index })).filter(({ finding }) => finding.scope_type === type);
  // Rows are told apart as the tabs tell them: a workload by its kind too.
  const byScope = new Map<string, Severity[]>();
  for (const { finding } of indexed) {
    const key = findingKey(finding);
    byScope.set(key, [...(byScope.get(key) ?? []), finding.severity]);
  }
  const worstByName = [...byScope.values()].map(worstOf);
  const worst = worstOf(worstByName);
  const lead = indexed.find(({ finding }) => finding.severity === worst);
  if (worst === "healthy" || lead === undefined) {
    return null;
  }
  const name = scopeName(lead.finding, t("report.node.unknownPool"));
  const peak = lead.finding.evidence.find((item) => item.name === "peak_percent");
  const counts = (["critical", "attention"] as const)
    .map((severity) => ({ severity, count: worstByName.filter((value) => value === severity).length }))
    .filter(({ count }) => count > 0);
  return (
    <li className="grid grid-cols-[8rem_1fr] gap-3 border-t border-border py-3">
      <div>
        <StatusMark status={worst} label={t(`report.overview.scope.${type}`)} withStatus />
      </div>
      <div className="flex flex-col gap-1">
        <p className="text-sm">
          <Link
            to={`../findings?scope=${type}&finding=${String(lead.index)}`}
            className="underline decoration-border underline-offset-4 hover:decoration-accent"
          >
            {lead.finding.sentence}
          </Link>
        </p>
        {name === null && peak === undefined ? null : (
          <p className="flex flex-wrap gap-x-3 text-sm">
            {name === null ? null : (
              <span className={cn("break-all", !isUnknownPool(lead.finding) && "font-mono")}>{name}</span>
            )}
            {peak === undefined ? null : (
              <span className="whitespace-nowrap">
                <span className="text-muted-fg">{t("evidence.peak_percent")}</span> <span className="font-mono">{peak.text}</span>
              </span>
            )}
          </p>
        )}
        <p className="flex flex-wrap items-center gap-x-1.5 text-sm text-muted-fg">
          {counts.map(({ severity, count }, position) => (
            <span key={severity} className="inline-flex items-center gap-1.5">
              {position > 0 ? <span aria-hidden="true">·</span> : null}
              <StatusShape status={severity} />
              {t(`status.${severity}`)} <span className="font-mono italic">{count}</span>
            </span>
          ))}
        </p>
      </div>
    </li>
  );
}

// The scopes with a finding above healthy, in a fixed order.
function Pressure() {
  const { t } = useTranslation();
  const { findings } = useView();
  const any = findings.some(
    (finding) => finding.severity !== "healthy" && (scopeTypes as readonly string[]).includes(finding.scope_type),
  );
  return (
    <div className="flex flex-col gap-3">
      <h2 className="label">{t("report.overview.pressure")}</h2>
      {any ? (
        <ul>
          {scopeTypes.map((type) => (
            <PressureLine key={type} type={type} />
          ))}
        </ul>
      ) : (
        <p className="text-sm text-muted-fg">{t("report.overview.noPressure")}</p>
      )}
    </div>
  );
}

// The top of the overview: whether the cluster can grow, then where it runs short.
function Verdict() {
  return (
    <section className="flex flex-col gap-6 border-b border-border py-6 xl:grid xl:grid-cols-[4fr_7fr] xl:gap-10">
      <Growth />
      <Pressure />
    </section>
  );
}

const statusText: Record<Severity, string> = {
  critical: "text-critical",
  attention: "text-attention",
  healthy: "text-healthy",
};

// The answer first: the findings of each severity, the worst present in its colour, then the way to all of them. A
// severity the view's minimum leaves out keeps its count and offers no findings to see.
function AnswerStrip() {
  const { t } = useTranslation();
  const { summary, min_severity: floor } = useView();
  const worst = severities.find((severity) => (summary[severity].raw ?? 0) > 0);
  if (worst === undefined) {
    return <p className="text-xl">{t("report.findings.none")}</p>;
  }
  return (
    <div className="flex flex-wrap items-center gap-x-5 gap-y-2">
      <div role="group" aria-label={t("report.overview.totals")} className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xl">
        {severities.map((severity, position) => {
          const item = cn("inline-flex items-center gap-2", severity === worst ? `${statusText[severity]} font-semibold` : "text-fg");
          const content = (
            <>
              <StatusShape status={severity} className="size-4" />
              <span>{t(`status.${severity}`)}</span>
              <span className="font-mono italic">{summary[severity].text}</span>
            </>
          );
          return (
            <Fragment key={severity}>
              {position > 0 ? (
                <span aria-hidden="true" className="text-muted-fg">
                  ·
                </span>
              ) : null}
              {rank[severity] < (minimum[floor] ?? 0) ? (
                <span className={item}>{content}</span>
              ) : (
                <Link
                  to={`../findings?severity=${severity}`}
                  className={cn(item, "hover:underline focus-visible:outline-2 focus-visible:outline-accent")}
                >
                  {content}
                </Link>
              )}
            </Fragment>
          );
        })}
      </div>
      <Link to="../findings" className="inline-flex items-center gap-1 text-sm text-accent underline">
        {t("report.overview.allFindings")}
        <CaretRightIcon aria-hidden="true" className="size-3" />
      </Link>
    </div>
  );
}

const shownWarnings = 3;

// The warnings of the data, when there are any: the first three in their words, a count of the rest, and the tab.
function QualityNotice() {
  const { t } = useTranslation();
  const labelId = useId();
  const { warnings } = useView().quality;
  if (warnings.length === 0) {
    return null;
  }
  const rest = warnings.length - shownWarnings;
  return (
    <div role="group" aria-labelledby={labelId} className="flex flex-col gap-1 text-sm">
      <p className="flex items-center gap-2">
        <StatusShape status="attention" />
        <span id={labelId} className="font-semibold">
          {t("report.tab.quality")}
        </span>
        <span className="font-mono italic">{warnings.length}</span>
      </p>
      <ul className="flex flex-col gap-0.5 pl-5">
        {warnings.slice(0, shownWarnings).map((warning, index) => (
          <li key={index}>{warning.text}</li>
        ))}
        {rest > 0 ? <li className="text-muted-fg">{t("report.overview.moreWarnings", { count: rest })}</li> : null}
      </ul>
      <Link to="../quality" className="ml-5 inline-flex items-center gap-1 self-start text-accent underline">
        {t("report.overview.seeQuality")}
        <CaretRightIcon aria-hidden="true" className="size-3" />
      </Link>
    </div>
  );
}

// An action of the plan leads to the finding it comes from; with none to match, it stays plain text.
function PlanAction({ recommendation, findings }: { recommendation: ViewRecommendation; findings: ViewFinding[] }) {
  const index = findings.findIndex(matchRecommendation(recommendation));
  if (index < 0) {
    return recommendation.action;
  }
  return (
    <Link
      to={`../findings?scope=${recommendation.scope_type}&finding=${String(index)}`}
      className="underline decoration-border underline-offset-4 hover:decoration-accent"
    >
      {recommendation.action}
    </Link>
  );
}

function Plan() {
  const { t } = useTranslation();
  const headingId = useId();
  const { recommendations, findings } = useView();
  return (
    <section className="flex flex-col gap-2">
      <h2 id={headingId} className="text-base font-semibold">
        {t("report.plan.title")}
      </h2>
      {recommendations.length === 0 ? (
        <p className="text-sm text-muted-fg">{t("report.plan.empty")}</p>
      ) : (
        <div className="overflow-x-auto rounded-md border border-border bg-surface text-surface-fg">
          <table aria-labelledby={headingId} className="w-full text-sm">
            <thead className="border-b border-border text-left">
              <tr>
                <th className="label px-3 py-2">{t("report.plan.order")}</th>
                <th className="label px-3 py-2">{t("report.plan.priority")}</th>
                <th className="label px-3 py-2">{t("report.plan.action")}</th>
                <th className="label px-3 py-2">{t("report.plan.scope")}</th>
                <th className="label px-3 py-2">{t("report.plan.confidence")}</th>
              </tr>
            </thead>
            <tbody>
              {recommendations.map((recommendation, index) => (
                <tr key={index} className="border-b border-border last:border-0">
                  <td className="px-3 py-2 font-mono">{index + 1}</td>
                  <td className="px-3 py-2 whitespace-nowrap">
                    <StatusMark status={recommendation.severity} />
                  </td>
                  <td className="px-3 py-2">
                    <PlanAction recommendation={recommendation} findings={findings} />
                  </td>
                  <td className="px-3 py-2 font-mono whitespace-nowrap">{recommendation.scope_text}</td>
                  <td className="px-3 py-2">{t(`confidence.${recommendation.confidence}`)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

function About() {
  const { t } = useTranslation();
  const view = useView();
  const rows: { label: string; value: string; mono: boolean }[] = [
    { label: t("report.about.timezone"), value: view.report.timezone, mono: true },
    { label: t("report.about.namespaces"), value: view.report.namespaces.join(", "), mono: true },
    { label: t("report.about.minSeverity"), value: t(`minSeverity.${view.min_severity}`), mono: false },
    { label: t("report.about.issued"), value: view.report.emitted_at_text, mono: true },
    { label: t("report.about.version"), value: view.report.generator_version, mono: true },
  ];
  return (
    <details className="text-sm">
      <summary className="cursor-pointer text-accent focus-visible:outline-2 focus-visible:outline-accent">
        {t("report.about.title")}
      </summary>
      <dl className="mt-2 grid grid-cols-[max-content_1fr] gap-x-6 gap-y-1">
        {rows.map((row) => (
          <div key={row.label} className="contents">
            <dt className="text-muted-fg">{row.label}</dt>
            <dd className={cn(row.mono && "font-mono")}>{row.value}</dd>
          </div>
        ))}
      </dl>
    </details>
  );
}

export function OverviewTab() {
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-3 pt-2">
        <AnswerStrip />
        <QualityNotice />
      </div>
      <Verdict />
      <Plan />
      <About />
    </div>
  );
}
