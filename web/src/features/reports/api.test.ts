import { describe, expect, it, vi } from "vitest";
import { QueryClient } from "@tanstack/react-query";
import { installMockApi, principal, report, signedIn } from "../../../tests/msw";
import { createApi } from "@/api/client";
import { canDelete, isActive } from "./api";

const mock = installMockApi(signedIn());

describe("reports api", () => {
  it("can delete", () => {
    const own = { owner_id: principal().id };
    const other = { owner_id: "someone-else" };
    expect(canDelete(principal({ role: "administrator" }), other)).toBe(true);
    expect(canDelete(principal({ role: "operator" }), own)).toBe(true);
    expect(canDelete(principal({ role: "operator" }), other)).toBe(false);
    expect(canDelete(principal({ role: "reader" }), own)).toBe(false);
  });

  it("is active while queued, running, or a PDF is pending", () => {
    expect(isActive({ status: "queued", pdfs: [] })).toBe(true);
    expect(isActive({ status: "running", pdfs: [] })).toBe(true);
    expect(isActive({ status: "collected", pdfs: [{ language: "en", status: "pending", failure_code: null }] })).toBe(
      true,
    );
    expect(isActive({ status: "collected", pdfs: [{ language: "en", status: "ready", failure_code: null }] })).toBe(
      false,
    );
    expect(isActive({ status: "failed", pdfs: [] })).toBe(false);
  });

  it("the mock serves the reports in pages", async () => {
    mock.reports = [report({ id: "a" }), report({ id: "b" }), report({ id: "c" })];
    const api = createApi(new QueryClient(), vi.fn());
    const first = await api.GET("/reports", { params: { query: { limit: 2 } } });
    expect(first.data?.items.map((r) => r.id)).toEqual(["a", "b"]);
    const second = await api.GET("/reports", {
      params: { query: { limit: 2, cursor: first.data?.next_cursor ?? "" } },
    });
    expect(second.data?.items.map((r) => r.id)).toEqual(["c"]);
    expect(second.data?.next_cursor).toBeNull();
  });
});
