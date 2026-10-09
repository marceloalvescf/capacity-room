import { describe, expect, it } from "vitest";
import { i18next } from "@/i18n/i18n";
import { adminProfile } from "../../../../tests/msw";
import { summaries, thresholdKeys, valuesOf } from "./form";
import { fromLines, toLines } from "./lines";

describe("lines", () => {
  it("trims each line and drops the empty ones", () => {
    expect(toLines("a\n  b \n\n a")).toEqual(["a", "b", "a"]);
    expect(fromLines(["a", "b"])).toBe("a\nb");
  });
});

describe("values of a profile", () => {
  it("writes numbers as strings, lists as lines, and a null default as empty", () => {
    const profile = adminProfile({
      id: "production",
      display_name: "Production",
      active: false,
      endpoint_type: "grafana",
      timeout_seconds: 45,
      tls_verify: false,
      credential_file: "production-token",
      ca_bundle_file: "",
      cluster_name: "prod-1",
      nodepool_label: "agentpool",
      allowed_namespaces: ["payments", "billing"],
      excluded_containers: ["istio-proxy"],
      permitted_periods: ["24h", "7d"],
      default_period: null,
      permitted_timezones: ["UTC"],
      default_timezone: "UTC",
      permitted_report_profiles: ["executive"],
      default_report_profile: null,
      permitted_min_severities: ["all", "critical"],
      default_min_severity: "critical",
    });
    expect(valuesOf(profile)).toEqual({
      id: "production",
      display_name: "Production",
      active: false,
      endpoint_url: "https://prometheus.example.com",
      endpoint_type: "grafana",
      timeout_seconds: "45",
      tls_verify: false,
      credential_file: "production-token",
      ca_bundle_file: "",
      cluster_name: "prod-1",
      cluster_label: "cluster",
      filter_mode: "required",
      nodepool_label: "agentpool",
      node_autoscaling: "auto",
      allowed_namespaces: "payments\nbilling",
      excluded_containers: "istio-proxy",
      permitted_periods: "24h\n7d",
      default_period: "",
      permitted_timezones: "UTC",
      default_timezone: "UTC",
      permitted_report_profiles: ["executive"],
      default_report_profile: "",
      permitted_min_severities: ["all", "critical"],
      default_min_severity: "critical",
      thresholds: {
        warning_percent: "70",
        critical_percent: "85",
        low_utilization_percent: "30",
        autoscaler_near_max_percent: "80",
        nodepool_spread_points: "30",
        bursty_gap_percent: "100",
        hpa_replica_divergence: "1",
      },
    });
  });
});

describe("summaries", () => {
  const t = i18next.getFixedT("en");

  it("words the choices and the thresholds", () => {
    const values = valuesOf(
      adminProfile({ permitted_periods: ["24h", "7d"], default_period: "7d", default_report_profile: "executive" }),
    );
    expect(summaries(values, t)).toEqual({
      choices: "24h, 7d · 7d · America/Sao_Paulo · Executive · All",
      thresholds:
        "Attention 70 % · Critical 85 % · Low utilization 30 % · Near maximum 80 % · Node pool spread 30 · Variation 100 % · Divergence 1",
    });
  });

  it("falls back to the defaults of an empty choice", () => {
    expect(summaries(valuesOf(adminProfile()), t).choices).toBe("Any period · 24h · America/Sao_Paulo · Detailed · All");
  });

  it("names the seven thresholds", () => {
    expect(thresholdKeys).toEqual([
      "warning_percent",
      "critical_percent",
      "low_utilization_percent",
      "autoscaler_near_max_percent",
      "nodepool_spread_points",
      "bursty_gap_percent",
      "hpa_replica_divergence",
    ]);
  });
});
