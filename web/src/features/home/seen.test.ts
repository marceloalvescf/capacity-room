import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Schemas } from "./clusters";
import { markSeen, unseen } from "./seen";

type Cluster = Schemas["HomeCluster"];

function cluster(profile: string, reportId: string | null): Cluster {
  const latest = reportId === null ? null : ({ id: reportId } as Cluster["latest"]);
  return { profile_id: profile, display_name: profile, latest, newest: latest };
}

beforeEach(() => {
  window.localStorage.clear();
});
afterEach(() => {
  vi.restoreAllMocks();
});

describe("unseen", () => {
  it("records a first sight silently and reports seen", () => {
    expect(unseen("u1", cluster("p", "r1"))).toBe(false);
    expect(unseen("u1", cluster("p", "r1"))).toBe(false);
  });

  it("reports a changed latest report until it is marked seen", () => {
    unseen("u1", cluster("p", "r1"));
    expect(unseen("u1", cluster("p", "r2"))).toBe(true);
    markSeen("u1", "p", "r2");
    expect(unseen("u1", cluster("p", "r2"))).toBe(false);
  });

  it("keeps one map per user", () => {
    unseen("u1", cluster("p", "r1"));
    expect(unseen("u2", cluster("p", "r2"))).toBe(false);
    expect(unseen("u1", cluster("p", "r2"))).toBe(true);
  });

  it("has nothing to say about a cluster with no latest report", () => {
    expect(unseen("u1", cluster("p", null))).toBe(false);
  });

  it("reads as seen and throws nothing when storage is refused", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("denied");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("denied");
    });
    expect(unseen("u1", cluster("p", "r1"))).toBe(false);
    expect(unseen("u1", cluster("p", "r2"))).toBe(false);
    expect(() => {
      markSeen("u1", "p", "r2");
    }).not.toThrow();
  });
});
