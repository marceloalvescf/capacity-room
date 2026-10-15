import type { ViewFinding } from "./api";

type Evidence = ViewFinding["evidence"][number];

// What the evidence of a finding draws: a value read against its reference, from zero or from the start of a
// range; the evidence printed large beside it, then the rest of what it shows. Every number is the view's own: a
// percent is read against 100, a count against the maximum its evidence names, a range against 100 from its
// lowest value. A count is drawn in whole units.
export interface Proof {
  from: number;
  value: number;
  reference: number;
  count: boolean;
  lead: Evidence;
  rest: Evidence[];
}

// The count a finding reaches, then the maximum it is read against.
const counts = [
  ["current_peak", "spec_max"],
  ["current_nodes", "max_nodes"],
  ["spec_min", "spec_max"],
] as const;

// A percent drawn alone, then the evidence printed after it.
const percents = [
  ["peak_percent", "average_percent"],
  ["percent", null],
  ["maximum_average_percent", null],
] as const;

function numeric(item: Evidence | undefined): item is Evidence & { raw: number } {
  return item !== undefined && typeof item.raw === "number" && Number.isFinite(item.raw);
}

// The proof of a finding, or null when its evidence holds no number to draw against a reference.
export function proofOf(finding: ViewFinding): Proof | null {
  const item = (name: string) => finding.evidence.find((evidence) => evidence.name === name);
  const low = item("minimum_peak_percent");
  const high = item("maximum_peak_percent");
  if (numeric(low) && numeric(high)) {
    const spread = item("spread_percent");
    return { from: low.raw, value: high.raw, reference: 100, count: false, lead: high, rest: spread === undefined ? [low] : [low, spread] };
  }
  for (const [reached, maximum] of counts) {
    const value = item(reached);
    const reference = item(maximum);
    if (numeric(value) && numeric(reference) && reference.raw > 0) {
      return { from: 0, value: value.raw, reference: reference.raw, count: true, lead: value, rest: [reference] };
    }
  }
  for (const [name, beside] of percents) {
    const value = item(name);
    if (numeric(value)) {
      const after = beside === null ? undefined : item(beside);
      return { from: 0, value: value.raw, reference: 100, count: false, lead: value, rest: after === undefined ? [] : [after] };
    }
  }
  return null;
}
