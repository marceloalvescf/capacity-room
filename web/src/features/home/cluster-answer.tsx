import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { ApiFailure } from "@/api/client";
import { errorText, toError } from "@/api/errors";
import { Alert } from "@/components/ui/alert";
import { buttonStyles } from "@/components/ui/button";
import { cn } from "@/lib/cn";
import { formatAge } from "@/lib/format";
import { isLanguage } from "@/i18n/language";
import { useReportView, type ReportView } from "@/features/report/api";
import { FindingsChart } from "@/features/report/findings-chart";
import { AnswerStrip, GrowthPair, Plan, QualityNotice } from "@/features/report/overview-parts";
import { ReportMeta } from "@/features/report/report-meta";
import { homeKey } from "./api";
import { chipOf, type HomeCluster } from "./clusters";
import { markSeen } from "./seen";

const link = "inline-flex items-center gap-1 text-sm text-accent underline-offset-4 hover:underline";
const flightKeys = {
  queued: "home.flight.queued",
  collecting: "home.flight.collecting",
  rendering: "home.flight.rendering",
} as const;

// A request in flight, or the newest request failed: one line above the answer, with the way to its progress page.
function Notice({ cluster, now }: { cluster: HomeCluster; now: number }) {
  const { t, i18n } = useTranslation();
  const { newest } = cluster;
  const { line } = chipOf(cluster);
  if (newest === null) {
    return null;
  }
  if (line.kind === "step") {
    return (
      <Alert tone="notice">
        <p className="flex flex-wrap items-center gap-x-3">
          <span>{t(flightKeys[line.step], { age: formatAge(newest.created_at, now, i18n.language) })}</span>
          <Link to={`/jobs/${newest.id}`} className={link}>
            {t("home.flight.follow")}
          </Link>
        </p>
      </Alert>
    );
  }
  if (newest.status !== "failed") {
    return null;
  }
  const reason = newest.failure_code === null ? t("error.general") : t(`reportFailure.${newest.failure_code}`);
  return (
    <Alert tone="error">
      <p className="flex flex-wrap items-center gap-x-3">
        <span>{t("home.failed.notice", { reason })}</span>
        <Link to={`/jobs/${newest.id}`} className={link}>
          {t("home.failed.open")}
        </Link>
      </p>
    </Alert>
  );
}

function isGone(error: unknown): error is ApiFailure {
  return error instanceof ApiFailure && (error.status === 404 || error.status === 409 || error.status === 410);
}

// The overview's parts for the cluster's latest report. The view of the previous cluster stays in place while the
// next one loads, and the report counts as seen once its own view is on screen.
function LatestAnswer({
  latest,
  profileId,
  userId,
  onShown,
}: {
  latest: NonNullable<HomeCluster["latest"]>;
  profileId: string;
  userId: string;
  onShown: (profileId: string, reportId: string) => void;
}) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const language = isLanguage(i18n.language) ? i18n.language : "en";
  const view = useReportView(latest.id, language);
  const [held, setHeld] = useState<{ id: string; view: ReportView } | undefined>();
  if (view.data !== undefined && view.data !== held?.view) {
    setHeld({ id: latest.id, view: view.data });
  }
  const arrived = view.data !== undefined && !view.isPlaceholderData;
  const gone = view.data === undefined && isGone(view.error) ? view.error : null;
  useEffect(() => {
    if (arrived) {
      markSeen(userId, profileId, latest.id);
      onShown(profileId, latest.id);
    }
  }, [arrived, userId, profileId, latest.id, onShown]);
  useEffect(() => {
    if (gone !== null) {
      void queryClient.invalidateQueries({ queryKey: homeKey });
    }
  }, [gone, queryClient]);

  if (gone !== null) {
    return <Alert tone="notice">{t("home.answer.gone")}</Alert>;
  }
  if (view.isError && view.data === undefined) {
    return <Alert tone="error">{errorText(t, toError(view.error))}</Alert>;
  }
  // The view of the previous cluster stands only while this one loads.
  const shown = view.data === undefined ? held : { id: latest.id, view: view.data };
  if (shown === undefined) {
    return (
      <p role="status" className="text-sm text-muted-fg">
        {t("home.answer.loading")}
      </p>
    );
  }
  const base = `/reports/${shown.id}`;
  return (
    <div aria-busy={view.isPending} className="flex flex-col gap-6">
      {view.isPending ? (
        <p role="status" className="text-sm text-muted-fg">
          {t("home.answer.loading")}
        </p>
      ) : null}
      {view.data === undefined ? null : (
        <ReportMeta view={view.data} createdAt={latest.created_at} owner={latest.owner_username} selection={latest.selection?.namespaces} />
      )}
      <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between xl:gap-10">
        <AnswerStrip view={shown.view} base={base} />
        <QualityNotice view={shown.view} base={base} />
      </div>
      <FindingsChart view={shown.view} base={base} linkSegments={false} under={<GrowthPair view={shown.view} base={base} />} />
      <Plan view={shown.view} base={base} limit={3} />
    </div>
  );
}

// What the cluster has to say when it has no report to show.
function NoAnswer({ cluster, canRequest }: { cluster: HomeCluster; canRequest: boolean }) {
  const { t } = useTranslation();
  const expired = cluster.newest !== null && cluster.newest.status === "collected" && cluster.newest.expired;
  return (
    <div className="flex flex-col items-start gap-3 rounded-md border border-dashed border-input-border p-7 text-muted-fg">
      <p>{expired ? t("home.answer.expired") : t("home.answer.none")}</p>
      {canRequest ? (
        <Link to={`/generate?profile=${cluster.profile_id}`} className={buttonStyles("primary")}>
          {t("home.answer.request")}
        </Link>
      ) : null}
    </div>
  );
}

// The chosen cluster: its name, the way into its latest report, and the answer of that report.
export function ClusterAnswer({
  cluster,
  userId,
  now,
  canRequest,
  onShown,
}: {
  cluster: HomeCluster;
  userId: string;
  now: number;
  canRequest: boolean;
  onShown: (profileId: string, reportId: string) => void;
}) {
  const { t } = useTranslation();
  const headingId = useId();
  const { latest } = cluster;
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-4">
      <Notice cluster={cluster} now={now} />
      <div className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
        <h2 id={headingId} className="text-lg font-semibold">
          {cluster.display_name}
        </h2>
        <span className="font-mono text-sm text-muted-fg">{cluster.profile_id}</span>
        {latest === null ? null : (
          <Link to={`/reports/${latest.id}/overview`} className={cn(link, "ml-auto")}>
            {t("home.answer.open")}
          </Link>
        )}
      </div>
      {latest === null ? (
        <NoAnswer cluster={cluster} canRequest={canRequest} />
      ) : (
        <LatestAnswer latest={latest} profileId={cluster.profile_id} userId={userId} onShown={onShown} />
      )}
    </section>
  );
}
