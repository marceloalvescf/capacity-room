import { CaretRightIcon } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { useParams } from "react-router";
import { FindingsChart } from "./findings-chart";
import { AnswerStrip, GrowthPair, Plan, QualityNotice } from "./overview-parts";
import { useView } from "./report-page";

function About() {
  const { t } = useTranslation();
  const view = useView();
  const rows: { label: string; value: string }[] = [
    { label: t("report.about.timezone"), value: view.report.timezone },
    { label: t("report.about.namespaces"), value: view.report.namespaces.join(", ") },
    { label: t("report.about.issued"), value: view.report.emitted_at_text },
    { label: t("report.about.version"), value: view.report.generator_version },
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
            <dd className="font-mono">{row.value}</dd>
          </div>
        ))}
      </dl>
    </details>
  );
}

export function OverviewTab() {
  const view = useView();
  const base = `/reports/${useParams().id ?? ""}`;
  return (
    <div className="flex flex-col gap-8">
      <div className="flex flex-col gap-4 pt-4 xl:flex-row xl:items-start xl:justify-between xl:gap-10">
        <AnswerStrip view={view} base={base} />
        <QualityNotice view={view} base={base} />
      </div>
      <FindingsChart view={view} base={base} linkSegments={false} under={<GrowthPair view={view} base={base} />} />
      <Plan view={view} base={base} />
      <About />
    </div>
  );
}
