import { StatusMark, StatusShape } from "@/components/status-mark";
import type { ViewFinding } from "./api";
import { useView } from "./report-page";

const order = ["critical", "attention", "healthy"] as const;

// The worst finding of the cluster, the first of the worst severity; null when the cluster has none.
export function worstClusterFinding(findings: ViewFinding[]): ViewFinding | null {
  return findings
    .filter((finding) => finding.scope_type === "cluster")
    .reduce<ViewFinding | null>(
      (best, finding) => (best === null || order.indexOf(finding.severity) < order.indexOf(best.severity) ? finding : best),
      null,
    );
}

// The verdict on the cluster: its worst finding, then the warning that the evidence of the nodes is incomplete.
// With neither, nothing.
export function ClusterVerdict() {
  const { findings, quality } = useView();
  const worst = worstClusterFinding(findings);
  const evidenceWarning = quality.warnings.find((warning) => warning.code === "NODE_CAPACITY_EVIDENCE_INCOMPLETE");
  if (worst === null && evidenceWarning === undefined) {
    return null;
  }
  return (
    <>
      {worst === null ? null : (
        <div className="flex flex-col gap-1">
          <StatusMark status={worst.severity} />
          <p className="text-sm">{worst.sentence}</p>
        </div>
      )}
      {evidenceWarning === undefined ? null : (
        <p className="flex items-baseline gap-1.5 text-sm text-attention">
          <StatusShape status="attention" />
          <span>{evidenceWarning.text}</span>
        </p>
      )}
    </>
  );
}
