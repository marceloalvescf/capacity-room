import { CaretDownIcon } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Link, Navigate, NavLink, Outlet, useOutletContext, useParams } from "react-router";
import { ApiFailure } from "@/api/client";
import { errorText, toError } from "@/api/errors";
import { useScreenTitle } from "@/app/title";
import { StatusShape } from "@/components/status-mark";
import { Alert } from "@/components/ui/alert";
import { Button, buttonStyles } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { cn } from "@/lib/cn";
import { isLanguage } from "@/i18n/language";
import { useProfiles, useReport } from "@/features/reports/api";
import { PdfBlock } from "@/features/reports/pdf-block";
import { Gone } from "@/features/reports/progress-page";
import { usePrincipal } from "@/session/session";
import { useReportView, type ReportView } from "./api";
import { ReportMeta } from "./report-meta";
import { reportTabs } from "./tabs";

// The view the open tab shows.
export function useView(): ReportView {
  return useOutletContext<ReportView>();
}

function isGone(error: unknown): error is ApiFailure {
  return error instanceof ApiFailure && (error.status === 404 || error.status === 410);
}

export function ReportPage() {
  const { t, i18n } = useTranslation();
  const { id = "" } = useParams();
  const principal = usePrincipal();
  const language = isLanguage(i18n.language) ? i18n.language : "en";
  const report = useReport(id);
  const view = useReportView(id, language);
  const profiles = useProfiles();
  useScreenTitle(t("progress.screen"));

  if (principal === null) {
    return null;
  }
  const gone = [report.error, view.error].find(isGone);
  if (gone !== undefined) {
    return <Gone message={t(gone.status === 404 ? "progress.notFound" : "progress.expired")} />;
  }
  if (view.error instanceof ApiFailure && view.error.status === 409) {
    return <Navigate to={`/jobs/${id}`} replace />;
  }
  if (report.isPending || view.isPending) {
    return <h1 className="text-3xl font-semibold tracking-tight">{t("progress.screen")}</h1>;
  }
  // A view or a report that failed to reload leaves the one already held in place.
  const failed = (view.data === undefined ? view.error : null) ?? (report.data === undefined ? report.error : null);
  if (failed !== null) {
    return (
      <div className="flex flex-col items-start gap-3">
        <h1 className="text-3xl font-semibold tracking-tight">{t("progress.screen")}</h1>
        <Alert tone="error">{errorText(t, toError(failed))}</Alert>
        <Link to="/reports" className={buttonStyles()}>
          {t("progress.back")}
        </Link>
      </div>
    );
  }
  if (report.data === undefined || view.data === undefined) {
    return <h1 className="text-3xl font-semibold tracking-tight">{t("progress.screen")}</h1>;
  }

  const data = report.data;
  const warnings = view.data.quality.warnings.length;
  const profile = profiles.data?.find((candidate) => candidate.id === data.profile_id)?.display_name ?? data.profile_id;
  return (
    <div className="flex flex-col gap-5">
      <div className="flex items-start justify-between gap-4">
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <h1 className="text-3xl font-semibold tracking-tight">{t("progress.title", { profile })}</h1>
          <ReportMeta view={view.data} createdAt={data.created_at} owner={data.owner_username} />
        </div>
        <Popover>
          <PopoverTrigger asChild>
            <Button className="shrink-0">
              {t("progress.pdf")}
              <CaretDownIcon aria-hidden="true" className="size-4" />
            </Button>
          </PopoverTrigger>
          <PopoverContent align="end" aria-label={t("progress.pdf")} className="w-104 max-w-[calc(100vw-2rem)]">
            <PdfBlock report={data} canRequest={principal.role !== "reader"} />
          </PopoverContent>
        </Popover>
      </div>
      <div className="border-b border-border">
        <nav aria-label={t("report.tabs")} className="-mb-px flex flex-wrap">
          {reportTabs.map((tab) => {
            // The data quality tab carries the count of warnings; its name keeps the visible word and adds the count.
            const counted = tab === "quality" && warnings > 0;
            return (
              <NavLink
                key={tab}
                to={tab}
                className={({ isActive }) =>
                  cn(
                    "inline-flex items-center gap-1.5 border-b-2 px-2.5 py-2.5 text-sm font-medium focus-visible:outline-2 focus-visible:outline-accent",
                    isActive ? "border-fg text-fg" : "border-transparent text-muted-fg hover:border-input-border hover:text-fg",
                  )
                }
              >
                {counted ? (
                  // One text names the tab with its count, so no reader joins the parts with a stray space.
                  <>
                    <span aria-hidden="true">{t(`report.tab.${tab}`)}</span>
                    <span aria-hidden="true" className="inline-flex items-center gap-1 font-mono text-xs text-muted-fg">
                      <StatusShape status="attention" className="size-2.5" />
                      {warnings}
                    </span>
                    <span className="sr-only">{t("report.quality.tabName", { tab: t(`report.tab.${tab}`), count: warnings })}</span>
                  </>
                ) : (
                  t(`report.tab.${tab}`)
                )}
              </NavLink>
            );
          })}
        </nav>
      </div>
      <Outlet context={view.data} />
    </div>
  );
}
