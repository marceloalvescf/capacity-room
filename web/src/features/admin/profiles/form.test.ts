import { describe, expect, it } from "vitest";
import { i18next } from "@/i18n/i18n";
import { adminProfile } from "../../../../tests/msw";
import { ApiFailure } from "@/api/client";
import {
  applyRefusal,
  documentOf,
  fieldProblems,
  newProfileValues,
  profileSchema,
  summaries,
  thresholdKeys,
  valuesOf,
  type ProfileValues,
} from "./form";
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

describe("new values", () => {
  it("holds the defaults of the API", () => {
    expect(newProfileValues()).toEqual({
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
    });
  });
});

describe("document of the values", () => {
  it("round-trips a saved profile", () => {
    const profile = adminProfile({
      credential_file: "production-token",
      allowed_namespaces: ["payments", "billing"],
      permitted_periods: ["24h", "7d"],
      default_period: "7d",
      permitted_report_profiles: ["executive"],
      default_min_severity: "critical",
    });
    const document = Object.fromEntries(
      Object.entries(profile).filter(([key]) => !["id", "revision_id", "revision", "created_at"].includes(key)),
    );
    expect(documentOf(valuesOf(profile))).toEqual(document);
  });

  it("sends an empty default as null and a typed one as typed", () => {
    const values = { ...newProfileValues(), default_period: "", default_timezone: "Europe/Lisbon" };
    const document = documentOf(values);
    expect(document.default_period).toBeNull();
    expect(document.default_timezone).toBe("Europe/Lisbon");
    expect(document.default_report_profile).toBeNull();
    expect(document.default_min_severity).toBeNull();
  });

  it("sends numbers as numbers and lists as trimmed lines", () => {
    const document = documentOf({ ...newProfileValues(), timeout_seconds: "45", allowed_namespaces: " a \n\nb" });
    expect(document.timeout_seconds).toBe(45);
    expect(document.thresholds.warning_percent).toBe(70);
    expect(document.allowed_namespaces).toEqual(["a", "b"]);
  });
});

describe("checks of the form", () => {
  const valid: ProfileValues = {
    ...newProfileValues(),
    id: "lab",
    display_name: "Lab",
    endpoint_url: "https://lab.example.com",
    cluster_name: "lab-1",
    allowed_namespaces: "payments",
  };

  function problems(values: ProfileValues, isNew = true): string[] {
    const parsed = profileSchema(isNew).safeParse(values);
    return parsed.success ? [] : parsed.error.issues.map((issue) => `${issue.path.join(".")}: ${issue.message}`);
  }

  it("refuses a number that is not finite", () => {
    expect(problems({ ...valid, thresholds: { ...valid.thresholds, critical_percent: "Infinity" } })).toEqual([
      "thresholds.critical_percent: validation.notNegative",
    ]);
    expect(problems({ ...valid, timeout_seconds: "Infinity" })).toEqual(["timeout_seconds: validation.timeout"]);
    expect(problems({ ...valid, timeout_seconds: "abc" })).toEqual(["timeout_seconds: validation.timeout"]);
  });

  it("bounds five thresholds to 0..100 and two to 0 or more", () => {
    const capped = [
      "warning_percent",
      "critical_percent",
      "low_utilization_percent",
      "autoscaler_near_max_percent",
      "nodepool_spread_points",
    ] as const;
    for (const key of capped) {
      const over = problems({ ...valid, thresholds: { ...valid.thresholds, [key]: "101" } });
      expect(over).toContain(`thresholds.${key}: validation.range`);
      expect(problems({ ...valid, thresholds: { ...valid.thresholds, [key]: "-1" } })).toContain(
        `thresholds.${key}: validation.notNegative`,
      );
    }
    expect(problems({ ...valid, thresholds: { ...valid.thresholds, critical_percent: "100", warning_percent: "99" } })).toEqual([]);
    expect(problems({ ...valid, thresholds: { ...valid.thresholds, bursty_gap_percent: "250", hpa_replica_divergence: "5" } })).toEqual([]);
  });

  it("accepts a complete profile", () => {
    expect(problems(valid)).toEqual([]);
  });

  it("refuses each wrong value with its own message on its own field", () => {
    expect(problems({ ...valid, id: "" })).toEqual(["id: validation.required"]);
    expect(problems({ ...valid, id: "Bad" })).toEqual(["id: validation.identifier"]);
    expect(problems({ ...valid, id: "a..b" })).toEqual(["id: validation.identifier"]);
    expect(problems({ ...valid, id: "" }, false)).toEqual([]);
    expect(problems({ ...valid, display_name: "" })).toEqual(["display_name: validation.required"]);
    expect(problems({ ...valid, endpoint_url: "" })).toEqual(["endpoint_url: validation.required"]);
    expect(problems({ ...valid, cluster_name: "" })).toEqual(["cluster_name: validation.required"]);
    expect(problems({ ...valid, timeout_seconds: "0" })).toEqual(["timeout_seconds: validation.timeout"]);
    expect(problems({ ...valid, timeout_seconds: "601" })).toEqual(["timeout_seconds: validation.timeout"]);
    expect(problems({ ...valid, timeout_seconds: "" })).toEqual(["timeout_seconds: validation.timeout"]);
    expect(problems({ ...valid, tls_verify: false, ca_bundle_file: "ca" })).toEqual([
      "ca_bundle_file: validation.caWithoutTls",
    ]);
    expect(problems({ ...valid, permitted_periods: "24h", default_period: "7d" })).toEqual([
      "default_period: validation.defaultNotPermitted",
    ]);
    expect(problems({ ...valid, permitted_timezones: "UTC", default_timezone: "Europe/Lisbon" })).toEqual([
      "default_timezone: validation.defaultNotPermitted",
    ]);
    expect(
      problems({ ...valid, permitted_report_profiles: ["executive"], default_report_profile: "detailed" }),
    ).toEqual(["default_report_profile: validation.defaultNotPermitted"]);
    expect(problems({ ...valid, permitted_min_severities: ["all"], default_min_severity: "critical" })).toEqual([
      "default_min_severity: validation.defaultNotPermitted",
    ]);
    expect(problems({ ...valid, permitted_periods: "24h", default_period: "24h" })).toEqual([]);
    expect(problems({ ...valid, thresholds: { ...valid.thresholds, low_utilization_percent: "-1" } })).toEqual([
      "thresholds.low_utilization_percent: validation.notNegative",
    ]);
    expect(
      problems({ ...valid, thresholds: { ...valid.thresholds, warning_percent: "90", critical_percent: "85" } }),
    ).toEqual(["thresholds.warning_percent: validation.warningBelowCritical"]);
    expect(problems({ ...valid, allowed_namespaces: " \n " })).toEqual(["allowed_namespaces: validation.namespaceList"]);
  });
});

describe("refusal of the API", () => {
  function refuse(failure: ApiFailure, isNew = true) {
    const set: [string, string | undefined][] = [];
    const opened: string[] = [];
    const landed = applyRefusal(
      failure,
      (name, error) => {
        set.push([name, error.message]);
      },
      (section) => {
        opened.push(section);
      },
      isNew,
    );
    return { landed, set, opened };
  }

  it("names the six words of a field problem", () => {
    expect(fieldProblems).toEqual(["missing", "invalid", "repeated", "empty", "not_permitted", "incoherent"]);
  });

  it("places each field of a document refusal and opens its section", () => {
    const result = refuse(
      new ApiFailure(422, "PROFILE_DOCUMENT_INVALID", null, {
        default_timezone: "invalid",
        thresholds: "incoherent",
        display_name: "missing",
      }),
    );
    expect(result.landed).toBe(true);
    expect(result.set).toEqual([
      ["default_timezone", "fieldProblem.invalid"],
      ["thresholds.warning_percent", "fieldProblem.incoherent"],
      ["display_name", "fieldProblem.missing"],
    ]);
    expect(result.opened.sort()).toEqual(["choices", "thresholds"]);
  });

  it("lands nowhere on a field the form lacks", () => {
    const result = refuse(new ApiFailure(422, "PROFILE_DOCUMENT_INVALID", null, { nope: "invalid" }));
    expect(result).toEqual({ landed: false, set: [], opened: [] });
  });

  it("is not whole when a key the form lacks comes beside one that landed", () => {
    const result = refuse(
      new ApiFailure(422, "PROFILE_DOCUMENT_INVALID", null, { default_timezone: "invalid", nope: "invalid" }),
    );
    expect(result.landed).toBe(false);
    expect(result.set).toEqual([["default_timezone", "fieldProblem.invalid"]]);
    expect(result.opened).toEqual(["choices"]);
  });

  it("holds no identifier field on an edit", () => {
    const failure = new ApiFailure(422, "PROFILE_DOCUMENT_INVALID", null, { id: "invalid" });
    expect(refuse(failure, true).landed).toBe(true);
    expect(refuse(failure, false)).toEqual({ landed: false, set: [], opened: [] });
  });

  it("places the refusals of the address and of the identifier", () => {
    expect(refuse(new ApiFailure(422, "ENDPOINT_ADDRESS_REJECTED", null)).set).toEqual([
      ["endpoint_url", "error.ENDPOINT_ADDRESS_REJECTED"],
    ]);
    expect(refuse(new ApiFailure(422, "ENDPOINT_URL_REJECTED", null)).set).toEqual([
      ["endpoint_url", "error.ENDPOINT_URL_REJECTED"],
    ]);
    expect(refuse(new ApiFailure(409, "PROFILE_ID_TAKEN", null)).set).toEqual([["id", "error.PROFILE_ID_TAKEN"]]);
    expect(refuse(new ApiFailure(500, "INTERNAL_ERROR", null)).landed).toBe(false);
  });
});
