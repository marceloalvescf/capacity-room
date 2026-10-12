import { DatedText } from "@/components/dated-text";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useNavigate, useParams } from "react-router";
import { ApiFailure } from "@/api/client";
import { errorText, toError } from "@/api/errors";
import { profileTestCodeValues, reportPhaseValues } from "@/api/schema.gen";
import { useAnnounce } from "@/app/live-region";
import { useScreenTitle } from "@/app/title";
import { StatusMark } from "@/components/status-mark";
import { Alert } from "@/components/ui/alert";
import { Button, buttonStyles } from "@/components/ui/button";
import { cn } from "@/lib/cn";
import { formatDate, formatElapsed, formatTime } from "@/lib/format";
import { usePrincipal } from "@/session/session";
import { canDelete, useDeleteReport, useProfiles, useReport, type Report } from "./api";
import { DeleteDialog } from "./delete-dialog";
import { reportMark } from "./marks";
import { PdfBlock } from "./pdf-block";

function Elapsed({ since }: { since: string }) {
  const { t } = useTranslation();
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = setInterval(() => {
      setNow(Date.now());
    }, 1000);
    return () => {
      clearInterval(timer);
    };
  }, []);
  return (
    <p className="font-mono text-sm text-muted-fg">
      {t("progress.elapsed", { time: formatElapsed(now - new Date(since).getTime()) })}
    </p>
  );
}

function Steps({ phase }: { phase: Report["phase"] }) {
  const { t } = useTranslation();
  const current = reportPhaseValues.indexOf(phase);
  return (
    <ol aria-label={t("progress.steps")} className="flex flex-col gap-1.5">
      {reportPhaseValues.map((name, index) => (
        <li
          key={name}
          aria-current={index === current ? "step" : undefined}
          className={cn("flex items-center gap-2 text-sm", index === current ? "font-semibold" : "text-muted-fg")}
        >
          <svg aria-hidden="true" viewBox="0 0 12 12" className="size-3 shrink-0 text-accent">
            {index < current ? (
              <circle cx="6" cy="6" r="5" fill="currentColor" />
            ) : (
              <circle
                cx="6"
                cy="6"
                r="4.25"
                fill="none"
                stroke="currentColor"
                strokeWidth={index === current ? 3 : 1.5}
                className={index === current ? "motion-safe:animate-pulse" : undefined}
              />
            )}
          </svg>
          {t(`reportPhase.${name}`)}
          {index === current ? null : (
            <span className="sr-only">{`, ${t(index < current ? "progress.stepDone" : "progress.stepPending")}`}</span>
          )}
        </li>
      ))}
    </ol>
  );
}

// Announces a report turning collected, and each PDF turning ready, once. A
// failure is not announced: its alert, which mounts with it, is read instead.
function useAnnouncements(data: Report | undefined) {
  const { t } = useTranslation();
  const announce = useAnnounce();
  const before = useRef<Report | undefined>(undefined);
  useEffect(() => {
    const last = before.current;
    before.current = data;
    if (data === undefined || last === undefined || last === data) {
      return;
    }
    if (last.status !== data.status && data.status === "collected") {
      announce(t("progress.announce.collected"));
    }
    for (const state of data.pdfs) {
      if (state.status === "ready" && last.pdfs.some((p) => p.language === state.language && p.status === "pending")) {
        announce(t("progress.announce.pdfReady", { language: t(`language.${state.language}`) }));
      }
    }
  }, [data, announce, t]);
}

export function Gone({ message }: { message: string }) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col items-start gap-3">
      <h1 className="text-xl font-semibold">{t("progress.screen")}</h1>
      <Alert tone="error">{message}</Alert>
      <Link to="/reports" className={buttonStyles()}>
        {t("progress.back")}
      </Link>
    </div>
  );
}

export function ProgressPage() {
  const { t } = useTranslation();
  const { id = "" } = useParams();
  const principal = usePrincipal();
  const [deleting, setDeleting] = useState(false);
  const report = useReport(id, !deleting);
  const profiles = useProfiles();
  const deleteReport = useDeleteReport();
  const navigate = useNavigate();
  const [asking, setAsking] = useState(false);
  const [problem, setProblem] = useState<string | null>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  useScreenTitle(t("progress.screen"));
  useAnnouncements(report.data);

  if (principal === null) {
    return null;
  }
  const { data, error } = report;
  if (error instanceof ApiFailure && (error.status === 404 || error.status === 410)) {
    return <Gone message={t(error.status === 404 ? "progress.notFound" : "progress.expired")} />;
  }
  if (data === undefined) {
    return error === null ? null : (
      <div className="flex flex-col items-start gap-3">
        <h1 className="text-xl font-semibold">{t("progress.screen")}</h1>
        <Alert tone="error">{errorText(t, toError(error))}</Alert>
      </div>
    );
  }

  const profile = profiles.data?.find((candidate) => candidate.id === data.profile_id)?.display_name ?? data.profile_id;
  const canRequest = principal.role !== "reader";
  const profileCode = data.failure_code !== null && (profileTestCodeValues as readonly string[]).includes(data.failure_code);
  const checkProfile = profileCode && principal.role === "administrator";
  const finished = data.status === "collected" || data.status === "failed";

  async function confirm() {
    setProblem(null);
    setDeleting(true);
    try {
      await deleteReport.mutateAsync(id);
    } catch (thrown) {
      // A report already gone counts as gone.
      if (!(thrown instanceof ApiFailure && thrown.status === 404)) {
        setProblem(errorText(t, toError(thrown)));
        setDeleting(false);
        setAsking(false);
        return;
      }
    }
    await navigate("/reports");
  }

  return (
    <div className="flex max-w-3xl flex-col gap-4">
      <div className="flex flex-col gap-1">
        <h1 className="text-xl font-semibold">{t("progress.title", { profile })}</h1>
        <p className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-muted-fg">
          <span><DatedText i18nKey="progress.requested" values={{ time: formatTime(data.created_at), username: data.owner_username }} /></span>
          <StatusMark status={reportMark(data)} label={t(`reportStatus.${data.status}`)} />
          {data.status === "collected" ? (
            <span><DatedText i18nKey="progress.keptUntil" values={{ date: formatDate(data.expires_at) }} /></span>
          ) : null}
          {data.status === "collected" && !data.expired ? (
            <Link to={`/reports/${data.id}/overview`} className={buttonStyles("primary")}>
              {t("report.open")}
            </Link>
          ) : null}
        </p>
      </div>
      {problem === null ? null : <Alert tone="error">{problem}</Alert>}
      {finished ? null : (
        <section className="flex flex-col gap-3">
          <Steps phase={data.phase} />
          <Elapsed since={data.created_at} />
          <p className="text-sm text-muted-fg">{t("progress.closeable")}</p>
        </section>
      )}
      {data.status === "collected" ? <PdfBlock report={data} canRequest={canRequest} /> : null}
      {data.status === "failed" ? (
        <>
          <Alert tone="error">
            <p>{data.failure_code === null ? t("error.general") : t(`reportFailure.${data.failure_code}`)}</p>
            {data.failure_code === null ? null : (
              <p className="font-mono">{t("progress.code", { code: data.failure_code })}</p>
            )}
          </Alert>
          <div className="flex flex-wrap gap-3">
            <Link to="/reports" className={buttonStyles()}>
              {t("progress.back")}
            </Link>
            {checkProfile ? (
              <Link to={`/profiles/${data.profile_id}/edit`} className={buttonStyles("primary")}>
                {t("progress.checkProfile")}
              </Link>
            ) : null}
            {canRequest ? (
              <Link to={`/generate?from=${data.id}`} className={buttonStyles(checkProfile ? undefined : "primary")}>
                {t("progress.requestAgain")}
              </Link>
            ) : null}
          </div>
          {profileCode && principal.role !== "administrator" ? (
            <p className="text-sm text-muted-fg">{t("progress.askAdministrator")}</p>
          ) : null}
        </>
      ) : null}
      {(data.status === "collected" || data.status === "failed") && canDelete(principal, data) ? (
        <div>
          <Button
            ref={trigger}
            onClick={() => {
              setAsking(true);
            }}
          >
            {t("progress.deleteReport")}
          </Button>
        </div>
      ) : null}
      <DeleteDialog
        target={asking ? { kind: "one", id: data.id, profile, time: formatTime(data.created_at), username: data.owner_username } : null}
        busy={deleteReport.isPending}
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          trigger.current?.focus();
        }}
        onConfirm={() => {
          void confirm();
        }}
        onCancel={() => {
          setAsking(false);
        }}
      />
    </div>
  );
}
