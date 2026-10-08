import { CaretRightIcon } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { StatusMark } from "@/components/status-mark";
import { cn } from "@/lib/cn";
import { Counts } from "./pieces";
import { useView } from "./report-page";

const severities = ["critical", "attention", "healthy"] as const;

function Tiles() {
  const { t } = useTranslation();
  const { summary } = useView();
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
      {severities.map((severity) => (
        <div key={severity} className="flex flex-col gap-1 rounded-md border border-border bg-surface p-4 text-surface-fg">
          <StatusMark status={severity} />
          <span className="font-mono text-3xl font-semibold">{summary[severity].text}</span>
          <Link to={`../findings?severity=${severity}`} className="inline-flex items-center gap-1 text-sm text-accent underline">
            {t("report.overview.seeFindings")}
            <CaretRightIcon aria-hidden="true" className="size-3" />
          </Link>
        </div>
      ))}
    </div>
  );
}

function Inventory() {
  const { t } = useTranslation();
  const { summary } = useView();
  return (
    <div className="rounded-md border border-border bg-surface p-4 text-surface-fg">
      <Counts
        label={t("report.overview.inventory")}
        items={[
          { label: t("report.inventory.namespaces"), value: summary.namespaces },
          { label: t("report.inventory.workloads"), value: summary.workloads },
          { label: t("report.inventory.hpas"), value: summary.hpas },
          { label: t("report.inventory.workloadsWithoutHpa"), value: summary.workloads_without_hpa },
          { label: t("report.inventory.nodes"), value: summary.nodes },
          { label: t("report.inventory.nodepools"), value: summary.nodepools },
        ]}
      />
    </div>
  );
}

function Plan() {
  const { t } = useTranslation();
  const { recommendations } = useView();
  return (
    <section className="flex flex-col gap-2">
      <h2 className="text-base font-semibold">{t("report.plan.title")}</h2>
      {recommendations.length === 0 ? (
        <p className="text-sm text-muted-fg">{t("report.plan.empty")}</p>
      ) : (
        <div className="overflow-x-auto rounded-md border border-border bg-surface text-surface-fg">
          <table className="w-full text-sm">
            <thead className="border-b border-border text-left text-muted-fg">
              <tr>
                <th className="px-3 py-2 font-medium">{t("report.plan.order")}</th>
                <th className="px-3 py-2 font-medium">{t("report.plan.priority")}</th>
                <th className="px-3 py-2 font-medium">{t("report.plan.action")}</th>
                <th className="px-3 py-2 font-medium">{t("report.plan.scope")}</th>
                <th className="px-3 py-2 font-medium">{t("report.plan.confidence")}</th>
              </tr>
            </thead>
            <tbody>
              {recommendations.map((recommendation, index) => (
                <tr key={index} className="border-b border-border last:border-0">
                  <td className="px-3 py-2 font-mono">{index + 1}</td>
                  <td className="px-3 py-2 whitespace-nowrap">
                    <StatusMark status={recommendation.severity} />
                  </td>
                  <td className="px-3 py-2">{recommendation.action}</td>
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
    { label: t("report.about.issued"), value: view.report.emitted_at_text, mono: false },
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
      <Tiles />
      <Inventory />
      <Plan />
      <About />
    </div>
  );
}
