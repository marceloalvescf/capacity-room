import { useEffect, useRef, useState, type SyntheticEvent } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useSearchParams } from "react-router";
import { ApiFailure } from "@/api/client";
import { errorText, toError } from "@/api/errors";
import { languageValues } from "@/api/schema.gen";
import { useScreenTitle } from "@/app/title";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Segmented } from "@/components/ui/segmented";
import { Select } from "@/components/ui/select";
import { isLanguage, type Language } from "@/i18n/language";
import { useProfiles, useReport, useRequestReport, type ProfileSummary, type Report } from "./api";
import { periodDays, periodText, resolveCalendar, type PeriodValue } from "./period";
import { PeriodPicker } from "./period-picker";

interface Values {
  profileId: string;
  namespaces: string[];
  language: Language;
  period: PeriodValue;
  timezone: string;
  reportProfile: string;
  minSeverity: string;
}

type Marked = "namespaces" | "period" | "timezone" | "report_profile" | "min_severity";
type Problem = "notPermitted" | "invalid" | "required" | "wallClock" | "order" | "future";

const markable: readonly string[] = [
  "namespaces",
  "period",
  "timezone",
  "report_profile",
  "min_severity",
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
    namespaces: [],
    language,
    period: { kind: "length", period: profile.default_period },
    timezone: profile.default_timezone,
    reportProfile: profile.default_report_profile,
    minSeverity: profile.default_min_severity,
  };
}

function fromReport(source: Report, profiles: ProfileSummary[], fallback: Language): Values {
  const profile = profiles.find((candidate) => candidate.id === source.profile_id);
  if (profile === undefined) {
    return defaultsOf(profiles[0] as ProfileSummary, fallback);
  }
  const { selection } = source;
  // A profile that now limits its periods allows no range.
  const period: PeriodValue =
    selection.period !== null
      ? { kind: "length", period: selection.period }
      : profile.permitted_periods.length > 0
        ? defaultsOf(profile, fallback).period
        : { kind: "range", start: selection.start ?? "", end: selection.end ?? "" };
  return {
    profileId: profile.id,
    namespaces: selection.namespaces.filter((name) => profile.allowed_namespaces.includes(name)),
    language: selection.language,
    period,
    timezone: selection.timezone,
    reportProfile: selection.report_profile,
    minSeverity: selection.min_severity,
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
  const { t, i18n } = useTranslation();
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
  const form = useRef<HTMLFormElement>(null);
  const periodTrigger = useRef<HTMLButtonElement>(null);
  const focusPeriodOnOpen = useRef(false);
  // Counts the attempts that left a field marked; each one moves focus to the
  // first marked field once it is drawn. The period's trigger is a button,
  // which has no aria-invalid, so it says so with data-invalid.
  const [marks, setMarks] = useState(0);

  useEffect(() => {
    if (marks > 0) {
      form.current?.querySelector<HTMLElement>('[aria-invalid="true"], [data-invalid="true"]')?.focus();
    }
  }, [marks]);

  // A profile that vanished resets the form to the first one, which is a new
  // request: its key and its kept body go too.
  const vanished = !profiles.some((candidate) => candidate.id === values.profileId);
  useEffect(() => {
    if (vanished) {
      key.current = crypto.randomUUID();
      answered.current = false;
      kept.current = null;
    }
  }, [vanished]);

  useEffect(() => {
    if (optionsOpen && focusPeriodOnOpen.current) {
      focusPeriodOnOpen.current = false;
      periodTrigger.current?.focus();
    }
  }, [optionsOpen]);

  const profile = profiles.find((candidate) => candidate.id === values.profileId);
  if (profile === undefined) {
    setValues(defaultsOf(profiles[0] as ProfileSummary, values.language));
    return null;
  }
  const language = isLanguage(i18n.language) ? i18n.language : "en";

  function renewKey() {
    key.current = crypto.randomUUID();
    answered.current = false;
    kept.current = null;
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

  function toggle(name: string, on: boolean) {
    const kept = values.namespaces.filter((other) => other !== name);
    edit({ namespaces: on ? [...kept, name] : kept }, ["namespaces"]);
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
      } else if (markable.includes(name)) {
        marked[name as Marked] = reason === "not_permitted" ? "notPermitted" : "invalid";
      } else {
        rest = true;
      }
    }
    setProblems(marked);
    if (Object.keys(marked).some((name) => name !== "namespaces")) {
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
    const namespaces = profile.allowed_namespaces.filter((name) => values.namespaces.includes(name));
    if (namespaces.length === 0) {
      setProblems({ namespaces: "required" });
      setMarks((count) => count + 1);
      return;
    }
    const when = kept.current ?? bodyPeriod(values.period, values.timezone);
    if (when === null) {
      setProblems({ period: "order" });
      setOptionsOpen(true);
      setMarks((count) => count + 1);
      return;
    }
    kept.current = when;
    sending.current = true;
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
          min_severity: values.minSeverity,
        },
      });
      // The form stays locked: the page is left, and a second click in the
      // meantime must not send again.
      void navigate(`/jobs/${accepted.id}`);
    } catch (thrown) {
      sending.current = false;
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
  const summary = [
    periodText(t, language, values.period),
    values.timezone,
    t(`reportProfile.${values.reportProfile}`),
    t(`minSeverity.${values.minSeverity}`),
  ].join(" · ");
  const namespaceProblem = problemText("namespaces");

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

      <fieldset
        tabIndex={-1}
        className="flex flex-col gap-1 font-mono focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
        aria-invalid={namespaceProblem === undefined ? undefined : true}
        aria-describedby={namespaceProblem === undefined ? undefined : "namespaces-problem"}
      >
        <legend className="mb-1 font-sans text-sm font-medium">{t("generate.namespaces")}</legend>
        <div className="flex flex-wrap gap-x-6">
          {profile.allowed_namespaces.map((name) => (
            <Checkbox
              key={name}
              label={name}
              checked={values.namespaces.includes(name)}
              onChange={(event) => {
                toggle(name, event.target.checked);
              }}
            />
          ))}
        </div>
        {namespaceProblem === undefined ? null : (
          <p id="namespaces-problem" className="font-sans text-sm text-critical">
            {namespaceProblem}
          </p>
        )}
      </fieldset>

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

      <div className="flex flex-col gap-4 rounded-md border border-border bg-surface p-3 text-surface-fg">
        <div className="flex flex-wrap items-center gap-3">
          <b className="text-sm">{t("generate.options")}</b>
          {optionsOpen ? null : (
            <>
              <span className="font-mono text-sm text-muted-fg">{summary}</span>
              <Button
                className="ml-auto"
                onClick={() => {
                  focusPeriodOnOpen.current = true;
                  setOptionsOpen(true);
                }}
              >
                {t("generate.change")}
              </Button>
            </>
          )}
        </div>
        {optionsOpen ? (
          <div className="grid gap-4 sm:grid-cols-2">
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
            <Select
              label={t("generate.timezone")}
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
            <Select
              label={t("generate.minSeverity")}
              value={values.minSeverity}
              error={problemText("min_severity")}
              onChange={(event) => {
                edit({ minSeverity: event.target.value }, ["min_severity"]);
              }}
            >
              {withCurrent(profile.permitted_min_severities, values.minSeverity).map((level) => (
                <option key={level} value={level}>
                  {t(`minSeverity.${level}`)}
                </option>
              ))}
            </Select>
          </div>
        ) : null}
      </div>

      {days !== null && days > 15 ? <Alert tone="notice">{t("generate.longPeriod")}</Alert> : null}

      <div>
        <Button type="submit" variant="primary" disabled={request.isPending}>
          {request.isPending ? t("generate.submitting") : t("generate.submit")}
        </Button>
      </div>
    </form>
  );
}

function FromReport({ id, profiles, reload }: { id: string; profiles: ProfileSummary[]; reload: () => void }) {
  const { t, i18n } = useTranslation();
  const source = useReport(id);
  const fallback = isLanguage(i18n.language) ? i18n.language : "en";
  if (source.data === undefined && !source.isError) {
    return null;
  }
  const initial =
    source.data === undefined
      ? defaultsOf(profiles[0] as ProfileSummary, fallback)
      : fromReport(source.data, profiles, fallback);
  return (
    <div className="flex max-w-3xl flex-col gap-6">
      {source.error === null ? null : <Alert tone="error">{errorText(t, toError(source.error))}</Alert>}
      <GenerateForm profiles={profiles} initial={initial} reload={reload} />
    </div>
  );
}

export function GeneratePage() {
  const { t, i18n } = useTranslation();
  const [params] = useSearchParams();
  const profiles = useProfiles();
  useScreenTitle(t("generate.title"));
  const from = params.get("from");

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
          initial={defaultsOf(list[0] as ProfileSummary, isLanguage(i18n.language) ? i18n.language : "en")}
          reload={reload}
        />
      );
    } else {
      content = <FromReport id={from} profiles={list} reload={reload} />;
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-xl font-semibold">{t("generate.title")}</h1>
      {content}
    </div>
  );
}
