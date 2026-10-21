import type { components } from "@/api/schema.gen";
import type { Status } from "@/components/status-mark";
import { isActive } from "@/features/reports/api";

export type Schemas = components["schemas"];
export type HomeCluster = Schemas["HomeCluster"];

export type ChipLine =
  | { kind: "age"; at: string }
  | { kind: "step"; step: "queued" | "collecting" | "rendering" }
  | { kind: "failed" }
  | { kind: "expired" }
  | { kind: "none" };

type Step = Extract<ChipLine, { kind: "step" }>["step"];

function stepOf(newest: Schemas["ReportSummary"]): Step {
  if (newest.status === "queued") {
    return "queued";
  }
  return newest.status === "running" ? "collecting" : "rendering";
}

// The mark and the muted line of a cluster's chip.
export function chipOf(c: HomeCluster): { status: Status; line: ChipLine } {
  const { latest, newest } = c;
  let status: Status = "undefined";
  let line: ChipLine = { kind: "none" };
  if (latest !== null) {
    status = latest.worst_severity ?? "unavailable";
    line = { kind: "age", at: latest.completed_at ?? latest.created_at };
  } else if (newest !== null && newest.status === "collected" && newest.expired) {
    status = "unavailable";
    line = { kind: "expired" };
  }
  if (newest !== null && isActive(newest)) {
    line = { kind: "step", step: stepOf(newest) };
  } else if (newest !== null && newest.status === "failed" && latest === null) {
    line = { kind: "failed" };
  }
  return { status, line };
}

const rank: Record<Status, number> = { critical: 0, attention: 1, healthy: 2, unavailable: 3, undefined: 4 };

// Critical first, then attention, healthy, unavailable, and no base; ties by display name.
export function orderClusters(cs: HomeCluster[]): HomeCluster[] {
  return [...cs].sort(
    (a, b) => rank[chipOf(a).status] - rank[chipOf(b).status] || a.display_name.localeCompare(b.display_name),
  );
}
