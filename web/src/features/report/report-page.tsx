import { CaretDownIcon } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { Link, Navigate, NavLink, Outlet, useOutletContext, useParams } from "react-router";
import { ApiFailure } from "@/api/client";
import { errorText, toError } from "@/api/errors";
import { useScreenTitle } from "@/app/title";
import { Alert } from "@/components/ui/alert";
import { Button, buttonStyles } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { cn } from "@/lib/cn";
import { formatTime } from "@/lib/format";
import { isLanguage } from "@/i18n/language";
import { useProfiles, useReport } from "@/features/reports/api";
import { PdfBlock } from "@/features/reports/pdf-block";
import { Gone } from "@/features/reports/progress-page";
import { usePrincipal } from "@/session/session";
import { useReportView, type ReportView } from "./api";
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
  const failed = view.error ?? report.error;
  if (failed !== null) {
    return (
      <div className="flex flex-col items-start gap-3">
        <h1 className="text-xl font-semibold">{t("progress.screen")}</h1>
        <Alert tone="error">{errorText(t, toError(failed))}</Alert>
        <Link to="/reports" className={buttonStyles()}>
          {t("progress.back")}
        </Link>
      </div>
    );
  }
  if (report.data === undefined || view.data === undefined) {
    return <h1 className="text-xl font-semibold">{t("progress.screen")}</h1>;
  }

  const data = report.data;
  const profile = profiles.data?.find((candidate) => candidate.id === data.profile_id)?.display_name ?? data.profile_id;
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-1">
        <h1 className="text-xl font-semibold">{t("progress.title", { profile })}</h1>
        <p className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-muted-fg">
          <span>
            {t("report.cluster")} <span className="font-mono text-fg">{view.data.report.cluster_name}</span>
          </span>
          <span>{view.data.report.period_text}</span>
          <span>{t(`reportProfile.${view.data.report.profile}`)}</span>
          <span>{t("progress.requested", { time: formatTime(data.created_at), username: data.owner_username })}</span>
        </p>
      </div>
      <div className="flex items-end justify-between gap-4 border-b border-border">
        <nav aria-label={t("report.tabs")} className="-mb-px flex flex-wrap gap-1">
          {reportTabs.map((tab) => (
            <NavLink
              key={tab}
              to={tab}
              className={({ isActive }) =>
                cn(
                  "border-b-2 px-3 py-2 text-sm focus-visible:outline-2 focus-visible:outline-accent",
                  isActive ? "border-accent font-semibold text-fg" : "border-transparent text-muted-fg",
                )
              }
            >
              {t(`report.tab.${tab}`)}
            </NavLink>
          ))}
        </nav>
        <Popover>
          <PopoverTrigger asChild>
            <Button className="mb-1">
              {t("progress.pdf")}
              <CaretDownIcon aria-hidden="true" className="size-4" />
            </Button>
          </PopoverTrigger>
          <PopoverContent align="end" aria-label={t("progress.pdf")} className="w-104 max-w-[calc(100vw-2rem)]">
            <PdfBlock report={data} canRequest={principal.role !== "reader"} />
          </PopoverContent>
        </Popover>
      </div>
      <Outlet context={view.data} />
    </div>
  );
}
