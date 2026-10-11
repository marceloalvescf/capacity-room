import { describe, expect, it } from "vitest";
import type { components } from "@/api/schema.gen";
import viewEn from "../../../tests/fixtures/view/edge-scaling.en.json";
import directEn from "../../../tests/fixtures/view/direct-complete.en.json";
import {
  hpaKey,
  indexFindings,
  matchHPA,
  matchNamespace,
  matchNode,
  matchWorkload,
  namespaceKey,
  nodeKey,
  workloadKey,
} from "./pieces";

type View = components["schemas"]["ReportView"];
type Finding = View["findings"][number];

const order: readonly Finding["severity"][] = ["critical", "attention", "healthy"];

// What the matchers say of a row: the worst severity of its findings, or none.
function byMatcher(findings: readonly Finding[], match: (finding: Finding) => boolean): Finding["severity"] | undefined {
  const kept = findings.filter(match).map((finding) => finding.severity);
  return order.find((severity) => kept.includes(severity));
}

describe("finding index", () => {
  for (const [name, fixture] of [
    ["edge-scaling", viewEn],
    ["direct-complete", directEn],
  ] as const) {
    it(`gives each row of ${name} what the matchers give`, () => {
      const view = fixture as View;
      const index = indexFindings(view.findings);
      const rows: (readonly [string, Finding["severity"] | undefined])[] = [
        ...view.namespaces.map((row) => [namespaceKey(row), byMatcher(view.findings, matchNamespace(row))] as const),
        ...view.workloads.map((row) => [workloadKey(row), byMatcher(view.findings, matchWorkload(row))] as const),
        ...view.hpas.map((row) => [hpaKey(row), byMatcher(view.findings, matchHPA(row))] as const),
        ...view.nodes.map((row) => [nodeKey(row), byMatcher(view.findings, matchNode(row))] as const),
      ];
      expect(rows.length).toBeGreaterThan(0);
      expect(rows.some(([, worst]) => worst !== undefined)).toBe(true);
      for (const [key, worst] of rows) {
        expect({ key, worst: index.get(key), has: index.has(key) }).toEqual({ key, worst, has: worst !== undefined });
      }
    });
  }

  it("tells two workloads of one name apart by their kind", () => {
    const finding = (viewEn as View).findings.find((f) => f.scope_type === "workload") as Finding;
    const index = indexFindings([finding, { ...finding, scope_kind: "StatefulSet", severity: "healthy" }]);
    expect(index.size).toBe(2);
  });
});
