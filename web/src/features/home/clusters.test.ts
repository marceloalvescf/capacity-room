import { describe, expect, it } from "vitest";
import type { Schemas } from "./clusters";
import { chipOf, orderClusters } from "./clusters";

type Cluster = Schemas["HomeCluster"];
type Summary = Schemas["ReportSummary"];

function report(over: Partial<Summary>): Summary {
  return {
    id: "r1",
    status: "collected",
    phase: "complete",
    failure_code: null,
    pdfs: [],
    created_at: "2026-10-21T10:00:00Z",
    completed_at: "2026-10-21T10:05:00Z",
    expires_at: "2026-11-21T10:00:00Z",
    expired: false,
    selection: null,
    worst_severity: "healthy",
    cluster_name: "prod",
    ...over,
  } as Summary;
}

function cluster(name: string, latest: Summary | null, newest: Summary | null = latest): Cluster {
  return { profile_id: `p-${name}`, display_name: name, latest, newest };
}

describe("chipOf", () => {
  it("marks a live collected report with its severity and age", () => {
    const live = report({ worst_severity: "critical" });
    expect(chipOf(cluster("a", live))).toEqual({
      status: "critical",
      line: { kind: "age", at: "2026-10-21T10:05:00Z" },
    });
  });

  it("marks a report with no severity unavailable, keeping its age", () => {
    const live = report({ worst_severity: null });
    expect(chipOf(cluster("a", live)).status).toBe("unavailable");
    expect(chipOf(cluster("a", live)).line.kind).toBe("age");
  });

  it("marks only an expired report unavailable and expired", () => {
    const old = report({ expired: true });
    expect(chipOf(cluster("a", null, old))).toEqual({ status: "unavailable", line: { kind: "expired" } });
  });

  it("marks no report at all with the open circle", () => {
    expect(chipOf(cluster("a", null, null))).toEqual({ status: "undefined", line: { kind: "none" } });
  });

  it("shows the step of a running request over the live mark", () => {
    const live = report({ worst_severity: "attention" });
    const running = report({ id: "r2", status: "running", phase: "collecting", worst_severity: null });
    expect(chipOf(cluster("a", live, running))).toEqual({
      status: "attention",
      line: { kind: "step", step: "collecting" },
    });
  });

  it("maps queued and a pending PDF to their steps", () => {
    const live = report({});
    const queued = report({ id: "r2", status: "queued" });
    const rendering = report({ pdfs: [{ language: "en", status: "pending", failure_code: null }] });
    expect(chipOf(cluster("a", live, queued)).line).toEqual({ kind: "step", step: "queued" });
    expect(chipOf(cluster("a", rendering, rendering)).line).toEqual({ kind: "step", step: "rendering" });
  });

  it("says failed when the newest failed and nothing live was collected", () => {
    const failed = report({ id: "r2", status: "failed", failure_code: "INTERNAL_FAILURE" });
    expect(chipOf(cluster("a", null, failed)).line).toEqual({ kind: "failed" });
  });

  it("keeps the age when the newest failed over a live report", () => {
    const live = report({});
    const failed = report({ id: "r2", status: "failed", failure_code: "INTERNAL_FAILURE" });
    expect(chipOf(cluster("a", live, failed))).toEqual({
      status: "healthy",
      line: { kind: "age", at: "2026-10-21T10:05:00Z" },
    });
  });
});

describe("orderClusters", () => {
  it("orders by severity, then unavailable, then no base, then display name", () => {
    const crit = report({ worst_severity: "critical" });
    const ok = report({ worst_severity: "healthy" });
    const att = report({ worst_severity: "attention" });
    const unav = report({ worst_severity: null });
    const out = orderClusters([
      cluster("none", null, null),
      cluster("unav", unav),
      cluster("a-ok", ok),
      cluster("c-crit", crit),
      cluster("att", att),
      cluster("b-crit", crit),
    ]);
    expect(out.map((c) => c.display_name)).toEqual(["b-crit", "c-crit", "att", "a-ok", "unav", "none"]);
  });
});
