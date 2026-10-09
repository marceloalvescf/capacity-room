import type { TFunction } from "i18next";
import type { components } from "@/api/schema.gen";
import type { Profile } from "./api";
import { fromLines, toLines } from "./lines";

type Schemas = components["schemas"];


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

const thresholdDefaults: Record<ThresholdKey, number> = {
  warning_percent: 70,
  critical_percent: 85,
  low_utilization_percent: 30,
  autoscaler_near_max_percent: 80,
  nodepool_spread_points: 30,
  bursty_gap_percent: 100,
  hpa_replica_divergence: 1,
};

function thresholdValues(profile: Profile): Record<ThresholdKey, string> {
  const given = profile.thresholds ?? {};
  const text = (key: ThresholdKey) => String(given[key] ?? thresholdDefaults[key]);
  return {
    warning_percent: text("warning_percent"),
    critical_percent: text("critical_percent"),
    low_utilization_percent: text("low_utilization_percent"),
    autoscaler_near_max_percent: text("autoscaler_near_max_percent"),
    nodepool_spread_points: text("nodepool_spread_points"),
    bursty_gap_percent: text("bursty_gap_percent"),
    hpa_replica_divergence: text("hpa_replica_divergence"),
  };
}

// The form's values of a saved profile; a default that is null is empty.
export function valuesOf(profile: Profile): ProfileValues {
  return {
    id: profile.id,
    display_name: profile.display_name ?? "",
    active: profile.active ?? false,
    endpoint_url: profile.endpoint_url ?? "",
    endpoint_type: profile.endpoint_type ?? "auto",
    timeout_seconds: String(profile.timeout_seconds ?? 90),
    tls_verify: profile.tls_verify ?? true,
    credential_file: profile.credential_file ?? "",
    ca_bundle_file: profile.ca_bundle_file ?? "",
    cluster_name: profile.cluster_name ?? "",
    cluster_label: profile.cluster_label ?? "cluster",
    filter_mode: profile.filter_mode ?? "required",
    nodepool_label: profile.nodepool_label ?? "",
    node_autoscaling: profile.node_autoscaling ?? "auto",
    allowed_namespaces: fromLines(profile.allowed_namespaces ?? []),
    excluded_containers: fromLines(profile.excluded_containers ?? []),
    permitted_periods: fromLines(profile.permitted_periods ?? []),
    default_period: profile.default_period ?? "",
    permitted_timezones: fromLines(profile.permitted_timezones ?? []),
    default_timezone: profile.default_timezone ?? "",
    permitted_report_profiles: profile.permitted_report_profiles ?? [],
    default_report_profile: profile.default_report_profile ?? "",
    permitted_min_severities: profile.permitted_min_severities ?? [],
    default_min_severity: profile.default_min_severity ?? "",
    thresholds: thresholdValues(profile),
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
