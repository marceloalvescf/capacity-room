import { useRef, useState } from "react";
import { Trans, useTranslation } from "react-i18next";
import { Link } from "react-router";
import { ApiFailure } from "@/api/client";
import { errorText, toError } from "@/api/errors";
import { useAnnounce } from "@/app/live-region";
import { useScreenTitle } from "@/app/title";
import { Separator } from "@/components/separator";
import { StatusMark } from "@/components/status-mark";
import { Alert } from "@/components/ui/alert";
import { Button, buttonStyles } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { cn } from "@/lib/cn";
import { formatDate, formatTime, formatTimeSeconds } from "@/lib/format";
import { usePrincipal } from "@/session/session";
import {
  canDelete,
  isActive,
  useDeleteReport,
  useDeleteReports,
  useProfiles,
  useReports,
  type ReportSelection,
  type ReportSummary,
} from "./api";
import { DeleteDialog, type DeleteTarget } from "./delete-dialog";
import { pdfMark, reportMark } from "./marks";
import { periodText, type PeriodValue } from "./period";
import { isLanguage, type Language } from "@/i18n/language";

const linkButton = buttonStyles("primary");

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
  const lang: Language = isLanguage(i18n.language) ? i18n.language : "en";
  const scope = (selection: ReportSelection): string => {
    const { period, start, end } = selection;
    const value: PeriodValue = period === null ? { kind: "range", start: start ?? "", end: end ?? "" } : { kind: "length", period };
    return periodText(t, lang, value);
  };
  const deletable = rows.filter((row) => canDelete(principal, row));
  const chosen = deletable.filter((row) => selected.has(row.id));
  const selectable = deletable.length > 0;
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
        ? { kind: "one", id: only.id, profile: profileName(only), time: formatTime(only.created_at), username: only.owner_username }
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
          await deleteOne.mutateAsync(current.id);
        } catch (thrown) {
          // A report already gone counts as gone.
          if (!(thrown instanceof ApiFailure && thrown.status === 404)) {
            throw thrown;
          }
        }
        toggle(current.id, false);
        went = 1;
      } else {
        const { results, failure: cut } = await deleteMany.mutateAsync(current.ids);
        const refused = results.filter((result) => result.outcome === "forbidden").map((result) => result.id);
        const answered = new Set(results.map((result) => result.id));
        // What was refused, and what no answer covers, stays selected.
        setSelected(new Set([...refused, ...current.ids.filter((one) => !answered.has(one))]));
        setForbidden(refused.length > 0);
        went = results.filter((result) => result.outcome === "deleted" || result.outcome === "not_found").length;
        if (cut !== null) {
          setFailure(errorText(t, toError(cut)));
          if (went === 0) {
            focusHeading.current = true;
            setTarget(null);
            return;
          }
        }
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

  const retry = (
    <Button
      onClick={() => {
        void reports.refetch();
        void profiles.refetch();
      }}
    >
      {t("reports.retry")}
    </Button>
  );
  const namesFailed = profiles.data === undefined && profiles.isError;
  const loading = <p role="status" className="text-sm text-muted-fg">{t("reports.loading")}</p>;

  let content;
  if (reports.data === undefined) {
    content = reports.isError ? (
      <div className="flex flex-col items-start gap-3">
        <Alert tone="error">{t("reports.loadFailed")}</Alert>
        {retry}
      </div>
    ) : (
      loading
    );
  } else if (profiles.data === undefined && !namesFailed) {
    content = loading;
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
        {namesFailed ? (
          <div className="flex flex-col items-start gap-3">
            <Alert tone="notice">{t("reports.profilesFailed")}</Alert>
            {retry}
          </div>
        ) : null}
        {chosen.length === 0 ? null : (
          <div className="flex flex-wrap items-center gap-3 rounded-lg border border-border bg-surface px-3 py-2 text-sm text-surface-fg">
            <span>{t("reports.selected", { count: chosen.length })}</span>
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
        <div className="overflow-x-auto rounded-lg border border-border bg-surface text-surface-fg">
          <table className="w-full border-collapse text-sm">
            <caption className="sr-only">{t("reports.title")}</caption>
            <thead>
              <tr className="text-left">
                {selectable ? (
                  <th scope="col" className="px-3 py-2">
                    <Checkbox
                      hideLabel
                      label={t("reports.selectAll")}
                      checked={chosen.length === deletable.length}
                      indeterminate={chosen.length > 0 && chosen.length < deletable.length}
                      onChange={(event) => {
                        setSelected(new Set(event.target.checked ? deletable.map((row) => row.id) : []));
                      }}
                    />
                  </th>
                ) : null}
                <th scope="col" className="label px-3 py-2">
                  {t("reports.column.requested")}
                </th>
                <th scope="col" className="label px-3 py-2">
                  {t("reports.column.profile")}
                </th>
                <th scope="col" className="label px-3 py-2">
                  {t("reports.column.status")}
                </th>
                <th scope="col" className="label px-3 py-2">
                  {t("reports.column.pdf")}
                </th>
                <th scope="col" className="px-3 py-2">
                  <span className="sr-only">{t("reports.column.actions")}</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => {
                const mine = canDelete(principal, row);
                const time = formatTime(row.created_at);
                const exact = formatTimeSeconds(row.created_at);
                const label = { profile: profileName(row), time: exact };
                return (
                  <tr key={row.id} className="border-t border-border">
                    {selectable ? (
                      <td className="px-3 py-2">
                        {mine ? (
                          <Checkbox
                            hideLabel
                            label={t("reports.select", label)}
                            checked={selected.has(row.id)}
                            onChange={(event) => {
                              toggle(row.id, event.target.checked);
                            }}
                          />
                        ) : null}
                      </td>
                    ) : null}
                    <td className="px-3 py-2">
                      <div className="font-mono whitespace-nowrap">{time}</div>
                      <div className="text-xs text-muted-fg">
                        <Trans
                          i18nKey="progress.keptUntil"
                          values={{ date: formatDate(row.expires_at) }}
                          components={{ date: <span className="font-mono whitespace-nowrap" /> }}
                        />
                      </div>
                    </td>
                    <td className="px-3 py-2">
                      <div className={cn(!names.has(row.profile_id) && "font-mono")}>{profileName(row)}</div>
                      {row.cluster_name === null ? null : (
                        <div className="text-xs">
                          <span className="sr-only">{t("reports.column.cluster")}: </span>
                          <span className="font-mono">{row.cluster_name}</span>
                        </div>
                      )}
                      {row.selection !== null ? (
                        <div className="text-xs text-muted-fg">
                          <ReportScope selection={row.selection} lengthText={scope(row.selection)} language={t(`language.${row.selection.language}`)} />
                        </div>
                      ) : null}
                    </td>
                    <td className="px-3 py-2">
                      <div>
                        <StatusMark
                          status={reportMark(row)}
                          label={row.expired ? t("reports.expired") : t(`reportStatus.${row.status}`)}
                        />
                      </div>
                      {row.worst_severity === null ? null : (
                        <div>
                          <span className="sr-only">{t("reports.column.worst")}: </span>
                          <StatusMark status={row.worst_severity} />
                        </div>
                      )}
                    </td>
                    <td className="px-3 py-2">
                      {row.pdfs.length > 0 && !row.expired ? (
                        <ul className="grid grid-cols-[max-content_1fr] gap-x-2 gap-y-1">
                          {row.pdfs.map((pdf) => (
                            <li key={pdf.language} className="col-span-2 grid grid-cols-subgrid items-center">
                              <span className="font-mono whitespace-nowrap">{pdf.language}</span>
                              <StatusMark status={pdfMark(pdf.status)} label={t(`pdfStatus.${pdf.status}`)} />
                            </li>
                          ))}
                        </ul>
                      ) : row.status === "collected" && !row.expired ? (
                        t("reports.notGenerated")
                      ) : (
                        t("reports.noPdf")
                      )}
                    </td>
                    <td className="px-3 py-2">
                      <div className="flex items-center justify-end gap-2">
                        {row.expired ? null : (
                          <Link to={`/jobs/${row.id}`} aria-label={t("reports.openOne", label)} className="text-accent underline">
                            {t("reports.open")}
                          </Link>
                        )}
                        {mine ? (
                          <Button
                            aria-label={t("reports.deleteOne", label)}
                            onClick={() => {
                              open({ kind: "one", id: row.id, profile: profileName(row), time, username: row.owner_username });
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
          <h1 ref={heading} tabIndex={-1} className="text-3xl font-semibold tracking-tight outline-none">
            {t("reports.title")}
          </h1>
          {updating ? (
            <p className="flex items-center gap-2 text-sm text-muted-fg">
              <svg aria-hidden="true" viewBox="0 0 10 10" className="size-2.5 stroke-accent motion-safe:animate-pulse">
                <circle cx="5" cy="5" r="3.75" fill="none" strokeWidth="1.5" />
              </svg>
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

const shownNamespaces = 3;

// The namespaces a report covers, at most three, then the count of the rest, its period, and its language. The namespaces, a length,
// each wall time of a range, and the language stay whole; the line breaks between them.
function ReportScope({ selection, lengthText, language }: { selection: ReportSelection; lengthText: string; language: string }) {
  const { t } = useTranslation();
  const { namespaces } = selection;
  const rest = namespaces.length - shownNamespaces;
  return (
    <>
      {namespaces.length > 0 ? (
        <>
          <span className="font-mono">{namespaces.slice(0, shownNamespaces).join(", ")}</span>
          {rest > 0 ? <span className="font-mono"> {t("reports.moreNamespaces", { count: rest })}</span> : null}
          <Separator />
        </>
      ) : null}
      {selection.period === null ? (
        <span>
          <Trans
            i18nKey="period.range.scope"
            values={{ start: selection.start, end: selection.end }}
            components={{ date: <span className="font-mono whitespace-nowrap" /> }}
          />
        </span>
      ) : (
        <span className="whitespace-nowrap">{lengthText}</span>
      )}
      <Separator />
      <span className="whitespace-nowrap">{language}</span>
    </>
  );
}
