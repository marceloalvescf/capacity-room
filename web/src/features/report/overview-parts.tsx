import { CaretRightIcon } from "@phosphor-icons/react";
import { Fragment, useId } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { Separator } from "@/components/separator";
import { StatusMark, StatusShape } from "@/components/status-mark";
import { cn } from "@/lib/cn";
import type { ReportView, ViewRecommendation } from "./api";
import { ClusterVerdict } from "./cluster-verdict";
import { ValueText } from "./pieces";
import { ProofBar } from "./proof-bar";

// What a part of the overview needs: the view it words, and the address of its report, `/reports/<id>`, which its
// links are built under.
export type PartProps = { view: ReportView; base: string };

const severities = ["critical", "attention", "healthy"] as const;

const minimum: Record<string, number> = { all: 0, healthy: 0, attention: 1, critical: 2 };
const rank: Record<(typeof severities)[number], number> = { healthy: 0, attention: 1, critical: 2 };

// Whether the cluster can grow: the proof comes first under the title, the nodes against the autoscaler's maximum
// when the autoscaling is enabled and both are known and its state word otherwise, then the worst finding of the
// cluster and the counts that are known.
export function GrowthPair({ view, base }: PartProps) {
  const { t } = useTranslation();
  const { summary, verdict } = view;
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
  const detail = (
    <>
      <ClusterVerdict view={view} />
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
      <Link to={`${base}/autoscaling`} className="mt-1 inline-flex items-center gap-1 self-start text-sm text-accent underline-offset-4 hover:underline">
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
  return (
    <section className="flex flex-col rounded-lg border border-border bg-surface text-surface-fg">
      <div className="flex flex-col gap-2 p-4">
        <h2 className="font-semibold">{t("report.autoscaling.title")}</h2>
        {proof}
      </div>
      <div className="flex flex-col gap-1.5 border-t border-border p-4">{detail}</div>
    </section>
  );
}

// The answer first: the findings of each severity at the size of the answer, each beside the shape that carries
// its colour, then the way to all of them. A severity the view's minimum leaves out keeps its count and offers no findings to
// see.
export function AnswerStrip({ view, base }: PartProps) {
  const { t } = useTranslation();
  const { summary, min_severity: floor } = view;
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
                <Link to={`${base}/findings?severity=${severity}`} className={cn(item, "hover:[&>span:last-child]:underline")}>
                  {content}
                </Link>
              )}
            </Fragment>
          );
        })}
      </div>
      <Link to={`${base}/findings`} className="inline-flex items-center gap-1 text-sm text-accent underline-offset-4 hover:underline">
        {t("report.overview.allFindings")}
        <CaretRightIcon aria-hidden="true" className="size-3" />
      </Link>
    </div>
  );
}

const shownWarnings = 3;

// The warnings of the data, when there are any: the first three in their words, a count of the rest, and the tab.
export function QualityNotice({ view, base }: PartProps) {
  const { t } = useTranslation();
  const labelId = useId();
  const { warnings } = view.quality;
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
      <Link to={`${base}/quality`} className="ml-5 inline-flex items-center gap-1 self-start text-accent underline-offset-4 hover:underline">
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
function PlanAction({ recommendation, base }: { recommendation: ViewRecommendation; base: string }) {
  const { t } = useTranslation();
  if (recommendation.finding === null) {
    return recommendation.action;
  }
  return (
    <Link
      to={`${base}/findings?scope=${recommendation.scope_type}&finding=${String(recommendation.finding)}`}
      aria-label={t("report.plan.actionLink", { action: recommendation.action, scope: scopeLabel(recommendation) })}
      className="underline decoration-border underline-offset-4 hover:decoration-accent"
    >
      {recommendation.action}
    </Link>
  );
}

export function Plan({ view, base, limit }: PartProps & { limit?: number }) {
  const { t } = useTranslation();
  const headingId = useId();
  const { recommendations } = view;
  const shown = limit === undefined ? recommendations : recommendations.slice(0, limit);
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
                {shown.map((recommendation, index) => (
                  <tr key={index} className="border-b border-border last:border-0">
                    <td className="px-4 py-2.5 font-mono text-muted-fg">{index + 1}</td>
                    <td className="px-4 py-2.5 whitespace-nowrap">
                      <StatusMark status={recommendation.severity} />
                    </td>
                    <td className="px-4 py-2.5">
                      <PlanAction recommendation={recommendation} base={base} />
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
          {limit === undefined ? null : (
            <Link to={`${base}/overview`} className="inline-flex items-center gap-1 self-start text-sm text-accent underline-offset-4 hover:underline">
              {t("home.plan.all")}
              <CaretRightIcon aria-hidden="true" className="size-3" />
            </Link>
          )}
        </>
      )}
    </section>
  );
}
