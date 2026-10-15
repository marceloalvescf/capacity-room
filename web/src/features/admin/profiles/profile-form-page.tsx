import { DatedText } from "@/components/dated-text";
import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect, useId, useMemo, useRef, useState, type ReactNode } from "react";
import {
  useForm,
  useWatch,
  type FieldErrors,
  type UseFormRegisterReturn,
  type UseFormSetError,
} from "react-hook-form";
import { useTranslation } from "react-i18next";
import { Link, useNavigate, useParams } from "react-router";
import { ApiFailure } from "@/api/client";
import { errorText, toError } from "@/api/errors";
import {
  endpointTypeValues,
  filterModeValues,
  minSeverityValues,
  nodeAutoscalingValues,
  reportProfileValues,
} from "@/api/schema.gen";
import { useAnnounce } from "@/app/live-region";
import { useScreenTitle } from "@/app/title";
import { Separator } from "@/components/separator";
import { Alert } from "@/components/ui/alert";
import { Button, buttonStyles } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Field } from "@/components/ui/field";
import { Select } from "@/components/ui/select";
import { TextArea } from "@/components/ui/textarea";
import { formatTime } from "@/lib/format";
import { useAdminProfile, useCreateProfile, useReviseProfile, useTestDraft, type Profile } from "./api";
import { Folded } from "./folded";
import {
  applyRefusal,
  documentOf,
  foldedSectionOf,
  newProfileValues,
  profileSchema,
  summaries,
  thresholdKeys,
  valuesOf,
  type ProfileValues,
} from "./form";
import { LeaveDialog, useLeaveGuard } from "./leave-guard";
import { outcomeText, TestResultLine, type TestOutcome } from "./test-result";

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-4">
      <h2 className="text-lg font-semibold">{title}</h2>
      {children}
    </section>
  );
}

// A group of checkboxes that share one field: the legend names the group, and the hint and the error describe each box.
function CheckGroup({
  legend,
  hint,
  error,
  options,
  registration,
}: {
  legend: string;
  hint: string;
  error: string | undefined;
  options: readonly { value: string; label: string }[];
  registration: UseFormRegisterReturn;
}) {
  const id = useId();
  const described = [`${id}-hint`, ...(error === undefined ? [] : [`${id}-error`])].join(" ");
  return (
    <fieldset className="flex flex-col gap-1">
      <legend className="text-sm font-medium">{legend}</legend>
      <div className="flex flex-wrap gap-x-4">
        {options.map((option) => (
          <Checkbox
            key={option.value}
            label={option.label}
            value={option.value}
            aria-invalid={error === undefined ? undefined : true}
            aria-describedby={described}
            {...registration}
          />
        ))}
      </div>
      <p id={`${id}-hint`} className="text-sm text-muted-fg">
        {hint}
      </p>
      {error === undefined ? null : (
        <p id={`${id}-error`} className="text-sm text-critical">
          {error}
        </p>
      )}
    </fieldset>
  );
}

// The paths of the fields in error, so the sections that hold them can be opened.
function errorPaths(errors: FieldErrors<ProfileValues>): string[] {
  return Object.entries(errors).flatMap(([name, error]) =>
    name === "thresholds"
      ? Object.keys(error).map((key) => `thresholds.${key}`)
      : [name],
  );
}

// The form of a new profile, or of a saved one; `reload` fetches the latest revision of the saved one again.
function ProfileForm({ profile, reload }: { profile: Profile | undefined; reload?: () => Promise<Profile> }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const announce = useAnnounce();
  const create = useCreateProfile();
  const revise = useReviseProfile(profile?.id ?? "");
  const testDraft = useTestDraft();
  const [failure, setFailure] = useState<string | null>(null);
  const [stale, setStale] = useState(false);
  const [asking, setAsking] = useState(false);
  const [outcome, setOutcome] = useState<TestOutcome | null>(null);
  // Counts the changes of the form, so a test that answers after one does not describe the form any more.
  const changes = useRef(0);
  const [choicesOpen, setChoicesOpen] = useState(false);
  const [thresholdsOpen, setThresholdsOpen] = useState(false);
  // Counts the attempts that left a field in error; each one moves the focus to the first such field once it is drawn.
  const [marks, setMarks] = useState(0);
  const form = useRef<HTMLFormElement>(null);
  const isNew = profile === undefined;
  const schema = useMemo(() => profileSchema(isNew), [isNew]);
  const {
    register,
    handleSubmit,
    setError,
    clearErrors,
    control,
    getValues,
    reset,
    formState: { errors, isDirty },
  } = useForm<ProfileValues>({
    resolver: zodResolver(schema),
    defaultValues: isNew ? newProfileValues() : valuesOf(profile),
    shouldFocusError: false,
  });
  // Every field has a default, so the watched values are complete.
  const values = useWatch({ control }) as ProfileValues;
  const summary = summaries(values, t);
  const pending = create.isPending || revise.isPending;
  const guard = useLeaveGuard(isDirty);

  useEffect(() => {
    if (marks > 0) {
      form.current?.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus();
    }
  }, [marks]);

  function open(section: "choices" | "thresholds") {
    if (section === "choices") {
      setChoicesOpen(true);
    } else {
      setThresholdsOpen(true);
    }
  }

  function invalid(found: FieldErrors<ProfileValues>) {
    for (const path of errorPaths(found)) {
      const section = foldedSectionOf(path);
      if (section !== null) {
        open(section);
      }
    }
    setMarks((count) => count + 1);
  }

  // The fields the API marked, which keep the mark until their value changes or the next answer.
  const marked = useRef(new Set<string>());
  const markField: UseFormSetError<ProfileValues> = (name, error, options) => {
    marked.current.add(name);
    setError(name, error, options);
  };

  function clearMarks() {
    for (const name of marked.current) {
      clearErrors(name as keyof ProfileValues);
    }
    marked.current.clear();
  }

  async function runTest() {
    clearMarks();
    const values = getValues();
    const seen = changes.current;
    setFailure(null);
    let found: TestOutcome;
    let refusal: ApiFailure | null = null;
    try {
      const id = values.id.trim();
      found = { kind: "result", result: await testDraft.mutateAsync({ ...documentOf(values), ...(id === "" ? {} : { id }) }) };
    } catch (thrown) {
      const error = toError(thrown);
      if (error instanceof ApiFailure && error.status === 422) {
        refusal = error;
        found = { kind: "refused" };
      } else {
        found = { kind: "failed", error };
      }
    }
    // An answer for values that have changed since describes a form that is gone.
    if (changes.current !== seen) {
      return;
    }
    if (refusal !== null) {
      if (!applyRefusal(refusal, markField, open, isNew)) {
        setFailure(errorText(t, refusal));
      }
      setMarks((count) => count + 1);
    }
    setOutcome(found);
    announce(outcomeText(t, found));
  }

  async function loadLatest() {
    setAsking(false);
    setFailure(null);
    try {
      const latest = await reload?.();
      if (latest !== undefined) {
        reset(valuesOf(latest));
      }
      setStale(false);
    } catch (thrown) {
      setFailure(errorText(t, toError(thrown)));
    }
  }

  async function submit(submitted: ProfileValues) {
    clearMarks();
    setFailure(null);
    setStale(false);
    try {
      const document = documentOf(submitted);
      const saved = isNew
        ? await create.mutateAsync({ id: submitted.id, ...document })
        : await revise.mutateAsync({ base_revision_id: profile.revision_id, ...document });
      announce(t(isNew ? "admin.profile.created" : "admin.profile.saved", { name: saved.display_name }));
      guard.release();
      void navigate(`/profiles?open=${saved.id}`);
    } catch (thrown) {
      const error = toError(thrown);
      if (error instanceof ApiFailure && error.code === "PROFILE_REVISION_STALE") {
        setStale(true);
      } else if (!(error instanceof ApiFailure && applyRefusal(error, markField, open, isNew))) {
        setFailure(errorText(t, error));
      }
      setMarks((count) => count + 1);
    }
  }

  const fieldError = (message: string | undefined) => (message === undefined ? undefined : t(message));
  const thresholdError = (key: (typeof thresholdKeys)[number]) => fieldError(errors.thresholds?.[key]?.message);

  return (
    <form
      ref={form}
      noValidate
      className="flex flex-col gap-8"
      onChange={(event) => {
        const name = (event.nativeEvent.target as HTMLInputElement).name;
        // The API marks the key `thresholds` on the first threshold; any threshold field takes the mark off.
        const names = name.startsWith("thresholds.") ? [name, "thresholds.warning_percent"] : [name];
        for (const marker of names) {
          if (marked.current.delete(marker)) {
            clearErrors(marker as keyof ProfileValues);
          }
        }
        changes.current += 1;
        setOutcome(null);
      }}
      onSubmit={(event) => {
        void handleSubmit(submit, invalid)(event);
      }}
    >
      <Section title={t("admin.profile.section.identity")}>
        {isNew ? (
          <Field
            label={t("admin.profile.id")}
            hint={t("admin.profile.idHint")}
            autoComplete="off"
            className="font-mono"
            error={fieldError(errors.id?.message)}
            {...register("id")}
          />
        ) : null}
        <Field
          label={t("admin.profile.name")}
          error={fieldError(errors.display_name?.message)}
          {...register("display_name")}
        />
        <Checkbox
          label={t("admin.profile.active")}
          hint={t("admin.profile.activeHint")}
          error={fieldError(errors.active?.message)}
          {...register("active")}
        />
      </Section>
      <Section title={t("admin.profile.section.source")}>
        <Field
          label={t("admin.profile.url")}
          className="font-mono"
          error={fieldError(errors.endpoint_url?.message)}
          {...register("endpoint_url")}
        />
        <Select
          label={t("admin.profile.type")}
          error={fieldError(errors.endpoint_type?.message)}
          {...register("endpoint_type")}
        >
          {endpointTypeValues.map((value) => (
            <option key={value} value={value}>
              {t(`endpointType.${value}`)}
            </option>
          ))}
        </Select>
        <Field
          label={t("admin.profile.timeout")}
          inputMode="decimal"
          className="font-mono"
          error={fieldError(errors.timeout_seconds?.message)}
          {...register("timeout_seconds")}
        />
        <Checkbox
          label={t("admin.profile.tls")}
          error={fieldError(errors.tls_verify?.message)}
          {...register("tls_verify")}
        />
        <Field
          label={t("admin.profile.credential")}
          hint={t("admin.profile.credentialHint")}
          className="font-mono"
          error={fieldError(errors.credential_file?.message)}
          {...register("credential_file")}
        />
        <Field
          label={t("admin.profile.ca")}
          hint={t("admin.profile.caHint")}
          className="font-mono"
          error={fieldError(errors.ca_bundle_file?.message)}
          {...register("ca_bundle_file")}
        />
        <div className="flex flex-wrap items-center gap-3">
          <Button
            disabled={testDraft.isPending}
            onClick={() => {
              void runTest();
            }}
          >
            {testDraft.isPending ? t("admin.profiles.testing") : t("admin.profiles.test")}
          </Button>
          <TestResultLine outcome={outcome} />
        </div>
      </Section>
      <Section title={t("admin.profile.section.cluster")}>
        <Field
          label={t("admin.profile.clusterName")}
          className="font-mono"
          error={fieldError(errors.cluster_name?.message)}
          {...register("cluster_name")}
        />
        <Field
          label={t("admin.profile.clusterLabel")}
          className="font-mono"
          error={fieldError(errors.cluster_label?.message)}
          {...register("cluster_label")}
        />
        <Select
          label={t("admin.profile.filter")}
          error={fieldError(errors.filter_mode?.message)}
          {...register("filter_mode")}
        >
          {filterModeValues.map((value) => (
            <option key={value} value={value}>
              {t(`filterMode.${value}`)}
            </option>
          ))}
        </Select>
        <Select
          label={t("admin.profile.autoscaling")}
          error={fieldError(errors.node_autoscaling?.message)}
          {...register("node_autoscaling")}
        >
          {nodeAutoscalingValues.map((value) => (
            <option key={value} value={value}>
              {t(`nodeAutoscaling.${value}`)}
            </option>
          ))}
        </Select>
        <Field
          label={t("admin.profile.poolLabel")}
          hint={t("admin.profile.poolHint")}
          className="font-mono"
          error={fieldError(errors.nodepool_label?.message)}
          {...register("nodepool_label")}
        />
      </Section>
      <Section title={t("admin.profile.section.namespaces")}>
        <TextArea
          label={t("admin.profile.allowed")}
          hint={t("admin.profile.onePerLine")}
          rows={5}
          error={fieldError(errors.allowed_namespaces?.message)}
          {...register("allowed_namespaces")}
        />
        <TextArea
          label={t("admin.profile.excluded")}
          hint={t("admin.profile.onePerLine")}
          rows={5}
          error={fieldError(errors.excluded_containers?.message)}
          {...register("excluded_containers")}
        />
      </Section>
      <Folded
        title={t("admin.profile.section.choices")}
        summary={summary.choices}
        open={choicesOpen}
        onToggle={() => {
          setChoicesOpen((was) => !was);
        }}
      >
        <TextArea
          label={t("admin.profile.periods")}
          hint={t("admin.profile.periodsHint")}
          rows={3}
          error={fieldError(errors.permitted_periods?.message)}
          {...register("permitted_periods")}
        />
        <Field
          label={t("admin.profile.defaultPeriod")}
          placeholder="24h"
          className="font-mono"
          error={fieldError(errors.default_period?.message)}
          {...register("default_period")}
        />
        <TextArea
          label={t("admin.profile.zones")}
          hint={t("admin.profile.zonesHint")}
          rows={3}
          error={fieldError(errors.permitted_timezones?.message)}
          {...register("permitted_timezones")}
        />
        <Field
          label={t("admin.profile.defaultZone")}
          placeholder="America/Sao_Paulo"
          className="font-mono"
          error={fieldError(errors.default_timezone?.message)}
          {...register("default_timezone")}
        />
        <CheckGroup
          legend={t("admin.profile.profiles")}
          hint={t("admin.profile.noneTickedHint")}
          error={fieldError(errors.permitted_report_profiles?.message)}
          options={reportProfileValues.map((value) => ({ value, label: t(`reportProfile.${value}`) }))}
          registration={register("permitted_report_profiles")}
        />
        <Select
          label={t("admin.profile.defaultProfile")}
          error={fieldError(errors.default_report_profile?.message)}
          {...register("default_report_profile")}
        >
          <option value="">{t("admin.profile.fallback", { value: t("reportProfile.detailed") })}</option>
          {reportProfileValues.map((value) => (
            <option key={value} value={value}>
              {t(`reportProfile.${value}`)}
            </option>
          ))}
        </Select>
        <CheckGroup
          legend={t("admin.profile.severities")}
          hint={t("admin.profile.noneTickedHint")}
          error={fieldError(errors.permitted_min_severities?.message)}
          options={minSeverityValues.map((value) => ({ value, label: t(`minSeverity.${value}`) }))}
          registration={register("permitted_min_severities")}
        />
        <Select
          label={t("admin.profile.defaultSeverity")}
          error={fieldError(errors.default_min_severity?.message)}
          {...register("default_min_severity")}
        >
          <option value="">{t("admin.profile.fallback", { value: t("minSeverity.all") })}</option>
          {minSeverityValues.map((value) => (
            <option key={value} value={value}>
              {t(`minSeverity.${value}`)}
            </option>
          ))}
        </Select>
      </Folded>
      <Folded
        title={t("admin.profile.section.thresholds")}
        summary={summary.thresholds}
        open={thresholdsOpen}
        onToggle={() => {
          setThresholdsOpen((was) => !was);
        }}
      >
        <div className="grid gap-4 sm:grid-cols-3">
          {thresholdKeys.map((key) => (
            <Field
              key={key}
              label={t(`admin.profile.threshold.${key}`)}
              inputMode="decimal"
              className="font-mono"
              error={thresholdError(key)}
              {...register(`thresholds.${key}`)}
            />
          ))}
        </div>
      </Folded>
      {stale ? (
        <Alert tone="error">
          <div className="flex flex-wrap items-center gap-3">
            <p>{t("admin.profile.stale")}</p>
            <Button
              onClick={() => {
                if (isDirty) {
                  setAsking(true);
                } else {
                  void loadLatest();
                }
              }}
            >
              {t("admin.profile.loadLatest")}
            </Button>
          </div>
        </Alert>
      ) : null}
      {failure === null ? null : <Alert tone="error">{failure}</Alert>}
      <div className="sticky bottom-0 z-10 flex flex-wrap items-center gap-3 border-t border-border bg-bg py-3">
        <Button type="submit" variant="primary" disabled={pending}>
          {pending ? t("admin.profile.saving") : t("admin.profile.save")}
        </Button>
        <Link to="/profiles" className={buttonStyles()} onClick={(event) => {
            if (event.button === 0 && !event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey) {
              guard.release();
            }
          }}
        >
          {t("admin.profile.cancel")}
        </Link>
        <p className="text-sm text-muted-fg">
          {isNew
            ? t("admin.profile.saveHintNew")
            : t("admin.profile.saveHintEdit", { next: profile.revision + 1 })}
        </p>
      </div>
      {guard.dialog}
      <LeaveDialog
        open={asking}
        onLeave={() => {
          void loadLatest();
        }}
        onStay={() => {
          setAsking(false);
        }}
      />
    </form>
  );
}

function Header({ profile }: { profile: Profile | undefined }) {
  const { t } = useTranslation();
  if (profile === undefined) {
    return <h1 className="text-xl font-semibold">{t("admin.profile.newTitle")}</h1>;
  }
  return (
    <div>
      <h1 className="text-xl font-semibold">{profile.display_name}</h1>
      <p className="text-sm text-muted-fg">
        {t("admin.profile.kind")}
        <Separator />
        <span className="font-mono">{profile.id}</span>
        <Separator />
        <DatedText i18nKey="admin.profile.revision" values={{ revision: profile.revision, time: formatTime(profile.created_at) }} />
      </p>
    </div>
  );
}

function EditProfile({ id }: { id: string }) {
  const { t } = useTranslation();
  const profile = useAdminProfile(id);
  if (profile.data === undefined && profile.isError) {
    return (
      <div className="flex flex-col items-start gap-4">
        <Alert tone="error">{errorText(t, profile.error)}</Alert>
        <Link to="/profiles" className={buttonStyles()}>
          {t("admin.profile.back")}
        </Link>
      </div>
    );
  }
  if (profile.data === undefined) {
    return null;
  }
  return (
    <>
      <Header profile={profile.data} />
      <ProfileForm
        key={profile.data.revision_id}
        profile={profile.data}
        reload={async () => (await profile.refetch({ throwOnError: true })).data as Profile}
      />
    </>
  );
}

// The form of a new profile, or of the one named in the address.
export function ProfileFormPage() {
  const { t } = useTranslation();
  const { id } = useParams();
  useScreenTitle(t(id === undefined ? "admin.profile.newTitle" : "admin.profile.editScreen"));
  return (
    <div className="flex max-w-3xl flex-col gap-6">
      {id === undefined ? (
        <>
          <Header profile={undefined} />
          <ProfileForm profile={undefined} />
        </>
      ) : (
        <EditProfile id={id} />
      )}
    </div>
  );
}
