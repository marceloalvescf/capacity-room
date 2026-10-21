import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link, useSearchParams } from "react-router";
import { StatusMark, StatusShape } from "@/components/status-mark";
import { cn } from "@/lib/cn";
import type { ViewScope } from "./api";
import { useView } from "./report-page";

const severities = ["critical", "attention", "healthy"] as const;
type Severity = (typeof severities)[number];

const fill: Record<Severity, string> = { critical: "fill-critical", attention: "fill-attention", healthy: "fill-healthy" };

const centre = 100;
const outer = 98;
const inner = 66;
const fullTurn = 359.99;

function point(radius: number, degrees: number): string {
  const radians = ((degrees - 90) * Math.PI) / 180;
  return `${(centre + radius * Math.cos(radians)).toFixed(3)} ${(centre + radius * Math.sin(radians)).toFixed(3)}`;
}

// The ring between two angles, clockwise from the top.
function sector(from: number, to: number): string {
  const large = to - from > 180 ? 1 : 0;
  return `M ${point(outer, from)} A ${String(outer)} ${String(outer)} 0 ${String(large)} 1 ${point(outer, to)} L ${point(inner, to)} A ${String(inner)} ${String(inner)} 0 ${String(large)} 0 ${point(inner, from)} Z`;
}

// Every finding of the report as a ring, one slice for each severity that has any, with the total at its centre and
// the three counts beside it as text.
export function Donut({ critical, attention, healthy }: { critical: number; attention: number; healthy: number }) {
  const { t } = useTranslation();
  const counts: Record<Severity, number> = { critical, attention, healthy };
  const total = critical + attention + healthy;
  const sweeps = severities.map((severity) => (counts[severity] / total) * 360);
  const slices = severities.flatMap((severity, index) => {
    if (counts[severity] === 0) {
      return [];
    }
    const from = sweeps.slice(0, index).reduce((sum, sweep) => sum + sweep, 0);
    return [{ severity, d: sector(from, from + Math.min(sweeps[index] ?? 0, fullTurn)) }];
  });
  return (
    <div className="flex flex-col items-center gap-4">
      <div className="relative w-full max-w-60">
        <svg aria-hidden="true" viewBox="0 0 200 200" className="w-full">
          {slices.map((slice) => (
            <path key={slice.severity} d={slice.d} className={cn(fill[slice.severity], "stroke-surface")} strokeWidth="2" />
          ))}
        </svg>
        <p className="absolute inset-0 flex flex-col items-center justify-center">
          <span className="font-mono text-display font-semibold">{total}</span>
          <span className="text-sm text-muted-fg">{t("report.chart.findings")}</span>
        </p>
      </div>
      <ul className="flex flex-col gap-1 text-sm">
        {severities.map((severity) => (
          <li key={severity} className="flex items-center gap-2">
            <StatusShape status={severity} />
            <span className="w-8 text-right font-mono font-semibold">{counts[severity]}</span>
            <span className="text-muted-fg">{t(`status.${severity}`)}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

const barWidth = 1000;
const gap = 2;

// The findings of one scope as a bar of its three severities, drawn on the scale shared by every row. With `link`,
// each segment filters the Findings tab; without it, the row names the worst finding and links to it.
export function ScopeBar({ scope, scale, link }: { scope: ViewScope; scale: number; link: boolean }) {
  const { t } = useTranslation();
  const { findings } = useView();
  const [params] = useSearchParams();
  const name = t(`report.overview.scope.${scope.scope_type}`);
  const lead = findings[scope.finding];
  const counts: Record<Severity, number> = { critical: scope.critical, attention: scope.attention, healthy: scope.healthy };
  const widths = severities.map((severity) => (counts[severity] / scale) * barWidth);
  const segments = severities.flatMap((severity, index) =>
    counts[severity] === 0
      ? []
      : [{ severity, count: counts[severity], at: widths.slice(0, index).reduce((sum, width) => sum + width, 0), width: widths[index] ?? 0 }],
  );
  function target(severity: Severity): string {
    const next = new URLSearchParams(params);
    next.delete("finding");
    next.set("scope", scope.scope_type);
    next.set("severity", severity);
    return `?${next.toString()}`;
  }
  return (
    <li className="grid grid-cols-[minmax(0,8rem)_minmax(0,1fr)_auto] items-start gap-x-4 gap-y-1 border-b border-border py-3 first:pt-2 last:border-0">
      <span className="pt-0.5 font-medium">
        {link ? name : <StatusMark status={scope.severity} label={name} withStatus />}
      </span>
      <div className="flex min-w-0 flex-col gap-1.5">
        <svg viewBox={`0 0 ${String(barWidth)} 16`} className="h-4 w-full" aria-hidden={link ? undefined : "true"}>
          <rect width={barWidth} height="16" rx="2" className="fill-track" />
          {segments.map((segment) => {
            const rect = (
              <rect
                x={segment.at}
                width={Math.max(segment.width - gap, 1)}
                height="16"
                className={cn(fill[segment.severity], "stroke-surface")}
                strokeWidth={gap}
              />
            );
            return link ? (
              <Link
                key={segment.severity}
                to={target(segment.severity)}
                replace
                aria-label={t("report.chart.segment", {
                  count: segment.count,
                  severity: t(`status.${segment.severity}`).toLowerCase(),
                  scope: name,
                })}
                className="focus-visible:outline-2 focus-visible:outline-accent"
              >
                {rect}
              </Link>
            ) : (
              <g key={segment.severity}>{rect}</g>
            );
          })}
        </svg>
        {link || lead === undefined ? null : (
          <Link
            to={`../findings?scope=${scope.scope_type}&finding=${String(scope.finding)}`}
            className="self-start text-sm underline decoration-border underline-offset-4 hover:decoration-accent"
          >
            {lead.sentence}
          </Link>
        )}
      </div>
      <span className="flex items-center gap-3 pt-0.5 text-sm">
        {severities.map((severity) => (
          <span key={severity} className="inline-flex items-center gap-1">
            <StatusShape status={severity} />
            <span className="sr-only">{t(`status.${severity}`)}</span>
            <span className="min-w-5 text-right font-mono text-muted-fg">{counts[severity]}</span>
          </span>
        ))}
      </span>
    </li>
  );
}

// The donut of the report's findings and the bar of each scope, side by side. `under` goes below the donut.
export function FindingsChart({ linkSegments, under }: { linkSegments: boolean; under?: ReactNode }) {
  const { t } = useTranslation();
  const { summary, findings, verdict } = useView();
  const critical = summary.critical.raw ?? 0;
  const attention = summary.attention.raw ?? 0;
  const healthy = summary.healthy.raw ?? 0;
  const empty = findings.length === 0 || critical + attention + healthy === 0;
  const scale = Math.max(1, ...verdict.scopes.map((scope) => scope.critical + scope.attention + scope.healthy));
  return (
    <div className="grid items-start gap-4 lg:grid-cols-[minmax(0,20rem)_minmax(0,1fr)]">
      <div className="flex flex-col gap-4">
        <div className="rounded-lg border border-border bg-surface p-5 text-surface-fg">
          {empty ? (
            <p className="text-sm text-muted-fg">{t("report.noFinding")}</p>
          ) : (
            <Donut critical={critical} attention={attention} healthy={healthy} />
          )}
        </div>
        {under}
      </div>
      {empty ? null : (
        <section className="rounded-lg border border-border bg-surface p-5 text-surface-fg">
          <h2 className="mb-2 text-sm font-normal text-muted-fg">
            {t(linkSegments ? "report.chart.scopesLinked" : "report.chart.scopes")}
          </h2>
          <ul>
            {verdict.scopes.map((scope) => (
              <ScopeBar key={scope.scope_type} scope={scope} scale={scale} link={linkSegments} />
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}
