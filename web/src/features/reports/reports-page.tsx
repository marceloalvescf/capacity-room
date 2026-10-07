import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { ApiFailure } from "@/api/client";
import { errorText, toError } from "@/api/errors";
import { useAnnounce } from "@/app/live-region";
import { useScreenTitle } from "@/app/title";
import { StatusMark } from "@/components/status-mark";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { formatDate, formatTime } from "@/lib/format";
import { usePrincipal } from "@/session/session";
import {
  canDelete,
  isActive,
  useDeleteReport,
  useDeleteReports,
  useProfiles,
  useReports,
  type ReportSummary,
} from "./api";
import { DeleteDialog, type DeleteTarget } from "./delete-dialog";
import { pdfMark, reportMark } from "./marks";

const linkButton =
  "inline-flex min-h-9 items-center justify-center rounded-md border border-accent bg-accent px-3 text-sm font-medium text-accent-fg focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent";

export function ReportsPage() {
  const { t, i18n } = useTranslation();
  const principal = usePrincipal();
  const reports = useReports();
  const profiles = useProfiles();
  const deleteOne = useDeleteReport();
  const deleteMany = useDeleteReports();
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const [target, setTarget] = useState<DeleteTarget | null>(null);
  const [failure, setFailure] = useState<string | null>(null);
  const [forbidden, setForbidden] = useState(false);
  const announce = useAnnounce();
  const heading = useRef<HTMLHeadingElement>(null);
  const focusHeading = useRef(false);
  const opener = useRef<Element | null>(null);
  useScreenTitle(t("reports.title"));

  const rows = reports.data?.pages.flatMap((page) => page.items) ?? [];
  // The selection keeps only the rows still shown: it is pruned when the data changes.
  const [prunedFor, setPrunedFor] = useState(reports.data);
  if (reports.data !== prunedFor) {
    setPrunedFor(reports.data);
    if (reports.data !== undefined) {
      const shown = new Set(rows.map((row) => row.id));
      const kept = [...selected].filter((id) => shown.has(id));
      if (kept.length !== selected.size) {
        setSelected(new Set(kept));
      }
    }
  }

  if (principal === null) {
    return null;
  }

  const names = new Map(profiles.data?.map((profile) => [profile.id, profile.display_name]));
  const profileName = (row: ReportSummary): string => names.get(row.profile_id) ?? row.profile_id;
  const deletable = rows.filter((row) => canDelete(principal, row));
  const chosen = deletable.filter((row) => selected.has(row.id));
  const canRequest = principal.role !== "reader";
  const updating = rows.some(isActive);
  const busy = deleteOne.isPending || deleteMany.isPending;

  function toggle(id: string, on: boolean) {
    setSelected((current) => {
      const next = new Set(current);
      if (on) {
        next.add(id);
      } else {
        next.delete(id);
      }
      return next;
    });
  }

  // The dialog is not opened by a trigger of its own, so the page remembers
  // what opened it and gives focus back there.
  function open(next: DeleteTarget) {
    opener.current = document.activeElement;
    setTarget(next);
  }

  function askBulk() {
    const [only] = chosen;
    if (only === undefined) {
      return;
    }
    open(
      chosen.length === 1
        ? { kind: "one", report: only, profile: profileName(only) }
        : { kind: "many", ids: chosen.map((row) => row.id) },
    );
  }

  async function confirm() {
    const current = target;
    if (current === null || busy) {
      return;
    }
    setFailure(null);
    setForbidden(false);
    let went = 0;
    try {
      if (current.kind === "one") {
        try {
          await deleteOne.mutateAsync(current.report.id);
        } catch (thrown) {
          // A report already gone counts as gone.
          if (!(thrown instanceof ApiFailure && thrown.status === 404)) {
            throw thrown;
          }
        }
        toggle(current.report.id, false);
        went = 1;
      } else {
        const results = await deleteMany.mutateAsync(current.ids);
        const refused = results.filter((result) => result.outcome === "forbidden").map((result) => result.id);
        setSelected(new Set(refused));
        setForbidden(refused.length > 0);
        went = results.length - refused.length;
      }
      announce(t("reports.deleted", { count: went }));
    } catch (thrown) {
      setFailure(errorText(t, toError(thrown)));
    }
    focusHeading.current = true;
    setTarget(null);
  }

  const newReport = canRequest ? (
    <Link to="/generate" className={linkButton}>
      {t("nav.generate")}
    </Link>
  ) : null;

  let content;
  if (reports.data === undefined) {
    content = reports.isError ? (
      <div className="flex flex-col items-start gap-3">
        <Alert tone="error">{t("reports.loadFailed")}</Alert>
        <Button
          onClick={() => {
            void reports.refetch();
          }}
        >
          {t("reports.retry")}
        </Button>
      </div>
    ) : null;
  } else if (rows.length === 0) {
    content = (
      <div className="flex flex-col items-center gap-3 rounded-md border border-dashed border-input-border p-7 text-center text-muted-fg">
        <p>{t("reports.empty")}</p>
        {newReport}
      </div>
    );
  } else {
    content = (
      <>
        {chosen.length === 0 ? null : (
          <div className="flex flex-wrap items-center gap-3 rounded-md border border-border bg-surface px-3 py-2 text-sm text-surface-fg">
            <span className="font-mono">{t("reports.selected", { count: chosen.length })}</span>
            <Button variant="danger" onClick={askBulk}>
              {t("reports.deleteSelected")}
            </Button>
            <Button
              onClick={() => {
                setSelected(new Set());
              }}
            >
              {t("reports.clearSelection")}
            </Button>
          </div>
        )}
        <div className="overflow-x-auto rounded-md border border-border bg-surface text-surface-fg">
          <table className="w-full border-collapse text-sm">
            <caption className="sr-only">{t("reports.title")}</caption>
            <thead>
              <tr className="text-left text-xs text-muted-fg">
                {canRequest ? (
                  <th scope="col" className="px-3 py-2">
                    {deletable.length === 0 ? null : (
                      <Checkbox
                        hideLabel
                        label={t("reports.selectAll")}
                        checked={chosen.length === deletable.length}
                        indeterminate={chosen.length > 0 && chosen.length < deletable.length}
                        onChange={(event) => {
                          setSelected(new Set(event.target.checked ? deletable.map((row) => row.id) : []));
                        }}
                      />
                    )}
                  </th>
                ) : null}
                <th scope="col" className="px-3 py-2 font-semibold">
                  {t("reports.column.requested")}
                </th>
                <th scope="col" className="px-3 py-2 font-semibold">
                  {t("reports.column.profile")}
                </th>
                <th scope="col" className="px-3 py-2 font-semibold">
                  {t("reports.column.status")}
                </th>
                <th scope="col" className="px-3 py-2 font-semibold">
                  {t("reports.column.pdf")}
                </th>
                <th scope="col" className="px-3 py-2 font-semibold">
                  {t("reports.column.keptUntil")}
                </th>
                <th scope="col" className="px-3 py-2" />
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => {
                const mine = canDelete(principal, row);
                const time = formatTime(row.created_at);
                const pdf = row.pdfs.find((state) => state.language === i18n.language);
                return (
                  <tr key={row.id} className="border-t border-border">
                    {canRequest ? (
                      <td className="px-3 py-2">
                        {mine ? (
                          <Checkbox
                            hideLabel
                            label={t("reports.select", { time })}
                            checked={selected.has(row.id)}
                            onChange={(event) => {
                              toggle(row.id, event.target.checked);
                            }}
                          />
                        ) : null}
                      </td>
                    ) : null}
                    <td className="px-3 py-2 font-mono">{time}</td>
                    <td className="px-3 py-2 font-mono">{profileName(row)}</td>
                    <td className="px-3 py-2">
                      <StatusMark
                        status={reportMark(row)}
                        label={row.expired ? t("reports.expired") : t(`reportStatus.${row.status}`)}
                      />
                    </td>
                    <td className="px-3 py-2">
                      {pdf !== undefined && !row.expired ? (
                        <StatusMark status={pdfMark(pdf.status)} label={t(`pdfStatus.${pdf.status}`)} />
                      ) : row.status === "collected" && !row.expired ? (
                        t("reports.notGenerated")
                      ) : (
                        "—"
                      )}
                    </td>
                    <td className="px-3 py-2 font-mono">{formatDate(row.expires_at)}</td>
                    <td className="px-3 py-2">
                      <div className="flex items-center justify-end gap-2">
                        {row.expired ? null : (
                          <Link to={`/jobs/${row.id}`} className="text-accent underline">
                            {t("reports.open")}
                          </Link>
                        )}
                        {mine ? (
                          <Button
                            onClick={() => {
                              open({
                                kind: "one",
                                report: row,
                                profile: profileName(row),
                              });
                            }}
                          >
                            {t("reports.delete")}
                          </Button>
                        ) : null}
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
        {reports.hasNextPage ? (
          <div>
            <Button
              disabled={reports.isFetchingNextPage}
              onClick={() => {
                void reports.fetchNextPage();
              }}
            >
              {t("reports.loadMore")}
            </Button>
          </div>
        ) : null}
      </>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-col gap-1">
          <h1 ref={heading} tabIndex={-1} className="text-xl font-semibold outline-none">
            {t("reports.title")}
          </h1>
          {updating ? (
            <p className="flex items-center gap-2 text-sm text-muted-fg">
              <span aria-hidden="true" className="size-2 rounded-full bg-accent motion-safe:animate-pulse" />
              {t("reports.updating")}
            </p>
          ) : null}
        </div>
        {rows.length === 0 ? null : newReport}
      </div>
      {reports.isError && reports.data !== undefined ? <Alert tone="error">{t("reports.loadFailed")}</Alert> : null}
      {failure === null ? null : <Alert tone="error">{failure}</Alert>}
      {forbidden ? <Alert tone="error">{t("reports.deleteForbidden")}</Alert> : null}
      {content}
      <DeleteDialog
        target={target}
        busy={busy}
        onCloseAutoFocus={(event) => {
          event.preventDefault();
          const back = opener.current;
          if (!focusHeading.current && back instanceof HTMLElement && back.isConnected) {
            back.focus();
          } else {
            heading.current?.focus();
          }
          focusHeading.current = false;
          opener.current = null;
        }}
        onConfirm={() => {
          void confirm();
        }}
        onCancel={() => {
          focusHeading.current = false;
          setTarget(null);
        }}
      />
    </div>
  );
}
