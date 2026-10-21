import type { ViewFinding } from "./api";

type Evidence = ViewFinding["evidence"][number];

// What the evidence of a finding draws: a value read against its reference from zero, or a range between two
// peaks; the evidence printed large beside it, then the rest of what it shows. Every number is the view's own: a
// percent is read against 100, a count against the maximum its evidence names, a range against 100. A count
// divides its track into units; an average count stays where it falls between them.
export type Peak = Evidence & { raw: number };
export type Proof =
  | { kind: "range"; low: Peak; high: Peak; spread: Evidence | null }
  | { kind: "bar"; value: number; reference: number; count: boolean; lead: Evidence; rest: Evidence[] };

// The names of the evidence a proof prints, so the list of the rest can leave them out.
export function provedNames(proof: Proof): string[] {
  return proof.kind === "range"
    ? [proof.high.name, proof.low.name, ...(proof.spread === null ? [] : [proof.spread.name])]
    : [proof.lead.name, ...proof.rest.map((item) => item.name)];
}

// The count a finding reaches, then the maximum it is read against. A pinned HPA, whose minimum equals its
// maximum, reaches nothing, so its range is not drawn.
const counts = [
  ["current_peak", "spec_max"],
  ["current_average", "spec_max"],
  ["current_nodes", "max_nodes"],
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
    return { kind: "range", low, high, spread: spread ?? null };
  }
  for (const [reached, maximum] of counts) {
    const value = item(reached);
    const reference = item(maximum);
    if (numeric(value) && numeric(reference) && reference.raw > 0) {
      return { kind: "bar", value: value.raw, reference: reference.raw, count: true, lead: value, rest: [reference] };
    }
  }
  for (const [name, beside] of percents) {
    const value = item(name);
    if (numeric(value)) {
      const after = beside === null ? undefined : item(beside);
      return { kind: "bar", value: value.raw, reference: 100, count: false, lead: value, rest: after === undefined ? [] : [after] };
    }
  }
  return null;
}
