import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { components } from "@/api/schema.gen";
import { formatTime } from "@/lib/format";
import { i18next } from "@/i18n/i18n";
import { installMockApi, profileSummary, report, server, signedIn } from "../../../tests/msw";
import { renderApp } from "../../../tests/render";
import { mockColourScheme } from "../../../tests/match-media";
import { reportsKey } from "./api";

const mock = installMockApi(signedIn());

type Role = components["schemas"]["Role"];

const other = "9a9a9a9a-0000-4000-8000-000000000001";

function id(n: number): string {
  return `${String(n).padStart(8, "0")}-0000-4000-8000-000000000000`;
}

function many(count: number, overrides: (n: number) => Partial<ReturnType<typeof report>> = () => ({})) {
  return Array.from({ length: count }, (_, n) =>
    report({
      id: id(n),
      created_at: new Date(Date.UTC(2026, 9, 7, 17, 0) - n * 60_000).toISOString(),
      ...overrides(n),
    }),
  );
}

function signIn(role: Role) {
  mock.session = signedIn({ role });
}

function rowOf(reportId: string): HTMLElement {
  const created = mock.reports.find((r) => r.id === reportId)?.created_at ?? "";
  const cell = screen.getByText(formatTime(created));
  return cell.closest("tr") as HTMLElement;
}

function listCalls() {
  return mock.calls.filter((call) => call.method === "GET" && call.path === "/reports");
}

const t = (key: string, options: Record<string, string | number> = {}): string => i18next.t(key, options);

beforeEach(() => {
  window.localStorage.clear();
  mockColourScheme(false);
  mock.profiles = [profileSummary()];
});

afterEach(() => {
  vi.useRealTimers();
});

describe("report list", () => {
  it("newest first, load more", async () => {
    mock.reports = many(60);
    const user = userEvent.setup();
    await renderApp("/reports");
    await screen.findByRole("heading", { level: 1, name: t("reports.title") });
    await waitFor(() => {
      expect(screen.getAllByRole("row")).toHaveLength(51);
    });
    const requested = screen
      .getAllByRole("row")
      .slice(1)
      .map((row) => within(row).getAllByRole("cell")[1]?.textContent);
    expect(requested[0]).toBe(formatTime(mock.reports[0]?.created_at ?? ""));
    expect(requested[49]).toBe(formatTime(mock.reports[49]?.created_at ?? ""));
    await user.click(screen.getByRole("button", { name: t("reports.loadMore") }));
    await waitFor(() => {
      expect(screen.getAllByRole("row")).toHaveLength(61);
    });
    expect(screen.queryByRole("button", { name: t("reports.loadMore") })).not.toBeInTheDocument();
  });

  it("a workflow mark reads its own word and no severity", async () => {
    mock.reports = [report({ pdfs: [{ language: "en", status: "ready", failure_code: null }] })];
    await renderApp("/reports");
    await screen.findByRole("heading", { level: 1, name: t("reports.title") });
    const row = await waitFor(() => rowOf(mock.reports[0]?.id ?? ""));
    expect(row).toHaveTextContent(t("reportStatus.collected"));
    expect(row).not.toHaveTextContent(t("status.healthy"));
    expect(row).not.toHaveTextContent(t("status.attention"));
  });

  it("polling keeps the loaded pages", async () => {
    mock.reports = many(60, (n) => (n === 3 ? { status: "running", phase: "collecting", completed_at: null } : {}));
    const user = userEvent.setup({
      advanceTimers: vi.advanceTimersByTimeAsync,
    });
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { queryClient } = await renderApp("/reports");
    await waitFor(() => {
      expect(screen.getAllByRole("row")).toHaveLength(51);
    });
    expect(screen.getByText(t("reports.updating"))).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: t("reports.loadMore") }));
    await waitFor(() => {
      expect(screen.getAllByRole("row")).toHaveLength(61);
    });
    expect(within(rowOf(id(3))).getByText(t("reportStatus.running"))).toBeInTheDocument();

    await act(async () => {
      await queryClient.refetchQueries({ queryKey: reportsKey });
    });
    mock.reports = many(60);
    mock.calls.length = 0;
    await act(async () => {
      await vi.advanceTimersByTimeAsync(4000);
    });
    expect(listCalls()).toEqual([]);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1100);
    });
    await waitFor(() => {
      expect(listCalls()).toHaveLength(2);
    });
    await waitFor(() => {
      expect(within(rowOf(id(3))).getByText(t("reportStatus.collected"))).toBeInTheDocument();
    });
    const rows = screen.getAllByRole("row");
    expect(rows).toHaveLength(61);
    const times = rows.slice(1).map((row) => within(row).getAllByRole("cell")[1]?.textContent);
    expect(new Set(times).size).toBe(60);
    expect(screen.queryByText(t("reports.updating"))).not.toBeInTheDocument();

    mock.calls.length = 0;
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10_000);
    });
    expect(mock.calls.filter((call) => call.path === "/reports")).toEqual([]);
  });

  it("a pending PDF keeps the checking until it is ready", async () => {
    const pending = (status: "pending" | "ready") => [{ language: "en" as const, status, failure_code: null }];
    mock.reports = many(2, (n) => (n === 0 ? { pdfs: pending("pending") } : {}));
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { queryClient } = await renderApp("/reports");
    await screen.findByText(t("reports.updating"));
    await act(async () => {
      await queryClient.refetchQueries({ queryKey: reportsKey });
    });
    mock.calls.length = 0;
    await act(async () => {
      await vi.advanceTimersByTimeAsync(4000);
    });
    expect(listCalls()).toEqual([]);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1100);
    });
    await waitFor(() => {
      expect(listCalls()).toHaveLength(1);
    });
    mock.reports = many(2, (n) => (n === 0 ? { pdfs: pending("ready") } : {}));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5000);
    });
    await waitFor(() => {
      expect(screen.queryByText(t("reports.updating"))).not.toBeInTheDocument();
    });
    mock.calls.length = 0;
    await act(async () => {
      await vi.advanceTimersByTimeAsync(20_000);
    });
    expect(listCalls()).toEqual([]);
  });

  it("a reader", async () => {
    signIn("reader");
    mock.reports = many(2);
    await renderApp("/reports");
    await screen.findAllByRole("row");
    expect(screen.getAllByRole("row")).toHaveLength(3);
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: t("reports.delete") })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: t("nav.generate") })).not.toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: t("reports.open") })).toHaveLength(2);
  });

  it("an operator deletes only their own", async () => {
    signIn("operator");
    mock.reports = many(3, (n) => (n === 1 ? { owner_id: other, owner_username: "bia" } : {}));
    await renderApp("/reports");
    await screen.findAllByRole("row");
    expect(within(rowOf(id(0))).getByRole("checkbox")).toBeInTheDocument();
    expect(within(rowOf(id(0))).getByRole("button", { name: t("reports.delete") })).toBeInTheDocument();
    expect(within(rowOf(id(1))).queryByRole("checkbox")).not.toBeInTheDocument();
    expect(within(rowOf(id(1))).queryByRole("button", { name: t("reports.delete") })).not.toBeInTheDocument();
    expect(within(rowOf(id(2))).getByRole("checkbox")).toBeInTheDocument();
    expect(within(screen.getByRole("main")).getByRole("link", { name: t("nav.generate") })).toBeInTheDocument();
  });

  it("delete one", async () => {
    mock.reports = many(3, (n) => (n === 1 ? { owner_username: "bia" } : {}));
    const user = userEvent.setup();
    const { queryClient } = await renderApp("/reports");
    await screen.findAllByRole("row");
    queryClient.setQueryData([...reportsKey, id(1)], mock.reports[1]);
    mock.calls.length = 0;
    await user.click(within(rowOf(id(1))).getByRole("button", { name: t("reports.delete") }));
    const dialog = await screen.findByRole("alertdialog");
    expect(within(dialog).getByText("Delete the report on Production?")).toBeInTheDocument();
    expect(
      within(dialog).getByText("The report requested 2026-10-07 16:59 by bia, its data, and its PDFs are removed. This cannot be undone."),
    ).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: t("delete.confirm") }));
    await waitFor(() => {
      expect(screen.getAllByRole("row")).toHaveLength(3);
    });
    expect(mock.calls.filter((call) => call.method === "DELETE").map((call) => call.path)).toEqual([
      `/reports/${id(1)}`,
    ]);
    expect(mock.calls.filter((call) => call.path === `/reports/${id(1)}` && call.method === "GET")).toEqual([]);
    expect(queryClient.getQueryData([...reportsKey, id(1)])).toBeUndefined();
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("bulk delete", async () => {
    mock.reports = many(4);
    let sent: string[] = [];
    server.use(
      http.post(`${window.location.origin}/api/v1/reports/delete`, async ({ request }) => {
        const body = (await request.json()) as { ids: string[] };
        sent = body.ids;
        const outcomes = ["deleted", "not_found", "forbidden"] as const;
        return HttpResponse.json({
          results: body.ids.map((one, n) => ({
            id: one,
            outcome: outcomes[n],
          })),
        });
      }),
    );
    const user = userEvent.setup();
    await renderApp("/reports");
    await screen.findAllByRole("row");
    for (const n of [0, 1, 2]) {
      await user.click(within(rowOf(id(n))).getByRole("checkbox"));
    }
    expect(screen.getByText(t("reports.selected", { count: 3 }))).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: t("reports.deleteSelected") }));
    const dialog = await screen.findByRole("alertdialog");
    expect(within(dialog).getByText(t("delete.manyTitle", { count: 3 }))).toBeInTheDocument();
    mock.calls.length = 0;
    await user.click(within(dialog).getByRole("button", { name: t("delete.confirm") }));
    expect(await screen.findByText(t("reports.deleteForbidden"))).toBeInTheDocument();
    expect(sent).toEqual([id(0), id(1), id(2)]);
    expect(screen.getByText(t("reports.selected", { count: 1 }))).toBeInTheDocument();
    expect(within(rowOf(id(2))).getByRole("checkbox")).toBeChecked();
    expect(within(rowOf(id(3))).getByRole("checkbox")).not.toBeChecked();
  });

  it("bulk delete through the mock removes the rows and reloads", async () => {
    mock.reports = many(4);
    const user = userEvent.setup();
    await renderApp("/reports");
    await screen.findAllByRole("row");
    await user.click(screen.getByRole("checkbox", { name: t("reports.selectAll") }));
    expect(screen.getByText(t("reports.selected", { count: 4 }))).toBeInTheDocument();
    await user.click(within(rowOf(id(3))).getByRole("checkbox"));
    await user.click(screen.getByRole("button", { name: t("reports.deleteSelected") }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: t("delete.confirm") }));
    await waitFor(() => {
      expect(screen.getAllByRole("row")).toHaveLength(2);
    });
    expect(mock.reports.map((r) => r.id)).toEqual([id(3)]);
    expect(screen.queryByText(t("reports.deleteForbidden"))).not.toBeInTheDocument();
  });

  it("bulk of one", async () => {
    mock.reports = many(2);
    const user = userEvent.setup();
    await renderApp("/reports");
    await screen.findAllByRole("row");
    await user.click(within(rowOf(id(0))).getByRole("checkbox"));
    await user.click(screen.getByRole("button", { name: t("reports.deleteSelected") }));
    const dialog = await screen.findByRole("alertdialog");
    expect(within(dialog).getByText("Delete the report on Production?")).toBeInTheDocument();
  });

  it("cancel keeps everything", async () => {
    mock.reports = many(2);
    const user = userEvent.setup();
    await renderApp("/reports");
    await screen.findAllByRole("row");
    await user.click(within(rowOf(id(0))).getByRole("checkbox"));
    mock.calls.length = 0;
    await user.click(screen.getByRole("button", { name: t("reports.deleteSelected") }));
    await user.click(
      within(await screen.findByRole("alertdialog")).getByRole("button", {
        name: t("delete.cancel"),
      }),
    );
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    expect(mock.calls).toEqual([]);
    expect(within(rowOf(id(0))).getByRole("checkbox")).toBeChecked();
    expect(screen.getByText(t("reports.selected", { count: 1 }))).toBeInTheDocument();
  });

  it("select all stays indeterminate after a click that leaves the selection partial", async () => {
    mock.reports = many(3);
    const user = userEvent.setup();
    await renderApp("/reports");
    await screen.findAllByRole("row");
    await user.click(within(rowOf(id(0))).getByRole("checkbox"));
    const all = screen.getByRole<HTMLInputElement>("checkbox", { name: t("reports.selectAll") });
    expect(all.indeterminate).toBe(true);
    await user.click(within(rowOf(id(1))).getByRole("checkbox"));
    expect(all.indeterminate).toBe(true);
    await user.click(all);
    expect(all.indeterminate).toBe(false);
    expect(all).toBeChecked();
    await user.click(all);
    expect(all).not.toBeChecked();
    expect(screen.queryByText(t("reports.selected", { count: 0 }))).not.toBeInTheDocument();
  });

  it("expired row", async () => {
    mock.reports = many(1, () => ({ expired: true }));
    await renderApp("/reports");
    await screen.findAllByRole("row");
    const row = rowOf(id(0));
    expect(within(row).getByText(t("reports.expired"))).toBeInTheDocument();
    expect(within(row).queryByRole("link", { name: t("reports.open") })).not.toBeInTheDocument();
    expect(within(row).getByRole("button", { name: t("reports.delete") })).toBeInTheDocument();
  });

  it("the interface language's PDF", async () => {
    mock.reports = many(3, (n) => ({
      pdfs: [
        {
          language: "en",
          status: n === 0 ? "ready" : "pending",
          failure_code: null,
        },
        {
          language: "pt-BR",
          status: "failed",
          failure_code: "PDF_COMPILE_FAILED",
        },
      ],
    }));
    mock.reports[2] = report({
      id: id(2),
      created_at: many(3)[2]?.created_at ?? "",
      pdfs: [{ language: "pt-BR", status: "ready", failure_code: null }],
    });
    mock.reports[1] = report({
      id: id(1),
      created_at: many(3)[1]?.created_at ?? "",
      status: "queued",
      phase: "waiting",
      completed_at: null,
    });
    await renderApp("/reports");
    await screen.findAllByRole("row");
    expect(within(rowOf(id(0))).getByText(t("pdfStatus.ready"))).toBeInTheDocument();
    expect(within(rowOf(id(2))).getByText(t("reports.notGenerated"))).toBeInTheDocument();
    expect(within(rowOf(id(1))).queryByText(t("reports.notGenerated"))).not.toBeInTheDocument();
  });

  it("the profile's id when it is not listed", async () => {
    mock.reports = many(1, () => ({ profile_id: "gone" }));
    await renderApp("/reports");
    await screen.findAllByRole("row");
    expect(within(rowOf(id(0))).getByText("gone")).toBeInTheDocument();
  });

  it("empty", async () => {
    signIn("operator");
    await renderApp("/reports");
    expect(await screen.findByText(t("reports.empty"))).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: t("nav.generate") }).length).toBeGreaterThan(0);
  });

  it("a failed load offers a retry", async () => {
    const url = `${window.location.origin}/api/v1/reports`;
    let fail = true;
    server.use(
      http.get(url, () =>
        fail
          ? HttpResponse.json({ error: { code: "INTERNAL_ERROR", message: "m", fields: {} } }, { status: 500 })
          : HttpResponse.json({ items: many(1), next_cursor: null }),
      ),
    );
    const user = userEvent.setup();
    await renderApp("/reports");
    expect(await screen.findByText(t("reports.loadFailed"))).toBeInTheDocument();
    fail = false;
    await user.click(screen.getByRole("button", { name: t("reports.retry") }));
    await screen.findAllByRole("row");
    expect(screen.queryByText(t("reports.loadFailed"))).not.toBeInTheDocument();
  });

  it("a delete the server refuses as gone still reloads the list", async () => {
    mock.reports = many(2);
    server.use(
      http.delete(`${window.location.origin}/api/v1/reports/${id(1)}`, () => {
        mock.reports = mock.reports.filter((r) => r.id !== id(1));
        return HttpResponse.json({ error: { code: "REPORT_NOT_FOUND", message: "m", fields: {} } }, { status: 404 });
      }),
    );
    const user = userEvent.setup();
    await renderApp("/reports");
    await screen.findAllByRole("row");
    await user.click(within(rowOf(id(1))).getByRole("button", { name: t("reports.delete") }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: t("delete.confirm") }));
    await waitFor(() => {
      expect(screen.getAllByRole("row")).toHaveLength(2);
    });
  });

  it("after deleting one, focus is on the heading and the outcome is announced", async () => {
    mock.reports = many(2);
    const user = userEvent.setup();
    await renderApp("/reports");
    await screen.findAllByRole("row");
    await user.click(within(rowOf(id(0))).getByRole("button", { name: t("reports.delete") }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: t("delete.confirm") }));
    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1, name: t("reports.title") })).toHaveFocus();
    });
    expect(await screen.findByText(t("reports.deleted", { count: 1 }))).toBeInTheDocument();
  });

  it("after a bulk delete, focus is on the heading and the count that went is announced", async () => {
    mock.reports = many(4);
    server.use(
      http.post(`${window.location.origin}/api/v1/reports/delete`, async ({ request }) => {
        const body = (await request.json()) as { ids: string[] };
        const outcomes = ["deleted", "not_found", "forbidden"] as const;
        return HttpResponse.json({ results: body.ids.map((one, n) => ({ id: one, outcome: outcomes[n] })) });
      }),
    );
    const user = userEvent.setup();
    await renderApp("/reports");
    await screen.findAllByRole("row");
    for (const n of [0, 1, 2]) {
      await user.click(within(rowOf(id(n))).getByRole("checkbox"));
    }
    await user.click(screen.getByRole("button", { name: t("reports.deleteSelected") }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: t("delete.confirm") }));
    expect(await screen.findByText(t("reports.deleted", { count: 2 }))).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1, name: t("reports.title") })).toHaveFocus();
    });
  });

  it("the bulk dialog sends the ids it counted, never an empty list", async () => {
    mock.reports = many(3);
    const user = userEvent.setup();
    const { queryClient } = await renderApp("/reports");
    await screen.findAllByRole("row");
    await user.click(within(rowOf(id(0))).getByRole("checkbox"));
    await user.click(within(rowOf(id(1))).getByRole("checkbox"));
    await user.click(screen.getByRole("button", { name: t("reports.deleteSelected") }));
    const dialog = await screen.findByRole("alertdialog");
    expect(within(dialog).getByText(t("delete.manyTitle", { count: 2 }))).toBeInTheDocument();
    const gone = mock.reports;
    mock.reports = [];
    await act(async () => {
      await queryClient.invalidateQueries({ queryKey: reportsKey });
    });
    mock.reports = gone;
    mock.calls.length = 0;
    await user.click(within(dialog).getByRole("button", { name: t("delete.confirm") }));
    await waitFor(() => {
      expect(mock.calls.some((call) => call.method === "POST" && call.path === "/reports/delete")).toBe(true);
    });
    const post = mock.calls.find((call) => call.method === "POST" && call.path === "/reports/delete");
    expect(post?.body).toEqual({ ids: [id(0), id(1)] });
  });

  it("an expired row can be deleted, one at a time and in a bulk", async () => {
    mock.reports = many(3, () => ({ expired: true }));
    const user = userEvent.setup();
    await renderApp("/reports");
    await screen.findAllByRole("row");
    await user.click(within(rowOf(id(0))).getByRole("button", { name: t("reports.delete") }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: t("delete.confirm") }));
    expect(await screen.findByText(t("reports.deleted", { count: 1 }))).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getAllByRole("row")).toHaveLength(3);
    });
    await user.click(screen.getByRole("checkbox", { name: t("reports.selectAll") }));
    await user.click(screen.getByRole("button", { name: t("reports.deleteSelected") }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: t("delete.confirm") }));
    expect(await screen.findByText(t("reports.deleted", { count: 2 }))).toBeInTheDocument();
    expect(mock.reports).toEqual([]);
  });

  it("a bulk delete over 200 ids goes in chunks of 200, one request after the other", async () => {
    mock.reports = many(230);
    const sent: number[] = [];
    let running = 0;
    let overlapped = false;
    server.use(
      http.post(`${window.location.origin}/api/v1/reports/delete`, async ({ request }) => {
        const body = (await request.json()) as { ids: string[] };
        sent.push(body.ids.length);
        running += 1;
        overlapped ||= running > 1;
        await new Promise((resolve) => setTimeout(resolve, 20));
        running -= 1;
        return HttpResponse.json({ results: body.ids.map((one) => ({ id: one, outcome: "deleted" })) });
      }),
    );
    const user = userEvent.setup();
    await renderApp("/reports");
    await screen.findAllByRole("row");
    for (let page = 1; page < 5; page += 1) {
      await user.click(await screen.findByRole("button", { name: t("reports.loadMore") }));
      await waitFor(() => {
        expect(document.querySelectorAll("tbody tr")).toHaveLength(Math.min(50 * (page + 1), 230));
      });
    }
    await user.click(screen.getByRole("checkbox", { name: t("reports.selectAll") }));
    await user.click(screen.getByRole("button", { name: t("reports.deleteSelected") }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: t("delete.confirm") }));
    await waitFor(() => {
      expect(within(dialog).getByRole("button", { name: t("delete.cancel") })).toBeDisabled();
    });
    expect(await screen.findByText(t("reports.deleted", { count: 230 }))).toBeInTheDocument();
    expect(sent).toEqual([200, 30]);
    expect(overlapped).toBe(false);
    // Renders 230 rows with a checkbox each, so every role query walks hundreds of nodes.
  }, 15_000);

  it("a double click on confirm sends one request", async () => {
    mock.reports = many(3);
    let posts = 0;
    server.use(
      http.post(`${window.location.origin}/api/v1/reports/delete`, async ({ request }) => {
        posts += 1;
        const body = (await request.json()) as { ids: string[] };
        await new Promise((resolve) => setTimeout(resolve, 150));
        return HttpResponse.json({ results: body.ids.map((one) => ({ id: one, outcome: "deleted" })) });
      }),
    );
    const user = userEvent.setup();
    await renderApp("/reports");
    await screen.findAllByRole("row");
    await user.click(within(rowOf(id(0))).getByRole("checkbox"));
    await user.click(within(rowOf(id(1))).getByRole("checkbox"));
    await user.click(screen.getByRole("button", { name: t("reports.deleteSelected") }));
    const confirm = within(await screen.findByRole("alertdialog")).getByRole("button", { name: t("delete.confirm") });
    await user.dblClick(confirm);
    await waitFor(() => {
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });
    expect(posts).toBe(1);
  });

  it("the selection keeps only the rows still shown", async () => {
    mock.reports = many(3);
    const user = userEvent.setup();
    const { queryClient } = await renderApp("/reports");
    await screen.findAllByRole("row");
    for (const n of [0, 1, 2]) {
      await user.click(within(rowOf(id(n))).getByRole("checkbox"));
    }
    const all = mock.reports;
    mock.reports = all.filter((r) => r.id !== id(1));
    await act(async () => {
      await queryClient.invalidateQueries({ queryKey: reportsKey });
    });
    expect(await screen.findByText(t("reports.selected", { count: 2 }))).toBeInTheDocument();
    mock.reports = all;
    await act(async () => {
      await queryClient.invalidateQueries({ queryKey: reportsKey });
    });
    await waitFor(() => {
      expect(screen.getAllByRole("row")).toHaveLength(4);
    });
    expect(within(rowOf(id(1))).getByRole("checkbox")).not.toBeChecked();
    expect(screen.getByText(t("reports.selected", { count: 2 }))).toBeInTheDocument();
  });

  it("a failed check keeps the rows and shows an alert above them", async () => {
    mock.reports = many(2);
    const { queryClient } = await renderApp("/reports");
    await screen.findAllByRole("row");
    server.use(
      http.get(`${window.location.origin}/api/v1/reports`, () =>
        HttpResponse.json({ error: { code: "INTERNAL_ERROR", message: "m", fields: {} } }, { status: 500 }),
      ),
    );
    await act(async () => {
      await queryClient.invalidateQueries({ queryKey: reportsKey });
    });
    expect(await screen.findByText(t("reports.loadFailed"))).toBeInTheDocument();
    expect(screen.getAllByRole("row")).toHaveLength(3);
  });

  it("a delete at work cannot be dismissed, and the next cancel returns focus to its trigger", async () => {
    mock.reports = many(3);
    let release = (): void => undefined;
    server.use(
      http.delete(`${window.location.origin}/api/v1/reports/${id(0)}`, async () => {
        await new Promise<void>((resolve) => {
          release = resolve;
        });
        mock.reports = mock.reports.filter((r) => r.id !== id(0));
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const user = userEvent.setup();
    await renderApp("/reports");
    await screen.findAllByRole("row");
    await user.click(within(rowOf(id(0))).getByRole("button", { name: t("reports.delete") }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: t("delete.confirm") }));
    await waitFor(() => {
      expect(within(dialog).getByRole("button", { name: t("delete.cancel") })).toBeDisabled();
    });
    await user.keyboard("{Escape}");
    expect(screen.getByRole("alertdialog")).toBeInTheDocument();
    release();
    await waitFor(() => {
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });
    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1, name: t("reports.title") })).toHaveFocus();
    });
    const trigger = within(rowOf(id(1))).getByRole("button", { name: t("reports.delete") });
    await user.click(trigger);
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: t("delete.cancel") }));
    await waitFor(() => {
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });
    await waitFor(() => {
      expect(trigger).toHaveFocus();
    });
  });

  it("a single delete answering 404 counts as gone", async () => {
    mock.reports = many(2);
    server.use(
      http.delete(`${window.location.origin}/api/v1/reports/${id(1)}`, () => {
        mock.reports = mock.reports.filter((r) => r.id !== id(1));
        return HttpResponse.json({ error: { code: "REPORT_NOT_FOUND", message: "m", fields: {} } }, { status: 404 });
      }),
    );
    const user = userEvent.setup();
    await renderApp("/reports");
    await screen.findAllByRole("row");
    await user.click(within(rowOf(id(1))).getByRole("button", { name: t("reports.delete") }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: t("delete.confirm") }));
    expect(await screen.findByText(t("reports.deleted", { count: 1 }))).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1, name: t("reports.title") })).toHaveFocus();
    });
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("another failure of a single delete shows the error", async () => {
    mock.reports = many(2);
    server.use(
      http.delete(`${window.location.origin}/api/v1/reports/${id(1)}`, () =>
        HttpResponse.json({ error: { code: "FORBIDDEN", message: "m", fields: {} } }, { status: 403 }),
      ),
    );
    const user = userEvent.setup();
    await renderApp("/reports");
    await screen.findAllByRole("row");
    await user.click(within(rowOf(id(1))).getByRole("button", { name: t("reports.delete") }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: t("delete.confirm") }));
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.queryByText(t("reports.deleted", { count: 1 }))).not.toBeInTheDocument();
  });

  it("an operator whose shown rows are all someone else's has no select column", async () => {
    signIn("operator");
    mock.reports = many(2, () => ({ owner_id: other, owner_username: "bia" }));
    await renderApp("/reports");
    await screen.findAllByRole("row");
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    const headers = screen.getAllByRole("columnheader");
    expect(headers).toHaveLength(6);
    expect(headers.every((header) => header.textContent !== "")).toBe(true);
    expect(within(rowOf(id(0))).getAllByRole("cell")).toHaveLength(6);
  });
});

describe("date cells", () => {
  it("never break inside a date", async () => {
    mock.reports = [report()];
    await renderApp("/reports");
    await screen.findByRole("heading", { level: 1, name: t("reports.title") });
    const row = await waitFor(() => rowOf(mock.reports[0]?.id ?? ""));
    const cells = within(row).getAllByRole("cell");
    const dated = cells.filter((cell) => /^\d{4}-\d{2}-\d{2}( \d{2}:\d{2})?$/.test(cell.textContent));
    expect(dated.length).toBeGreaterThanOrEqual(2);
    for (const cell of dated) {
      expect(cell).toHaveClass("whitespace-nowrap");
    }
  });
});

describe("report scope", () => {
  const selection = (fields: Partial<ReturnType<typeof report>["selection"]>) => ({
    ...report().selection,
    ...fields,
  });

  it("names the namespaces and the period of a length report", async () => {
    mock.reports = many(1, () => ({ selection: selection({ namespaces: ["payments", "billing"], period: "24h" }) }));
    await renderApp("/reports");
    expect(await screen.findByText("payments, billing")).toBeInTheDocument();
    const cell = screen.getByText("payments, billing").closest("td") as HTMLElement;
    expect(cell).toHaveTextContent("payments, billing · Last 24 hours");
  });

  it("names the range of a range report", async () => {
    mock.reports = many(1, () => ({
      selection: selection({ namespaces: ["payments"], period: null, start: "2026-10-01 00:00", end: "2026-10-02 00:00" }),
    }));
    await renderApp("/reports");
    const cell = (await screen.findByText("payments")).closest("td") as HTMLElement;
    expect(cell).toHaveTextContent("payments · 2026-10-01 00:00 to 2026-10-02 00:00");
  });

  it("shows no second line for a report whose request could not be read", async () => {
    mock.reports = many(1, () => ({ profile_id: "gone" })).map((row) => ({ ...row, selection: null }));
    await renderApp("/reports");
    await screen.findAllByRole("row");
    const cell = within(rowOf(id(0))).getByText("gone").closest("td") as HTMLElement;
    expect(cell.children).toHaveLength(1);
  });

  it("counts the namespaces past the third", async () => {
    mock.reports = many(1, () => ({ selection: selection({ namespaces: ["a", "b", "c", "d", "e"], period: "7d" }) }));
    await renderApp("/reports");
    const cell = (await screen.findByText("a, b, c")).closest("td") as HTMLElement;
    expect(cell).toHaveTextContent("a, b, c +2 · Last 7 days");
  });
});
