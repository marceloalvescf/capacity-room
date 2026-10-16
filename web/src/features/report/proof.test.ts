import { describe, expect, it } from "vitest";
import type { components } from "@/api/schema.gen";
import viewEn from "../../../tests/fixtures/view/edge-scaling.en.json";
import directComplete from "../../../tests/fixtures/view/direct-complete.en.json";
import type { ViewFinding } from "./api";
import { proofOf } from "./proof";

type View = components["schemas"]["ReportView"];

const findings = [...(viewEn as View).findings, ...(directComplete as View).findings];

function first(code: string): ViewFinding {
  const found = findings.find((finding) => finding.code === code);
  if (found === undefined) {
    throw new Error(`no finding ${code} in the golden views`);
  }
  return found;
}

// The golden views no longer hold a finding of near maximum, which a report
// stored before still can.
const nearMax: ViewFinding = {
  ...first("HPA_MAX_REPLICAS_REACHED"),
  code: "NODE_AUTOSCALING_NEAR_MAX",
  scope_type: "cluster",
  evidence: [
    { name: "current_nodes", raw: 3, text: "3" },
    { name: "max_nodes", raw: 3, text: "3" },
  ],
};

const names = (proof: ReturnType<typeof proofOf>) =>
  proof === null ? null : { lead: proof.lead.name, rest: proof.rest.map((item) => item.name) };

describe("proof", () => {
  it("a peak percent is read against 100, its average printed beside it", () => {
    const proof = proofOf(first("NAMESPACE_CPU_LIMIT_CRITICAL"));
    expect(proof && { from: proof.from, value: proof.value, reference: proof.reference }).toEqual({ from: 0, value: 330, reference: 100 });
    expect(names(proof)).toEqual({ lead: "peak_percent", rest: ["average_percent"] });
    expect(proof?.count).toBe(false);
  });

  it("a count is drawn in whole units, a percent is not", () => {
    expect(proofOf(first("HPA_MAX_REPLICAS_REACHED"))?.count).toBe(true);
    expect(proofOf(nearMax)?.count).toBe(true);
    expect(proofOf(first("NODE_CPU_REQUEST_HIGH"))?.count).toBe(false);
  });

  it("a share of the node is a percent read against 100", () => {
    const proof = proofOf(first("NODE_CPU_REQUEST_HIGH"));
    expect(proof?.reference).toBe(100);
    expect(names(proof)).toEqual({ lead: "percent", rest: [] });
  });

  it("the highest average of a node is read against 100", () => {
    expect(names(proofOf(first("NODE_LOW_UTILIZATION")))).toEqual({ lead: "maximum_average_percent", rest: [] });
  });

  it("the replicas of an HPA are read against its maximum", () => {
    const proof = proofOf(first("HPA_MAX_REPLICAS_REACHED"));
    expect(proof && { from: proof.from, value: proof.value, reference: proof.reference }).toEqual({ from: 0, value: 4, reference: 4 });
    expect(names(proof)).toEqual({ lead: "current_peak", rest: ["spec_max"] });
  });

  it("the nodes of the cluster are read against the autoscaler's maximum", () => {
    const proof = proofOf(nearMax);
    expect(proof && { value: proof.value, reference: proof.reference }).toEqual({ value: 3, reference: 3 });
    expect(names(proof)).toEqual({ lead: "current_nodes", rest: ["max_nodes"] });
  });

  it("a pinned HPA reaches nothing, so it draws nothing", () => {
    expect(proofOf(first("HPA_PINNED_REPLICAS"))).toBeNull();
  });

  it("an HPA near its maximum draws its average replicas against the maximum, where they fall", () => {
    const proof = proofOf({
      ...first("HPA_MAX_REPLICAS_REACHED"),
      code: "HPA_NEAR_MAX_REPLICAS",
      evidence: [
        { name: "current_average", raw: 3.5, text: "3.5" },
        { name: "spec_max", raw: 4, text: "4" },
      ],
    });
    expect(proof && { value: proof.value, reference: proof.reference, count: proof.count }).toEqual({ value: 3.5, reference: 4, count: true });
    expect(names(proof)).toEqual({ lead: "current_average", rest: ["spec_max"] });
  });

  it("an uneven pool draws its peaks as a range from the lowest, the spread printed after", () => {
    const proof = proofOf(first("NODEPOOL_CPU_UNEVEN"));
    expect(proof && { from: proof.from, value: proof.value, reference: proof.reference }).toEqual({ from: 20, value: 60, reference: 100 });
    expect(names(proof)).toEqual({ lead: "maximum_peak_percent", rest: ["minimum_peak_percent", "spread_percent"] });
  });

  it("evidence with nothing to draw against a reference gives no proof", () => {
    for (const code of ["DEPLOYMENT_WITHOUT_HPA", "HPA_REPLICA_DIVERGENCE", "NODE_OVERREQUESTED", "NODEPOOL_UNKNOWN", "NODE_NOT_READY"]) {
      expect(proofOf(first(code)), code).toBeNull();
    }
  });

  it("a maximum of none, or a value that is not a number, draws nothing", () => {
    const base = first("HPA_MAX_REPLICAS_REACHED");
    expect(
      proofOf({
        ...base,
        evidence: [
          { name: "current_peak", raw: 4, text: "4" },
          { name: "spec_max", raw: 0, text: "0" },
        ],
      }),
    ).toBeNull();
    expect(proofOf({ ...base, evidence: [{ name: "peak_percent", raw: "n/a", text: "n/a" }] })).toBeNull();
  });
});
