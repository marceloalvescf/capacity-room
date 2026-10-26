import type { components } from "@/api/schema.gen";

type View = components["schemas"]["ReportView"];

// The view an executive report would give from a detailed one: the findings below attention are left out, the totals
// list critical and attention alone, and what pointed at a finding left out points at none. The findings of the
// detailed views are ordered by severity, so the ones kept keep their indices.
export function executiveOf(view: View): View {
  const findings = view.findings.filter((finding) => finding.severity !== "healthy");
  const kept = (index: number) => index < findings.length;
  const count = (severity: "critical" | "attention") => findings.filter((finding) => finding.severity === severity).length;
  return {
    ...view,
    report: { ...view.report, profile: "executive" },
    findings,
    recommendations: view.recommendations.filter((recommendation) => recommendation.finding !== null && kept(recommendation.finding)),
    verdict: {
      ...view.verdict,
      totals: [
        { severity: "critical", count: count("critical") },
        { severity: "attention", count: count("attention") },
      ],
      cluster_finding: view.verdict.cluster_finding !== null && kept(view.verdict.cluster_finding) ? view.verdict.cluster_finding : null,
      scopes: view.verdict.scopes.filter((scope) => kept(scope.finding)),
    },
  };
}

// An executive view with nothing to show: no finding, no recommendation, no scope, and totals of zero.
export function emptyOf(view: View): View {
  return {
    ...view,
    report: { ...view.report, profile: "executive" },
    findings: [],
    recommendations: [],
    verdict: {
      ...view.verdict,
      totals: [
        { severity: "critical", count: 0 },
        { severity: "attention", count: 0 },
      ],
      cluster_finding: null,
      scopes: [],
      pressure: [],
    },
  };
}
