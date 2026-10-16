import { CaretRightIcon } from "@phosphor-icons/react";
import { Fragment, useId, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { Separator } from "@/components/separator";
import { StatusMark, StatusShape } from "@/components/status-mark";
import { cn } from "@/lib/cn";
import type { ViewPressure, ViewRecommendation } from "./api";
import { ClusterVerdict } from "./cluster-verdict";
import { isUnknownPool, scopeName, ValueText } from "./pieces";
import { proofOf } from "./proof";
import { FindingProof, ProofBar } from "./proof-bar";
import { useView } from "./report-page";

const severities = ["critical", "attention", "healthy"] as const;

const minimum: Record<string, number> = { all: 0, healthy: 0, attention: 1, critical: 2 };
const rank: Record<(typeof severities)[number], number> = { healthy: 0, attention: 1, critical: 2 };

// A claim beside its proof: the words on the left, the evidence that proves them on the right. Without a proof the
// claim takes the whole row.
function Pair({ claim, proof }: { claim: ReactNode; proof: ReactNode }) {
  return (
    <li className="grid rounded-lg border border-border bg-surface text-surface-fg lg:grid-cols-2">
      <div className={cn("flex flex-col gap-1.5 p-4", proof === null ? "lg:col-span-2" : "border-border max-lg:border-b lg:border-r")}>
        {claim}
      </div>
      {proof === null ? null : <div className="flex flex-col justify-center gap-2 p-4">{proof}</div>}
    </li>
  );
}

// Whether the cluster can grow: the worst finding of the cluster and the counts that are known, beside the nodes
// against the autoscaler's maximum when the autoscaling is enabled and both are known, and beside its state word
// otherwise.
function GrowthPair() {
  const { t } = useTranslation();
  const { summary, verdict } = useView();
  const { autoscaling } = summary;
  const { gauge } = verdict;
  const enabled = autoscaling.status === "enabled";
  const paired = enabled && autoscaling.current_nodes.raw !== null && autoscaling.max_nodes.raw !== null;
  const facts = (
    paired
      ? [
          { label: t("report.autoscaling.minNodes"), value: autoscaling.min_nodes },
          { label: t("report.autoscaling.unschedulablePods"), value: autoscaling.unschedulable_pods },
          { label: t("report.autoscaling.scaleEvents"), value: autoscaling.scale_events },
        ]
      : [
          { label: t("report.autoscaling.currentNodes"), value: autoscaling.current_nodes },
          { label: t("report.autoscaling.unschedulablePods"), value: autoscaling.unschedulable_pods },
          { label: t("report.autoscaling.scaleEvents"), value: autoscaling.scale_events },
        ]
  ).filter((fact) => fact.value.raw !== null);
  const claim = (
    <>
      <h3 className="font-semibold">{t("report.autoscaling.title")}</h3>
      <ClusterVerdict />
      {enabled && facts.length > 0 ? (
        <p className="text-sm text-muted-fg">
          {facts.map((fact, position) => (
            <Fragment key={fact.label}>
              {position > 0 ? <Separator /> : null}
              <span className="whitespace-nowrap">
                {fact.label}{" "}
                <span className="font-mono text-fg">
                  <ValueText value={fact.value} />
                </span>
              </span>
            </Fragment>
          ))}
        </p>
      ) : null}
      <Link to="../autoscaling" className="mt-1 inline-flex items-center gap-1 self-start text-sm text-accent underline-offset-4 hover:underline">
        {t("report.overview.seeAutoscaling")}
        <CaretRightIcon aria-hidden="true" className="size-3" />
      </Link>
    </>
  );
  const proof = paired ? (
    <>
      <p className="text-sm text-muted-fg">{t("report.overview.nodesCaption")}</p>
      <p className="font-mono text-display font-semibold">
        <ValueText value={autoscaling.current_nodes} />
        <span className="mx-1.5 font-normal text-muted-fg">/</span>
        <ValueText value={autoscaling.max_nodes} />
      </p>
      {gauge === null ? null : (
        <ProofBar value={gauge.current} reference={gauge.max} status={gauge.hazard} units floor={gauge.min} className="mt-1" />
      )}
    </>
  ) : (
    <p className="text-2xl font-semibold">{t(`autoscalingStatus.${autoscaling.status}`)}</p>
  );
  return <Pair claim={claim} proof={proof} />;
}

// A scope under pressure: its worst finding as the claim, with the counts of its findings, beside the finding's
// evidence drawn.
function PressurePair({ pressure }: { pressure: ViewPressure }) {
  const { t } = useTranslation();
  const { findings } = useView();
  const lead = findings[pressure.finding];
  if (lead === undefined) {
    return null;
  }
  const { scope_type: type, severity, critical, attention } = pressure;
  const counts = [
    { severity: "critical", count: critical },
    { severity: "attention", count: attention },
  ] as const;
  const name = scopeName(lead, t("report.node.unknownPool"));
  const claim = (
    <>
      <h3 className="font-semibold">
        <StatusMark status={severity} label={t(`report.overview.scope.${type}`)} withStatus />
      </h3>
      <p>
        <Link
          to={`../findings?scope=${type}&finding=${String(pressure.finding)}`}
          aria-label={name === null ? undefined : t("report.overview.pressureLink", { sentence: lead.sentence, name })}
          className="underline decoration-border underline-offset-4 hover:decoration-accent"
        >
          {lead.sentence}
        </Link>
      </p>
      {name === null ? null : (
        <p className={cn("text-sm text-muted-fg [overflow-wrap:anywhere]", !isUnknownPool(lead) && "font-mono")}>{name}</p>
      )}
      <p className="text-sm text-muted-fg">
        {counts
          .filter(({ count }) => count > 0)
          .map(({ severity: reached, count }, position) => (
            <Fragment key={reached}>
              {position > 0 ? <Separator /> : null}
              <span className="inline-flex items-center gap-1.5">
                <StatusShape status={reached} />
                {t(`status.${reached}`)} <span className="font-mono text-fg">{count}</span>
              </span>
            </Fragment>
          ))}
      </p>
    </>
  );
  return <Pair claim={claim} proof={proofOf(lead) === null ? null : <FindingProof finding={lead} />} />;
}

// Where the cluster runs short: its growth first, then the scope types under pressure in the order the view gives.
function Pressure() {
  const { t } = useTranslation();
  const headingId = useId();
  const { pressure } = useView().verdict;
  return (
    <section className="flex flex-col gap-3">
      <h2 id={headingId} className="text-lg font-semibold">
        {t("report.overview.pressure")}
      </h2>
      <ul aria-labelledby={headingId} className="flex flex-col gap-3">
        <GrowthPair />
        {pressure.map((line) => (
          <PressurePair key={line.scope_type} pressure={line} />
        ))}
      </ul>
      {pressure.length === 0 ? <p className="text-sm text-muted-fg">{t("report.overview.noPressure")}</p> : null}
    </section>
  );
}

// The answer first: the findings of each severity at the size of the answer, each beside the shape that carries
// its colour, then the way to all of them. A severity the view's minimum leaves out keeps its count and offers no findings to
// see.
function AnswerStrip() {
  const { t } = useTranslation();
  const { summary, min_severity: floor } = useView();
  const worst = severities.find((severity) => (summary[severity].raw ?? 0) > 0);
  if (worst === undefined) {
    return <p className="text-xl">{t("report.findings.none")}</p>;
  }
  return (
    <div className="flex flex-wrap items-center gap-x-8 gap-y-3">
      <div role="group" aria-label={t("report.overview.totals")} className="flex flex-wrap items-center gap-x-8 gap-y-2">
        {severities.map((severity) => {
          const item = "inline-flex items-center gap-2.5 rounded-md text-fg";
          const content = (
            <>
              <StatusShape status={severity} className="size-4" />
              <span className="font-mono text-display font-semibold">{summary[severity].text}</span>
              <span className="text-sm text-muted-fg">{t(`status.${severity}`)}</span>
            </>
          );
          return (
            <Fragment key={severity}>
              {rank[severity] < (minimum[floor] ?? 0) ? (
                <span className={item}>{content}</span>
              ) : (
                <Link to={`../findings?severity=${severity}`} className={cn(item, "hover:[&>span:last-child]:underline")}>
                  {content}
                </Link>
              )}
            </Fragment>
          );
        })}
      </div>
      <Link to="../findings" className="inline-flex items-center gap-1 text-sm text-accent underline-offset-4 hover:underline">
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
        <span className="font-mono">{warnings.length}</span>
      </p>
      <ul className="flex flex-col gap-0.5 pl-5 text-muted-fg">
        {warnings.slice(0, shownWarnings).map((warning, index) => (
          <li key={index}>{warning.text}</li>
        ))}
        {rest > 0 ? <li>{t("report.overview.moreWarnings", { count: rest })}</li> : null}
      </ul>
      <Link to="../quality" className="ml-5 inline-flex items-center gap-1 self-start text-accent underline-offset-4 hover:underline">
        {t("report.overview.seeQuality")}
        <CaretRightIcon aria-hidden="true" className="size-3" />
      </Link>
    </div>
  );
}

// The scopes of an action by their full names; a long list is the summary the view gives.
function scopeLabel(recommendation: ViewRecommendation): string {
  return recommendation.scopes.length <= 3 ? recommendation.scopes.join(", ") : recommendation.scope_text;
}

// An action of the plan leads to the finding the view names; with none, it stays plain text.
function PlanAction({ recommendation }: { recommendation: ViewRecommendation }) {
  const { t } = useTranslation();
  if (recommendation.finding === null) {
    return recommendation.action;
  }
  return (
    <Link
      to={`../findings?scope=${recommendation.scope_type}&finding=${String(recommendation.finding)}`}
      aria-label={t("report.plan.actionLink", { action: recommendation.action, scope: scopeLabel(recommendation) })}
      className="underline decoration-border underline-offset-4 hover:decoration-accent"
    >
      {recommendation.action}
    </Link>
  );
}

function Plan() {
  const { t } = useTranslation();
  const headingId = useId();
  const { recommendations } = useView();
  return (
    <section className="flex flex-col gap-3">
      <h2 id={headingId} className="text-lg font-semibold">
        {t("report.plan.title")}
      </h2>
      {recommendations.length === 0 ? (
        <p className="text-sm text-muted-fg">{t("report.plan.empty")}</p>
      ) : (
        <>
          <div className="overflow-x-auto rounded-lg border border-border bg-surface text-surface-fg">
            <table aria-labelledby={headingId} className="w-full text-sm">
              <thead className="border-b border-border text-left">
                <tr>
                  <th className="label px-4 py-2.5">{t("report.plan.order")}</th>
                  <th className="label px-4 py-2.5">{t("report.plan.priority")}</th>
                  <th className="label px-4 py-2.5">{t("report.plan.action")}</th>
                  <th className="label px-4 py-2.5">{t("report.plan.scope")}</th>
                  <th className="label px-4 py-2.5">{t("report.plan.confidence")}</th>
                </tr>
              </thead>
              <tbody>
                {recommendations.map((recommendation, index) => (
                  <tr key={index} className="border-b border-border last:border-0">
                    <td className="px-4 py-2.5 font-mono text-muted-fg">{index + 1}</td>
                    <td className="px-4 py-2.5 whitespace-nowrap">
                      <StatusMark status={recommendation.severity} />
                    </td>
                    <td className="px-4 py-2.5">
                      <PlanAction recommendation={recommendation} />
                    </td>
                    <td className="px-4 py-2.5">
                      {recommendation.scopes.length <= 3 ? (
                        <>
                          <span className="text-muted-fg">{t(`scope.${recommendation.scope_type}`)}</span>{" "}
                          <span className="font-mono break-words">{scopeLabel(recommendation)}</span>
                        </>
                      ) : (
                        scopeLabel(recommendation)
                      )}
                    </td>
                    <td className="px-4 py-2.5">{t(`confidence.${recommendation.confidence}`)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="text-sm text-muted-fg">{t("report.plan.confidenceNote")}</p>
        </>
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
    <details className="group text-sm">
      <summary className="inline-flex cursor-pointer list-none items-center gap-1.5 rounded-md text-accent focus-visible:outline-2 focus-visible:outline-accent [&::-webkit-details-marker]:hidden">
        <CaretRightIcon aria-hidden="true" className="size-3 group-open:rotate-90 motion-safe:transition-transform" />
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
    <div className="flex flex-col gap-8">
      <div className="flex flex-col gap-4 pt-4 xl:flex-row xl:items-start xl:justify-between xl:gap-10">
        <AnswerStrip />
        <QualityNotice />
      </div>
      <Pressure />
      <Plan />
      <About />
    </div>
  );
}
