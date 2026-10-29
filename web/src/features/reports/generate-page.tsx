import { useCallback, useEffect, useRef, useState, type ReactNode, type SyntheticEvent } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { useNavigate, useSearchParams } from "react-router";
import { ApiFailure } from "@/api/client";
import { errorText, toError } from "@/api/errors";
import { languageValues } from "@/api/schema.gen";
import { useScreenTitle } from "@/app/title";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { NamespacePicker } from "@/components/namespace-picker";
import { SummaryLine, valuePart, wordPart, type Summary } from "@/components/summary-line";
import { Segmented } from "@/components/ui/segmented";
import { Select } from "@/components/ui/select";
import { isLanguage, type Language } from "@/i18n/language";
import { useProfileNamespaces, useProfiles, useReport, useRequestReport, type ProfileSummary, type Report } from "./api";
import { periodDays, periodText, resolveCalendar, type PeriodValue } from "./period";
import { PeriodPicker } from "./period-picker";

interface Values {
  profileId: string;
  namespaceScope: "all" | "choose";
  namespaces: string[];
  language: Language;
  period: PeriodValue;
  timezone: string;
  reportProfile: string;
}

type Marked = "namespaces" | "period" | "timezone" | "report_profile";
type Problem = "notPermitted" | "invalid" | "required" | "wallClock" | "order" | "future";

const markable: readonly string[] = [
  "namespaces",
  "period",
  "timezone",
  "report_profile",
] satisfies Marked[];

// How the form words a reason the server gives for the start or the end.
const rangeWords: Record<string, Problem | undefined> = {
  invalid: "wallClock",
  order: "order",
  future: "future",
  not_permitted: "notPermitted",
};

function defaultsOf(profile: ProfileSummary, language: Language): Values {
  return {
    profileId: profile.id,
    namespaceScope: "all",
    namespaces: [],
    language,
    period: { kind: "length", period: profile.default_period },
    timezone: profile.default_timezone,
    reportProfile: profile.default_report_profile,
  };
}

// What a copy of a report left behind, to be told to the reader.
type FromNotice =
  | { kind: "profileGone"; profile: string }
  | { kind: "namespacesDropped"; names: string[] }
  | { kind: "periodDropped"; period: PeriodValue };

function fromReport(
  source: Report,
  profiles: ProfileSummary[],
  fallback: Language,
): { values: Values; notices: FromNotice[] } {
  const profile = profiles.find((candidate) => candidate.id === source.profile_id);
  if (profile === undefined) {
    const first = profiles[0] as ProfileSummary;
    return {
      values: defaultsOf(first, fallback),
      notices: [{ kind: "profileGone", profile: first.display_name }],
    };
  }
  const { selection } = source;
  const notices: FromNotice[] = [];
  let period: PeriodValue;
  if (selection.period !== null) {
    period = { kind: "length", period: selection.period };
  } else if (profile.permitted_periods.length > 0) {
    // A profile that now limits its periods allows no range.
    period = defaultsOf(profile, fallback).period;
    notices.push({ kind: "periodDropped", period });
  } else {
    period = { kind: "range", start: selection.start ?? "", end: selection.end ?? "" };
  }
  const selected = selection.namespaces ?? [];
  // A profile with no limit allows every name; one that limits its namespaces no longer allows the others.
  const limit = profile.allowed_namespaces;
  const namespaces = limit.length === 0 ? selected : selected.filter((name) => limit.includes(name));
  const dropped = selected.filter((name) => !namespaces.includes(name));
  if (dropped.length > 0) {
    notices.unshift({ kind: "namespacesDropped", names: dropped });
  }
  return {
    values: {
      profileId: profile.id,
      namespaceScope: selection.namespaces === null ? "all" : "choose",
      namespaces,
      language: selection.language,
      period,
      timezone: selection.timezone,
      reportProfile: selection.report_profile,
    },
    notices,
  };
}

// The list a select shows always holds the value it carries, so what is shown
// is what is sent.
function withCurrent(list: readonly string[], current: string): string[] {
  return list.includes(current) ? [...list] : [current, ...list];
}

function zonesOf(profile: ProfileSummary): readonly string[] {
  if (profile.permitted_timezones.length > 0) {
    return profile.permitted_timezones;
  }
  const all = Intl.supportedValuesOf("timeZone");
  return all.includes(profile.default_timezone) ? all : [profile.default_timezone, ...all];
}

// The period fields of the request body, or null for a range with no time in it.
function bodyPeriod(
  value: PeriodValue,
  zone: string,
): { period: string; start: null; end: null } | { period: null; start: string; end: string } | null {
  if (value.kind === "length") {
    return { period: value.period, start: null, end: null };
  }
  const { start, end } = value.kind === "calendar" ? resolveCalendar(value.name, zone, new Date()) : value;
  // Wall strings of the grammar compare as text.
  return start < end ? { period: null, start, end } : null;
}

function GenerateForm({
  profiles,
  initial,
  reload,
}: {
  profiles: ProfileSummary[];
  initial: Values;
  reload: () => void;
}) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const request = useRequestReport();
  const [values, setValues] = useState(initial);
  const [problems, setProblems] = useState<Partial<Record<Marked, Problem>>>({});
  const [failure, setFailure] = useState<string | null>(null);
  const [optionsOpen, setOptionsOpen] = useState(false);
  // One key per attempt: it is renewed once an attempt was answered and a value
  // changes, so that a resend of an attempt that got no answer is replayed.
  const key = useRef(crypto.randomUUID());
  const answered = useRef(false);
  // The period fields of the last attempt, sent again as they were for as long
  // as its key is kept, so a resend carries the body its key was made for.
  const kept = useRef<ReturnType<typeof bodyPeriod>>(null);
  const sending = useRef(false);
  const [locked, setLocked] = useState(false);
  const form = useRef<HTMLFormElement>(null);
  const periodTrigger = useRef<HTMLButtonElement>(null);
  const timezoneSelect = useRef<HTMLSelectElement>(null);
  const focusTimezoneOnOpen = useRef(false);
  // Counts the attempts that left a field marked; each one moves focus to the
  // first marked field once it is drawn. The period's trigger is a button,
  // which has no aria-invalid, so it says so with data-invalid.
  const [marks, setMarks] = useState(0);

  useEffect(() => {
    if (marks > 0) {
      form.current?.querySelector<HTMLElement>('[aria-invalid="true"], [data-invalid="true"]')?.focus();
    }
  }, [marks]);

  const renewKey = useCallback(() => {
    key.current = crypto.randomUUID();
    answered.current = false;
    kept.current = null;
  }, []);

  const profile = profiles.find((candidate) => candidate.id === values.profileId);
  const listed = useProfileNamespaces(values.profileId);

  useEffect(() => {
    if (optionsOpen && focusTimezoneOnOpen.current) {
      focusTimezoneOnOpen.current = false;
      timezoneSelect.current?.focus();
    }
  }, [optionsOpen]);

  // A profile that left the list gives way to the first one. The reset clears
  // the namespaces, so the next send follows an edit, which renews the key once
  // an attempt was answered.
  if (profile === undefined) {
    setValues(defaultsOf(profiles[0] as ProfileSummary, values.language));
    return null;
  }

  function edit(next: Partial<Values>, cleared: Marked[] = []) {
    if (answered.current) {
      renewKey();
    }
    kept.current = null;
    setValues((current) => ({ ...current, ...next }));
    setProblems((current) => {
      return Object.fromEntries(Object.entries(current).filter(([name]) => !cleared.some((gone) => gone === name)));
    });
  }

  function chooseProfile(id: string) {
    const next = profiles.find((candidate) => candidate.id === id);
    if (next === undefined) {
      return;
    }
    if (answered.current) {
      renewKey();
    }
    kept.current = null;
    setValues(defaultsOf(next, values.language));
    setProblems({});
  }

  function mark(error: ApiFailure): boolean {
    if (error.code !== "REQUEST_INVALID" && error.code !== "REPORT_SELECTION_NOT_PERMITTED") {
      return false;
    }
    const marked: Partial<Record<Marked, Problem>> = {};
    let rest = false;
    for (const [name, reason] of Object.entries(error.fields)) {
      if (name === "start" || name === "end") {
        const word = rangeWords[reason];
        if (word === undefined) {
          rest = true;
        } else {
          marked.period ??= word;
        }
      } else if (markable.includes(name) && (name !== "namespaces" || values.namespaceScope === "choose")) {
        marked[name as Marked] = reason === "not_permitted" ? "notPermitted" : "invalid";
      } else {
        rest = true;
      }
    }
    setProblems(marked);
    if (error.code === "REPORT_SELECTION_NOT_PERMITTED") {
      reload();
    }
    if (Object.keys(marked).some((name) => name !== "namespaces" && name !== "period")) {
      setOptionsOpen(true);
    }
    if (Object.keys(marked).length > 0) {
      setMarks((count) => count + 1);
    }
    return Object.keys(marked).length > 0 && !rest;
  }

  async function submit(event: SyntheticEvent) {
    event.preventDefault();
    if (sending.current || profile === undefined) {
      return;
    }
    const namespaces = values.namespaceScope === "all" ? null : [...values.namespaces].sort();
    if (namespaces !== null && namespaces.length === 0) {
      setProblems({ namespaces: "required" });
      setMarks((count) => count + 1);
      return;
    }
    const when = kept.current ?? bodyPeriod(values.period, values.timezone);
    if (when === null) {
      setProblems({ period: "order" });
      setMarks((count) => count + 1);
      return;
    }
    kept.current = when;
    sending.current = true;
    setLocked(true);
    setFailure(null);
    setProblems({});
    try {
      const accepted = await request.mutateAsync({
        key: key.current,
        body: {
          profile_id: values.profileId,
          language: values.language,
          namespaces,
          ...when,
          timezone: values.timezone,
          report_profile: values.reportProfile,
        },
      });
      // The lock stays on after an accepted request: the page is being left,
      // and a second click in the meantime must not send again.
      void navigate(`/jobs/${accepted.id}`);
    } catch (thrown) {
      sending.current = false;
      setLocked(false);
      const error = toError(thrown);
      if (!(error instanceof ApiFailure)) {
        setFailure(errorText(t, error));
      } else {
        answered.current = true;
        if (error.code === "PROFILE_INACTIVE") {
          setFailure(t("generate.profileChanged"));
          reload();
        } else if (!mark(error)) {
          // A key whose first request made a report that is gone cannot be replayed.
          if (error.code === "IDEMPOTENCY_KEY_REUSED" || error.status === 404 || error.status === 410) {
            renewKey();
          }
          setFailure(errorText(t, error));
        }
      }
    }
  }

  const problemText = (name: Marked): string | undefined => {
    const problem = problems[name];
    if (problem === undefined) {
      return undefined;
    }
    switch (problem) {
      case "required":
        return t("validation.namespaces");
      case "notPermitted":
        return t("validation.notPermitted");
      case "wallClock":
        return t("period.range.wallClock", { zone: values.timezone });
      case "order":
        return t("period.range.order");
      case "future":
        return t("period.range.future");
      case "invalid":
        return t("validation.invalid");
    }
  };

  const days = periodDays(
    values.period.kind === "calendar"
      ? { kind: "range", ...resolveCalendar(values.period.name, values.timezone, new Date()) }
      : values.period,
  );
  const summary: Summary = [
    valuePart(values.timezone),
    wordPart(t(`reportProfile.${values.reportProfile}`)),
  ];
  const answer = listed.data;
  const listOk = answer?.code === "PROFILE_OK";
  const listLoading = listed.isFetching && !listOk;
  const listFailure =
    answer !== undefined && !listOk
      ? t(`profileTest.${answer.code}`)
      : listed.isError
        ? errorText(t, toError(listed.error))
        : "";
  const unknownText = listLoading ? t("namespaces.loading") : listFailure;
  let listLine: ReactNode = null;
  if (listLoading) {
    listLine = <p className="text-sm text-muted-fg">{t("namespaces.loading")}</p>;
  } else if (listFailure !== "") {
    listLine = (
      <Alert tone="error">
        <div className="flex flex-wrap items-center gap-3">
          <p>{listFailure}</p>
          <Button
            onClick={() => {
              void listed.refetch();
            }}
          >
            {t("reports.retry")}
          </Button>
        </div>
      </Alert>
    );
  } else if (listOk && answer.namespaces.length === 0) {
    listLine = <p className="text-sm text-muted-fg">{t("namespaces.none")}</p>;
  } else if (listOk && answer.limited) {
    listLine = <p className="text-sm text-muted-fg">{t("generate.limited", { names: answer.namespaces.join(", ") })}</p>;
  }

  return (
    <form
      ref={form}
      noValidate
      className="flex max-w-3xl flex-col gap-6"
      onSubmit={(event) => {
        void submit(event);
      }}
    >
      {failure === null ? null : <Alert tone="error">{failure}</Alert>}
      <Select
        label={t("generate.profile")}
        value={values.profileId}
        onChange={(event) => {
          chooseProfile(event.target.value);
        }}
      >
        {profiles.map((candidate) => (
          <option key={candidate.id} value={candidate.id}>
            {candidate.display_name}
          </option>
        ))}
      </Select>

      <div className="flex flex-col gap-2">
        <Segmented
          legend={t("generate.namespaces")}
          name="report-namespaces"
          value={values.namespaceScope}
          options={[
            { value: "all", label: listOk ? t("namespaces.allCount", { count: answer.namespaces.length }) : t("namespaces.all") },
            { value: "choose", label: t("namespaces.choose") },
          ]}
          onChange={(next) => {
            edit({ namespaceScope: next }, ["namespaces"]);
          }}
        />
        {listLine}
        {values.namespaceScope === "choose" ? (
          <NamespacePicker
            label={t("generate.namespaces")}
            hideLabel
            names={listOk ? answer.namespaces : null}
            unknownText={unknownText}
            selected={values.namespaces}
            error={problemText("namespaces")}
            onChange={(next) => {
              edit({ namespaces: next }, ["namespaces"]);
            }}
          />
        ) : null}
      </div>

      <div className="max-w-96">
        <PeriodPicker
          value={values.period}
          zone={values.timezone}
          permitted={profile.permitted_periods}
          error={problemText("period")}
          triggerRef={periodTrigger}
          onChange={(period) => {
            edit({ period }, ["period"]);
          }}
        />
      </div>

      <Segmented
        legend={t("generate.language")}
        name="report-language"
        value={values.language}
        options={languageValues.map((option) => ({
          value: option,
          label: t(`language.${option}`, { lng: option }),
          lang: option,
        }))}
        onChange={(next) => {
          edit({ language: next });
        }}
      />

      <div className="flex flex-col gap-4 rounded-lg border border-border bg-surface p-3 text-surface-fg">
        <div className="flex flex-wrap items-center gap-3">
          <div className="min-w-0">
            <p className="font-semibold">{t("generate.options")}</p>
            {optionsOpen ? null : (
              <p className="text-sm break-words text-muted-fg">
                <SummaryLine summary={summary} />
              </p>
            )}
          </div>
          {optionsOpen ? null : (
            <Button
              className="ml-auto"
              onClick={() => {
                focusTimezoneOnOpen.current = true;
                setOptionsOpen(true);
              }}
            >
              {t("generate.change")}
            </Button>
          )}
        </div>
        {optionsOpen ? (
          <div className="grid gap-4 sm:grid-cols-2">
            <Select
              label={t("generate.timezone")}
              ref={timezoneSelect}
              className="font-mono"
              value={values.timezone}
              error={problemText("timezone")}
              onChange={(event) => {
                edit({ timezone: event.target.value }, ["timezone"]);
              }}
            >
              {withCurrent(zonesOf(profile), values.timezone).map((zone) => (
                <option key={zone} value={zone}>
                  {zone}
                </option>
              ))}
            </Select>
            <Select
              label={t("generate.reportProfile")}
              value={values.reportProfile}
              error={problemText("report_profile")}
              onChange={(event) => {
                edit({ reportProfile: event.target.value }, ["report_profile"]);
              }}
            >
              {withCurrent(profile.permitted_report_profiles, values.reportProfile).map((kind) => (
                <option key={kind} value={kind}>
                  {t(`reportProfile.${kind}`)}
                </option>
              ))}
            </Select>
          </div>
        ) : null}
      </div>

      {days !== null && days > 15 ? <Alert tone="notice">{t("generate.longPeriod")}</Alert> : null}

      <div>
        <Button type="submit" variant="primary" disabled={locked}>
          {locked ? t("generate.submitting") : t("generate.submit")}
        </Button>
      </div>
    </form>
  );
}

const noticeKeys: Record<FromNotice["kind"], string> = {
  profileGone: "generate.from.profileGone",
  namespacesDropped: "generate.from.namespacesDropped",
  periodDropped: "generate.from.periodDropped",
};

function noticeValues(t: TFunction, language: Language, notice: FromNotice): Record<string, string> {
  switch (notice.kind) {
    case "profileGone":
      return { profile: notice.profile };
    case "namespacesDropped":
      return { names: notice.names.join(", ") };
    case "periodDropped":
      return { period: periodText(t, language, notice.period) };
  }
}

function FromReport({ id, profiles, reload }: { id: string; profiles: ProfileSummary[]; reload: () => void }) {
  const { t } = useTranslation();
  const source = useReport(id);
  if (source.data === undefined && !source.isError) {
    return null;
  }
  return (
    <FromForm
      key={id}
      source={source.data}
      error={source.error === null ? null : errorText(t, toError(source.error))}
      profiles={profiles}
      reload={reload}
    />
  );
}

// The form's first values and the notices that go with them are worked out
// once, from the profiles as they were when the copy opened.
function FromForm({
  source,
  error,
  profiles,
  reload,
}: {
  source: Report | undefined;
  error: string | null;
  profiles: ProfileSummary[];
  reload: () => void;
}) {
  const { t, i18n } = useTranslation();
  const fallback = isLanguage(i18n.language) ? i18n.language : "en";
  const [{ values, notices }] = useState(() =>
    source === undefined
      ? { values: defaultsOf(profiles[0] as ProfileSummary, fallback), notices: [] }
      : fromReport(source, profiles, fallback),
  );
  return (
    <div className="flex max-w-3xl flex-col gap-6">
      {error === null ? null : <Alert tone="error">{error}</Alert>}
      {notices.map((notice) => (
        <Alert key={notice.kind} tone="notice">
          {t(noticeKeys[notice.kind], noticeValues(t, fallback, notice))}
        </Alert>
      ))}
      <GenerateForm profiles={profiles} initial={values} reload={reload} />
    </div>
  );
}

export function GeneratePage() {
  const { t, i18n } = useTranslation();
  const [params] = useSearchParams();
  const profiles = useProfiles();
  useScreenTitle(t("generate.title"));
  const from = params.get("from");
  const named = params.get("profile");

  let content = null;
  if (profiles.isError) {
    content = <Alert tone="error">{t("generate.profilesFailed")}</Alert>;
  } else if (profiles.data !== undefined) {
    const list = profiles.data;
    const reload = () => {
      void profiles.refetch();
    };
    if (list.length === 0) {
      content = (
        <div className="flex max-w-3xl flex-col items-center gap-3 rounded-md border border-dashed border-input-border p-7 text-center text-muted-fg">
          <p>{t("generate.noProfiles")}</p>
        </div>
      );
    } else if (from === null) {
      content = (
        <GenerateForm
          profiles={list}
          initial={defaultsOf(
            list.find((candidate) => candidate.id === named) ?? (list[0] as ProfileSummary),
            isLanguage(i18n.language) ? i18n.language : "en",
          )}
          reload={reload}
        />
      );
    } else {
      content = <FromReport id={from} profiles={list} reload={reload} />;
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-3xl font-semibold tracking-tight">{t("generate.title")}</h1>
      {content}
    </div>
  );
}
