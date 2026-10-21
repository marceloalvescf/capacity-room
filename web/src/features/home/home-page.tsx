import { useCallback, useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useSearchParams } from "react-router";
import { errorText, toError } from "@/api/errors";
import { useScreenTitle } from "@/app/title";
import { Alert } from "@/components/ui/alert";
import { Button, buttonStyles } from "@/components/ui/button";
import { usePrincipal } from "@/session/session";
import { useHome } from "./api";
import { ClusterAnswer } from "./cluster-answer";
import { ClusterStrip } from "./cluster-strip";
import { orderClusters } from "./clusters";
import { unseen } from "./seen";

const minute = 60_000;

// The time now, read again each minute, so that the ages on the page keep up.
function useNow(): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = window.setInterval(() => {
      setNow(Date.now());
    }, minute);
    return () => {
      window.clearInterval(timer);
    };
  }, []);
  return now;
}

export function HomePage() {
  const { t } = useTranslation();
  const principal = usePrincipal();
  const home = useHome();
  const [params] = useSearchParams();
  const now = useNow();
  const [shownReports, setShownReports] = useState<Readonly<Record<string, string>>>({});
  useScreenTitle(t("home.title"));

  const userId = principal?.id;
  const clusters = home.data?.clusters;
  // The latest report each unseen cluster holds. The first sight of a cluster is recorded as it is read, which gives
  // the same answer when it is read again.
  const unseenReports = useMemo(
    () =>
      userId === undefined || clusters === undefined
        ? []
        : clusters.flatMap((cluster) => (cluster.latest !== null && unseen(userId, cluster) ? [[cluster.profile_id, cluster.latest.id] as const] : [])),
    [userId, clusters],
  );
  // A report whose answer was shown on this page is seen.
  const dots = new Set(unseenReports.filter(([profileId, reportId]) => shownReports[profileId] !== reportId).map(([profileId]) => profileId));
  const shown = useCallback((profileId: string, reportId: string) => {
    setShownReports((current) => (current[profileId] === reportId ? current : { ...current, [profileId]: reportId }));
  }, []);
  const ordered = useMemo(() => (clusters === undefined ? [] : orderClusters(clusters)), [clusters]);

  if (principal === null) {
    return null;
  }
  const wanted = params.get("cluster");
  const chosen = ordered.find((cluster) => cluster.profile_id === wanted) ?? ordered[0];
  const canRequest = principal.role !== "reader";
  const newReport =
    chosen === undefined || !canRequest ? null : (
      <Link
        to={chosen.latest === null ? `/generate?profile=${chosen.profile_id}` : `/generate?from=${chosen.latest.id}`}
        className={buttonStyles("primary")}
      >
        {t("nav.generate")}
      </Link>
    );

  let body;
  if (home.data === undefined) {
    body = home.isError ? (
      <div className="flex flex-col items-start gap-3">
        <Alert tone="error">{errorText(t, toError(home.error))}</Alert>
        <Button
          onClick={() => {
            void home.refetch();
          }}
        >
          {t("reports.retry")}
        </Button>
      </div>
    ) : (
      <p role="status" className="text-sm text-muted-fg">
        {t("reports.loading")}
      </p>
    );
  } else if (chosen === undefined) {
    body = (
      <div className="flex flex-col items-start gap-3 rounded-md border border-dashed border-input-border p-7 text-muted-fg">
        <p>{t("home.empty.title")}</p>
        {principal.role === "administrator" ? (
          <Link to="/profiles/new" className={buttonStyles("primary")}>
            {t("home.empty.admin")}
          </Link>
        ) : (
          <p>{t("home.empty.other")}</p>
        )}
      </div>
    );
  } else {
    body = (
      <>
        <ClusterStrip clusters={ordered} chosen={chosen.profile_id} unseen={dots} now={now} />
        <ClusterAnswer cluster={chosen} userId={principal.id} now={now} canRequest={canRequest} onShown={shown} />
      </>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-3xl font-semibold tracking-tight">{t("home.title")}</h1>
        {newReport}
      </div>
      {body}
    </div>
  );
}
