import { zodResolver } from "@hookform/resolvers/zod";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useForm, useWatch, type FieldErrors } from "react-hook-form";
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
import { Alert } from "@/components/ui/alert";
import { Button, buttonStyles } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Field } from "@/components/ui/field";
import { Select } from "@/components/ui/select";
import { TextArea } from "@/components/ui/textarea";
import { formatTime } from "@/lib/format";
import { useAdminProfile, useCreateProfile, useReviseProfile, type Profile } from "./api";
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

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-4">
      <h2 className="text-lg font-semibold">{title}</h2>
      {children}
    </section>
  );
}

function Choices({ legend, hint, children }: { legend: string; hint: string; children: ReactNode }) {
  return (
    <fieldset className="flex flex-col gap-1">
      <legend className="text-sm font-medium">{legend}</legend>
      <div className="flex flex-wrap gap-x-4">{children}</div>
      <p className="text-sm text-muted-fg">{hint}</p>
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

function ProfileForm({ profile }: { profile: Profile | undefined }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const announce = useAnnounce();
  const create = useCreateProfile();
  const revise = useReviseProfile(profile?.id ?? "");
  const [failure, setFailure] = useState<string | null>(null);
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
    control,
    formState: { errors },
  } = useForm<ProfileValues>({
    resolver: zodResolver(schema),
    defaultValues: isNew ? newProfileValues() : valuesOf(profile),
    shouldFocusError: false,
  });
  // Every field has a default, so the watched values are complete.
  const values = useWatch({ control }) as ProfileValues;
  const summary = summaries(values, t);
  const pending = create.isPending || revise.isPending;

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

  async function submit(submitted: ProfileValues) {
    setFailure(null);
    try {
      const document = documentOf(submitted);
      const saved = isNew
        ? await create.mutateAsync({ id: submitted.id, ...document })
        : await revise.mutateAsync({ base_revision_id: profile.revision_id, ...document });
      announce(t(isNew ? "admin.profile.created" : "admin.profile.saved", { name: saved.display_name }));
      void navigate(`/profiles?open=${saved.id}`);
    } catch (thrown) {
      const error = toError(thrown);
      if (error instanceof ApiFailure && applyRefusal(error, setError, open)) {
        setMarks((count) => count + 1);
      } else {
        setFailure(errorText(t, error));
      }
    }
  }

  const fieldError = (message: string | undefined) => (message === undefined ? undefined : t(message));
  const thresholdError = (key: (typeof thresholdKeys)[number]) => fieldError(errors.thresholds?.[key]?.message);

  return (
    <form
      ref={form}
      noValidate
      className="flex flex-col gap-8"
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
        <div>
          <Checkbox label={t("admin.profile.active")} {...register("active")} />
          <p className="text-sm text-muted-fg">{t("admin.profile.activeHint")}</p>
        </div>
      </Section>
      <Section title={t("admin.profile.section.source")}>
        <Field
          label={t("admin.profile.url")}
          className="font-mono"
          error={fieldError(errors.endpoint_url?.message)}
          {...register("endpoint_url")}
        />
        <Select label={t("admin.profile.type")} {...register("endpoint_type")}>
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
        <Checkbox label={t("admin.profile.tls")} {...register("tls_verify")} />
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
        <Select label={t("admin.profile.filter")} {...register("filter_mode")}>
          {filterModeValues.map((value) => (
            <option key={value} value={value}>
              {t(`filterMode.${value}`)}
            </option>
          ))}
        </Select>
        <Select label={t("admin.profile.autoscaling")} {...register("node_autoscaling")}>
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
        <Choices legend={t("admin.profile.profiles")} hint={t("admin.profile.noneTickedHint")}>
          {reportProfileValues.map((value) => (
            <Checkbox
              key={value}
              label={t(`reportProfile.${value}`)}
              value={value}
              {...register("permitted_report_profiles")}
            />
          ))}
        </Choices>
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
        <Choices legend={t("admin.profile.severities")} hint={t("admin.profile.noneTickedHint")}>
          {minSeverityValues.map((value) => (
            <Checkbox
              key={value}
              label={t(`minSeverity.${value}`)}
              value={value}
              {...register("permitted_min_severities")}
            />
          ))}
        </Choices>
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
      {failure === null ? null : <Alert tone="error">{failure}</Alert>}
      <div className="sticky bottom-0 z-10 flex flex-wrap items-center gap-3 border-t border-border bg-bg py-3">
        <Button type="submit" variant="primary" disabled={pending}>
          {pending ? t("admin.profile.saving") : t("admin.profile.save")}
        </Button>
        <Link to="/profiles" className={buttonStyles()}>
          {t("admin.profile.cancel")}
        </Link>
        <p className="text-sm text-muted-fg">
          {isNew
            ? t("admin.profile.saveHintNew")
            : t("admin.profile.saveHintEdit", { next: profile.revision + 1 })}
        </p>
      </div>
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
      <p className="text-sm text-muted-fg">{t("admin.profile.eyebrow")}</p>
      <h1 className="text-xl font-semibold">{profile.display_name}</h1>
      <p className="text-sm text-muted-fg">
        <span className="font-mono">{profile.id}</span>
        {" · "}
        {t("admin.profile.revision", { revision: profile.revision, time: formatTime(profile.created_at) })}
      </p>
    </div>
  );
}

function EditProfile({ id }: { id: string }) {
  const { t } = useTranslation();
  const profile = useAdminProfile(id);
  if (profile.isError) {
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
      <ProfileForm key={profile.data.revision_id} profile={profile.data} />
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
