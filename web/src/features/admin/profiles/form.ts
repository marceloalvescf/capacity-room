import type { TFunction } from "i18next";
import type { UseFormSetError } from "react-hook-form";
import { z } from "zod";
import { ApiFailure } from "@/api/client";
import type { components } from "@/api/schema.gen";
import type { Profile, ProfileDocument } from "./api";
import { fromLines, toLines } from "./lines";

type Schemas = components["schemas"];

// The thresholds the API bounds to 0..100.
const cappedThresholds: readonly string[] = [
  "warning_percent",
  "critical_percent",
  "low_utilization_percent",
  "autoscaler_near_max_percent",
  "nodepool_spread_points",
];

// The seven thresholds of a profile, in the order the summary words them.
export const thresholdKeys = [
  "warning_percent",
  "critical_percent",
  "low_utilization_percent",
  "autoscaler_near_max_percent",
  "nodepool_spread_points",
  "bursty_gap_percent",
  "hpa_replica_divergence",
] as const satisfies readonly (keyof Schemas["ProfileThresholds"])[];

export type ThresholdKey = (typeof thresholdKeys)[number];

// The values of the form of a profile: a typed number or list is kept as typed, so every number is a string and
// every list of names is the text of a box with one item per line.
export interface ProfileValues {
  id: string;
  display_name: string;
  active: boolean;
  endpoint_url: string;
  endpoint_type: Schemas["EndpointType"];
  timeout_seconds: string;
  tls_verify: boolean;
  credential_file: string;
  ca_bundle_file: string;
  cluster_name: string;
  cluster_label: string;
  filter_mode: Schemas["FilterMode"];
  nodepool_label: string;
  node_autoscaling: Schemas["NodeAutoscaling"];
  allowed_namespaces: string;
  excluded_containers: string;
  permitted_periods: string;
  default_period: string;
  permitted_timezones: string;
  default_timezone: string;
  permitted_report_profiles: Schemas["ReportProfile"][];
  default_report_profile: string;
  permitted_min_severities: Schemas["MinSeverity"][];
  default_min_severity: string;
  thresholds: Record<ThresholdKey, string>;
}

// The form's values of a saved profile; a default that is null is empty.
export function valuesOf(profile: Profile): ProfileValues {
  return {
    id: profile.id,
    display_name: profile.display_name,
    active: profile.active,
    endpoint_url: profile.endpoint_url,
    endpoint_type: profile.endpoint_type,
    timeout_seconds: String(profile.timeout_seconds),
    tls_verify: profile.tls_verify,
    credential_file: profile.credential_file,
    ca_bundle_file: profile.ca_bundle_file,
    cluster_name: profile.cluster_name,
    cluster_label: profile.cluster_label,
    filter_mode: profile.filter_mode,
    nodepool_label: profile.nodepool_label,
    node_autoscaling: profile.node_autoscaling,
    allowed_namespaces: fromLines(profile.allowed_namespaces),
    excluded_containers: fromLines(profile.excluded_containers),
    permitted_periods: fromLines(profile.permitted_periods),
    default_period: profile.default_period ?? "",
    permitted_timezones: fromLines(profile.permitted_timezones),
    default_timezone: profile.default_timezone ?? "",
    permitted_report_profiles: profile.permitted_report_profiles,
    default_report_profile: profile.default_report_profile ?? "",
    permitted_min_severities: profile.permitted_min_severities,
    default_min_severity: profile.default_min_severity ?? "",
    thresholds: {
      warning_percent: String(profile.thresholds.warning_percent),
      critical_percent: String(profile.thresholds.critical_percent),
      low_utilization_percent: String(profile.thresholds.low_utilization_percent),
      autoscaler_near_max_percent: String(profile.thresholds.autoscaler_near_max_percent),
      nodepool_spread_points: String(profile.thresholds.nodepool_spread_points),
      bursty_gap_percent: String(profile.thresholds.bursty_gap_percent),
      hpa_replica_divergence: String(profile.thresholds.hpa_replica_divergence),
    },
  };
}

// The two lines that sum up the choices of the report form and the thresholds of a profile.
export function summaries(values: ProfileValues, t: TFunction): { choices: string; thresholds: string } {
  const periods = toLines(values.permitted_periods);
  const choices = [
    periods.length === 0 ? t("admin.profile.anyPeriod") : periods.join(", "),
    values.default_period === "" ? "24h" : values.default_period,
    values.default_timezone === "" ? "America/Sao_Paulo" : values.default_timezone,
    t(`reportProfile.${values.default_report_profile === "" ? "detailed" : values.default_report_profile}`),
    t(`minSeverity.${values.default_min_severity === "" ? "all" : values.default_min_severity}`),
  ];
  return {
    choices: choices.join(" · "),
    thresholds: thresholdKeys
      .map((key) => t(`admin.profile.short.${key}`, { value: values.thresholds[key] }))
      .join(" · "),
  };
}

// The values of a profile that is not saved yet: the defaults of the API, so a field left alone is what the API
// would have stored.
export function newProfileValues(): ProfileValues {
  return {
    id: "",
    display_name: "",
    active: false,
    endpoint_url: "",
    endpoint_type: "auto",
    timeout_seconds: "90",
    tls_verify: true,
    credential_file: "",
    ca_bundle_file: "",
    cluster_name: "",
    cluster_label: "cluster",
    filter_mode: "required",
    nodepool_label: "",
    node_autoscaling: "auto",
    allowed_namespaces: "",
    excluded_containers: "",
    permitted_periods: "",
    default_period: "",
    permitted_timezones: "",
    default_timezone: "",
    permitted_report_profiles: [],
    default_report_profile: "",
    permitted_min_severities: [],
    default_min_severity: "",
    thresholds: {
      warning_percent: "70",
      critical_percent: "85",
      low_utilization_percent: "30",
      autoscaler_near_max_percent: "80",
      nodepool_spread_points: "30",
      bursty_gap_percent: "100",
      hpa_replica_divergence: "1",
    },
  };
}

// A default left empty is sent as null, which the API reads as its own default.
function nullIfEmpty(text: string): string | null {
  return text === "" ? null : text;
}

// The document the API stores for the values of the form.
export function documentOf(values: ProfileValues): ProfileDocument {
  return {
    display_name: values.display_name,
    active: values.active,
    endpoint_url: values.endpoint_url,
    endpoint_type: values.endpoint_type,
    timeout_seconds: Number(values.timeout_seconds),
    tls_verify: values.tls_verify,
    credential_file: values.credential_file,
    ca_bundle_file: values.ca_bundle_file,
    cluster_name: values.cluster_name,
    cluster_label: values.cluster_label,
    filter_mode: values.filter_mode,
    nodepool_label: values.nodepool_label,
    node_autoscaling: values.node_autoscaling,
    allowed_namespaces: toLines(values.allowed_namespaces),
    excluded_containers: toLines(values.excluded_containers),
    permitted_periods: toLines(values.permitted_periods),
    default_period: nullIfEmpty(values.default_period),
    permitted_timezones: toLines(values.permitted_timezones),
    default_timezone: nullIfEmpty(values.default_timezone),
    permitted_report_profiles: values.permitted_report_profiles,
    default_report_profile: nullIfEmpty(values.default_report_profile),
    permitted_min_severities: values.permitted_min_severities,
    default_min_severity: nullIfEmpty(values.default_min_severity),
    thresholds: {
      warning_percent: Number(values.thresholds.warning_percent),
      critical_percent: Number(values.thresholds.critical_percent),
      low_utilization_percent: Number(values.thresholds.low_utilization_percent),
      autoscaler_near_max_percent: Number(values.thresholds.autoscaler_near_max_percent),
      nodepool_spread_points: Number(values.thresholds.nodepool_spread_points),
      bursty_gap_percent: Number(values.thresholds.bursty_gap_percent),
      hpa_replica_divergence: Number(values.thresholds.hpa_replica_divergence),
    },
  };
}

const identifier = /^[a-z0-9](?:[a-z0-9._-]{0,61}[a-z0-9])?$/;

// A typed number, or NaN when the text is empty, not a number, or not finite.
function numberOf(text: string): number {
  const number = text.trim() === "" ? Number.NaN : Number(text);
  return Number.isFinite(number) ? number : Number.NaN;
}

// The fields of the form that sit in the folded section of the choices of the report form.
const choiceFields: readonly string[] = [
  "permitted_periods",
  "default_period",
  "permitted_timezones",
  "default_timezone",
  "permitted_report_profiles",
  "default_report_profile",
  "permitted_min_severities",
  "default_min_severity",
];

// The section that holds a field of the form, when it is one of the folded ones.
export function foldedSectionOf(field: string): "choices" | "thresholds" | null {
  if (field === "thresholds" || field.startsWith("thresholds.")) {
    return "thresholds";
  }
  return choiceFields.includes(field) ? "choices" : null;
}

// What the browser checks before it sends; the rest is the API's to judge.
export function profileSchema(isNew: boolean): z.ZodType<ProfileValues, ProfileValues> {
  return z.custom<ProfileValues>().superRefine((values, context) => {
    function refuse(path: string, message: string) {
      context.addIssue({ code: "custom", path: path.split("."), message });
    }
    if (isNew) {
      if (values.id === "") {
        refuse("id", "validation.required");
      } else if (!identifier.test(values.id) || values.id.includes("..")) {
        refuse("id", "validation.identifier");
      }
    }
    for (const field of ["display_name", "endpoint_url", "cluster_name"] as const) {
      if (values[field] === "") {
        refuse(field, "validation.required");
      }
    }
    if (toLines(values.allowed_namespaces).length === 0) {
      refuse("allowed_namespaces", "validation.namespaceList");
    }
    const timeout = numberOf(values.timeout_seconds);
    if (!(timeout > 0 && timeout <= 600)) {
      refuse("timeout_seconds", "validation.timeout");
    }
    if (!values.tls_verify && values.ca_bundle_file !== "") {
      refuse("ca_bundle_file", "validation.caWithoutTls");
    }
    const defaults: [readonly string[], string, string][] = [
      [toLines(values.permitted_periods), values.default_period, "default_period"],
      [toLines(values.permitted_timezones), values.default_timezone, "default_timezone"],
      [values.permitted_report_profiles, values.default_report_profile, "default_report_profile"],
      [values.permitted_min_severities, values.default_min_severity, "default_min_severity"],
    ];
    for (const [permitted, value, field] of defaults) {
      if (value !== "" && permitted.length > 0 && !permitted.includes(value)) {
        refuse(field, "validation.defaultNotPermitted");
      }
    }
    for (const key of thresholdKeys) {
      const value = numberOf(values.thresholds[key]);
      if (!(value >= 0)) {
        refuse(`thresholds.${key}`, "validation.notNegative");
      } else if (value > 100 && cappedThresholds.includes(key)) {
        refuse(`thresholds.${key}`, "validation.range");
      }
    }
    const warning = numberOf(values.thresholds.warning_percent);
    const critical = numberOf(values.thresholds.critical_percent);
    if (warning >= 0 && critical >= 0 && warning >= critical) {
      refuse("thresholds.warning_percent", "validation.warningBelowCritical");
    }
  });
}

// The words the API gives a field it refused.
export const fieldProblems = ["missing", "invalid", "repeated", "empty", "not_permitted", "incoherent"] as const;

type FieldProblem = (typeof fieldProblems)[number];

function isFieldProblem(word: string): word is FieldProblem {
  return fieldProblems.some((problem) => problem === word);
}

const documentFields: readonly string[] = [
  "id",
  "display_name",
  "active",
  "endpoint_url",
  "endpoint_type",
  "timeout_seconds",
  "tls_verify",
  "credential_file",
  "ca_bundle_file",
  "cluster_name",
  "cluster_label",
  "filter_mode",
  "nodepool_label",
  "node_autoscaling",
  "allowed_namespaces",
  "excluded_containers",
  ...choiceFields,
];

// Puts the refusal of the API on the fields of the form and opens the folded section that holds one. Returns
// true when every key of the refusal landed on a field; otherwise the caller words the refusal in the alert.
// An edit holds no identifier field.
export function applyRefusal(
  failure: ApiFailure,
  setError: UseFormSetError<ProfileValues>,
  open: (section: "choices" | "thresholds") => void,
  isNew: boolean,
): boolean {
  let whole = true;
  const landed: string[] = [];
  function place(field: Parameters<UseFormSetError<ProfileValues>>[0], message: string) {
    setError(field, { message });
    landed.push(field);
    const section = foldedSectionOf(field);
    if (section !== null) {
      open(section);
    }
  }
  if (failure.code === "PROFILE_DOCUMENT_INVALID") {
    for (const [field, word] of Object.entries(failure.fields)) {
      const message = `fieldProblem.${isFieldProblem(word) ? word : "invalid"}`;
      if (field === "thresholds") {
        place("thresholds.warning_percent", message);
      } else if (documentFields.includes(field) && (isNew || field !== "id")) {
        place(field as keyof ProfileValues, message);
      } else {
        whole = false;
      }
    }
  } else if (failure.code === "ENDPOINT_URL_REJECTED" || failure.code === "ENDPOINT_ADDRESS_REJECTED") {
    place("endpoint_url", `error.${failure.code}`);
  } else if (failure.code === "PROFILE_ID_TAKEN") {
    place("id", `error.${failure.code}`);
  }
  return landed.length > 0 && whole;
}
