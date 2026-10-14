import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiContext } from "@/api/context";
import { installMockApi, principal, report, signedIn } from "../../../tests/msw";
import { createApi } from "@/api/client";
import { canDelete, isActive, reportsKey, useReport } from "./api";

const mock = installMockApi(signedIn());

afterEach(() => {
  vi.useRealTimers();
});

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

  it("counts the checks of each report from its own first active answer", async () => {
    const a = "report-a";
    const b = "report-b";
    mock.reports = [report({ id: a, status: "running", phase: "collecting" }), report({ id: b, status: "running", phase: "collecting" })];
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } });
    const api = createApi(queryClient, vi.fn());
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>
        <ApiContext value={api}>{children}</ApiContext>
      </QueryClientProvider>
    );
    // Report b is cached with a long history of answers, so its own count is high.
    for (let answer = 0; answer < 25; answer++) {
      queryClient.setQueryData([...reportsKey, b], report({ id: b, status: "running", phase: "collecting" }));
    }
    const view = renderHook(({ id }) => useReport(id), { wrapper, initialProps: { id: a } });
    await waitFor(() => {
      expect(view.result.current.data?.id).toBe(a);
    });
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout", "setInterval", "clearInterval", "Date"] });
    const checksOfB = () => mock.calls.filter((call) => call.method === "GET" && call.path === `/reports/${b}`).length;
    view.rerender({ id: b });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1499);
    });
    expect(checksOfB()).toBe(0);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(checksOfB()).toBe(1);
  });
});
