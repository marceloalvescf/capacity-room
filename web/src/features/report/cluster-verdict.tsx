import { StatusMark, StatusShape } from "@/components/status-mark";
import type { ReportView } from "./api";

// The verdict on the cluster: the finding and the warning the view names, that the evidence of the nodes is
// incomplete. With neither, nothing.
export function ClusterVerdict({ view }: { view: ReportView }) {
  const { findings, quality, verdict } = view;
  const worst = verdict.cluster_finding === null ? null : (findings[verdict.cluster_finding] ?? null);
  const evidenceWarning = verdict.node_evidence_warning === null ? null : (quality.warnings[verdict.node_evidence_warning] ?? null);
  if (worst === null && evidenceWarning === null) {
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
      {evidenceWarning === null ? null : (
        <p className="flex items-baseline gap-1.5 text-sm">
          <StatusShape status="attention" />
          <span>{evidenceWarning.text}</span>
        </p>
      )}
    </>
  );
}
