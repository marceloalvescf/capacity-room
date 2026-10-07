import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http } from "msw";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { components } from "@/api/schema.gen";
import { i18next } from "@/i18n/i18n";
import { failure, installMockApi, profileSummary, report, server, signedIn } from "../../../tests/msw";
import { location, renderApp } from "../../../tests/render";
import { mockColourScheme } from "../../../tests/match-media";

const mock = installMockApi(signedIn());

type Role = components["schemas"]["Role"];
type Pdf = components["schemas"]["ReportPDFState"];

const t = (key: string, options: Record<string, string | number> = {}): string => i18next.t(key, options);
const id = "0b6c3b52-2f0e-4d0a-8d57-6f6f6c1f0a01";

const running = { status: "running", phase: "collecting", completed_at: null, collected_at: null } as const;

function pdf(language: Pdf["language"], status: Pdf["status"], code: Pdf["failure_code"] = null): Pdf {
  return { language, status, failure_code: code };
}

function signIn(role: Role) {
  mock.session = signedIn({ role });
}

function checks() {
  return mock.calls.filter((call) => call.method === "GET" && call.path === `/reports/${id}`);
}

async function open(language: "en" | "pt-BR" = "en") {
  const view = await renderApp(`/jobs/${id}`, language);
  await screen.findByRole(
    "heading",
    { level: 1, name: t("progress.title", { profile: "Production" }) },
    { timeout: 4000 },
  );
  return view;
}

function group(language: "en" | "pt-BR"): HTMLElement {
  return screen.getByRole("group", { name: t(`language.${language}`) });
}

// The messages the live region reads, one entry for each time it is set.
function listen(): string[] {
  const region = document.querySelector("[aria-live='polite']");
  const heard: string[] = [];
  new MutationObserver(() => {
    const text = region?.textContent ?? "";
    if (text !== "") {
      heard.push(text);
    }
  }).observe(region as Node, { childList: true, characterData: true, subtree: true });
  return heard;
}

async function advance(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
}

beforeEach(() => {
  window.localStorage.clear();
  mockColourScheme(false);
  mock.profiles = [profileSummary()];
});

afterEach(() => {
  vi.useRealTimers();
});

describe("progress screen", () => {
  it("steps advance", async () => {
    mock.reports = [report({ ...running, status: "queued", phase: "waiting" })];
    vi.useFakeTimers({ shouldAdvanceTime: true });
    await open();
    const steps = screen.getByRole("list", { name: t("progress.steps") });
    const current = () => within(steps).getByRole("listitem", { current: "step" }).textContent;
    const phases = ["waiting", "preflight", "collecting", "analyzing", "snapshotting", "rendering", "complete"];
    // What a screen reader hears: the name, then the state of a step that is not the current one.
    const heard = () => within(steps).getAllByRole("listitem").map((item) => item.textContent);
    expect(heard()).toEqual([
      t("reportPhase.waiting"),
      ...phases.slice(1).map((phase) => `${t(`reportPhase.${phase}`)}${t("progress.stepPending")}`),
    ]);
    expect(current()).toBe(t("reportPhase.waiting"));

    mock.reports = [report(running)];
    await advance(1700);
    await waitFor(() => {
      expect(current()).toBe(t("reportPhase.collecting"));
    });
    expect(heard()).toEqual(
      phases.map((phase, index) =>
        index === 2
          ? t("reportPhase.collecting")
          : `${t(`reportPhase.${phase}`)}${t(index < 2 ? "progress.stepDone" : "progress.stepPending")}`,
      ),
    );

    mock.reports = [report({ pdfs: [pdf("en", "pending")] })];
    await advance(1700);
    await screen.findByRole("group", { name: t("language.en") });
    expect(screen.queryByRole("list", { name: t("progress.steps") })).not.toBeInTheDocument();
    expect(screen.getByText(t("progress.keptUntil", { date: "2026-11-06" }))).toBeInTheDocument();
  });

  it("checks 19 times after 1.5 s, 20 after 3 s, then every 6 s", async () => {
    mock.reports = [report(running)];
    // The clock moves only when the test moves it; the real timer lets an answer settle.
    const realTimeout = globalThis.setTimeout;
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout", "setInterval", "clearInterval", "Date"] });
    // The query client hands its notices to React through a zero-delay timer, which runs when the clock is
    // advanced by nothing.
    const settle = async () => {
      await act(async () => {
        await new Promise((resolve) => realTimeout(resolve, 15));
        await vi.advanceTimersByTimeAsync(0);
        await new Promise((resolve) => realTimeout(resolve, 15));
        await vi.advanceTimersByTimeAsync(0);
      });
    };
    await renderApp(`/jobs/${id}`);
    for (let turn = 0; turn < 80 && checks().length === 0; turn++) {
      await settle();
    }
    await settle();
    expect(screen.getByRole("heading", { level: 1 })).toBeInTheDocument();
    expect(checks()).toHaveLength(1);
    for (let made = 1; made < 45; made++) {
      const wait = made < 20 ? 1500 : made < 40 ? 3000 : 6000;
      await advance(wait - 1);
      await settle();
      expect(checks(), `just before check ${String(made + 1)}`).toHaveLength(made);
      await advance(1);
      await settle();
      expect(checks(), `at check ${String(made + 1)}`).toHaveLength(made + 1);
    }
  }, 60_000);

  it("shows the elapsed time, and never a negative one", async () => {
    mock.reports = [report({ ...running, created_at: new Date(Date.now() - 42_000).toISOString() })];
    vi.useFakeTimers({ shouldAdvanceTime: true });
    await open();
    expect(screen.getByText(t("progress.elapsed", { time: "0:42" }))).toBeInTheDocument();
    expect(screen.getByText(t("progress.closeable"))).toBeInTheDocument();
    await advance(3000);
    await screen.findByText(t("progress.elapsed", { time: "0:45" }));
    mock.reports = [report({ ...running, created_at: new Date(Date.now() + 5000).toISOString() })];
    await advance(1700);
    await screen.findByText(t("progress.elapsed", { time: "0:00" }));
  });

  it("stops when final", async () => {
    for (const final of [{}, { status: "failed", failure_code: "ENDPOINT_UNREACHABLE" } as const]) {
      mock.reports = [report(final)];
      vi.useFakeTimers({ shouldAdvanceTime: true });
      const { unmount } = await open();
      mock.calls.length = 0;
      await advance(10_000);
      expect(checks()).toEqual([]);
      unmount();
      vi.useRealTimers();
    }
  });

  it("the PDF block follows the language", async () => {
    mock.reports = [report({ pdfs: [pdf("en", "ready")] })];
    await open();
    const order = () => screen.getAllByRole("group").map((node) => node.getAttribute("aria-labelledby"));
    expect(screen.getAllByRole("group")).toHaveLength(2);
    expect(order()[0]).toBe(group("en").getAttribute("aria-labelledby"));
    expect(within(group("en")).getByRole("link", { name: t("progress.download") })).toHaveAttribute(
      "href",
      `/api/v1/reports/${id}/pdf?lang=en`,
    );
    expect(within(group("pt-BR")).getByText(t("reports.notGenerated"))).toBeInTheDocument();

    await act(async () => {
      await i18next.changeLanguage("pt-BR");
    });
    expect(screen.getAllByRole("group")[0]).toBe(group("pt-BR"));
    expect(screen.getAllByRole("group")[1]).toBe(group("en"));
    expect(within(group("pt-BR")).getByText(t("reports.notGenerated"))).toBeInTheDocument();
    expect(within(group("pt-BR")).getByRole("button", { name: "Gerar PDF" })).toBeInTheDocument();
    expect(within(group("en")).getByRole("link", { name: "Baixar PDF" })).toBeInTheDocument();
  });

  it("generates the other language", async () => {
    mock.reports = [report({ pdfs: [pdf("en", "ready")] })];
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTimeAsync });
    await open();
    const heard = listen();
    await user.click(within(group("pt-BR")).getByRole("button", { name: t("progress.generatePdf") }));
    const posts = mock.calls.filter((call) => call.method === "POST" && call.path === `/reports/${id}/pdfs`);
    expect(posts.map((call) => call.body)).toEqual([{ language: "pt-BR" }]);
    await waitFor(() => {
      expect(within(group("pt-BR")).getByText(t("pdfStatus.pending"))).toBeInTheDocument();
    });
    expect(within(group("pt-BR")).queryByRole("button")).not.toBeInTheDocument();

    mock.reports = [report({ pdfs: [pdf("en", "ready"), pdf("pt-BR", "ready")] })];
    await advance(1700);
    await waitFor(() => {
      expect(within(group("pt-BR")).getByRole("link", { name: t("progress.download") })).toHaveAttribute(
        "href",
        `/api/v1/reports/${id}/pdf?lang=pt-BR`,
      );
    });
    await advance(10_000);
    expect(heard).toEqual([t("progress.announce.pdfReady", { language: t("language.pt-BR") })]);
  });

  it("retries a failed PDF", async () => {
    mock.reports = [report({ pdfs: [pdf("en", "failed", "PDF_COMPILE_FAILED")] })];
    const user = userEvent.setup();
    await open();
    expect(within(group("en")).getByText(t("pdfFailure.PDF_COMPILE_FAILED"))).toBeInTheDocument();
    await user.click(within(group("en")).getByRole("button", { name: t("progress.retryPdf") }));
    const posts = mock.calls.filter((call) => call.method === "POST" && call.path === `/reports/${id}/pdfs`);
    expect(posts.map((call) => call.body)).toEqual([{ language: "en" }]);
    await waitFor(() => {
      expect(within(group("en")).getByText(t("pdfStatus.pending"))).toBeInTheDocument();
    });
  });

  it("a reader downloads only", async () => {
    signIn("reader");
    mock.reports = [report({ pdfs: [pdf("en", "ready"), pdf("pt-BR", "failed", "PDF_TOO_LARGE")] })];
    await open();
    expect(within(group("en")).getByRole("link", { name: t("progress.download") })).toBeInTheDocument();
    expect(within(group("pt-BR")).getByText(t("pdfFailure.PDF_TOO_LARGE"))).toBeInTheDocument();
    expect(within(document.getElementById("main") as HTMLElement).queryByRole("button")).not.toBeInTheDocument();
  });

  it("a failed report", async () => {
    mock.reports = [report({ status: "failed", failure_code: "ENDPOINT_UNREACHABLE" })];
    const user = userEvent.setup();
    const { router } = await open();
    const alert = await screen.findByRole("alert");
    expect(within(alert).getByText(t("reportFailure.ENDPOINT_UNREACHABLE"))).toBeInTheDocument();
    expect(within(alert).getByText(t("progress.code", { code: "ENDPOINT_UNREACHABLE" }))).toHaveClass("font-mono");
    expect(screen.getByRole("link", { name: t("progress.back") })).toHaveAttribute("href", "/reports");
    await user.click(screen.getByRole("link", { name: t("progress.requestAgain") }));
    await waitFor(() => {
      expect(location(router)).toBe(`/generate?from=${id}`);
    });
  });

  it("a reader cannot request again", async () => {
    signIn("reader");
    mock.reports = [report({ status: "failed", failure_code: "ENDPOINT_UNREACHABLE" })];
    await open();
    await screen.findByRole("alert");
    expect(screen.getByRole("link", { name: t("progress.back") })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: t("progress.requestAgain") })).not.toBeInTheDocument();
  });

  it("a report deleted elsewhere", async () => {
    mock.reports = [report(running)];
    vi.useFakeTimers({ shouldAdvanceTime: true });
    await open();
    mock.reports = [];
    await advance(1700);
    expect(await screen.findByRole("heading", { level: 1, name: t("progress.screen") })).toBeInTheDocument();
    expect(await screen.findByText(t("progress.notFound"))).toBeInTheDocument();
    expect(screen.getByRole("link", { name: t("progress.back") })).toHaveAttribute("href", "/reports");
    mock.calls.length = 0;
    await advance(20_000);
    expect(checks()).toEqual([]);
  });

  it("expired", async () => {
    server.use(http.get(`${window.location.origin}/api/v1/reports/:id`, () => failure(410, "REPORT_EXPIRED")));
    vi.useFakeTimers({ shouldAdvanceTime: true });
    await renderApp(`/jobs/${id}`);
    expect(await screen.findByRole("heading", { level: 1, name: t("progress.screen") })).toBeInTheDocument();
    expect(await screen.findByText(t("progress.expired"))).toBeInTheDocument();
    expect(screen.getByRole("link", { name: t("progress.back") })).toBeInTheDocument();
    mock.calls.length = 0;
    await advance(20_000);
    expect(checks()).toEqual([]);
  });

  it("deletes from here", async () => {
    mock.reports = [report()];
    const user = userEvent.setup();
    const { router } = await open();
    await user.click(screen.getByRole("button", { name: t("progress.deleteReport") }));
    const dialog = await screen.findByRole("alertdialog");
    await user.click(within(dialog).getByRole("button", { name: t("delete.confirm") }));
    await waitFor(() => {
      expect(location(router)).toBe("/reports");
    });
    const sent = mock.calls.map((call) => `${call.method} ${call.path}`);
    expect(sent).toContain(`DELETE /reports/${id}`);
    expect(sent.slice(sent.indexOf(`DELETE /reports/${id}`))).not.toContain(`GET /reports/${id}`);
  });

  it("an operator deletes only their own report", async () => {
    signIn("operator");
    mock.reports = [report({ owner_id: "9a9a9a9a-0000-4000-8000-000000000001" })];
    await open();
    expect(screen.queryByRole("button", { name: t("progress.deleteReport") })).not.toBeInTheDocument();
  });

  it("announces each change once", async () => {
    mock.reports = [report(running)];
    vi.useFakeTimers({ shouldAdvanceTime: true });
    await open();
    const heard = listen();
    mock.reports = [report({ pdfs: [pdf("en", "pending")] })];
    await advance(1700);
    await screen.findByRole("group", { name: t("language.en") });
    mock.reports = [report({ pdfs: [pdf("en", "ready")] })];
    await advance(1700);
    await waitFor(() => {
      expect(heard).toHaveLength(2);
    });
    await advance(20_000);
    expect(heard).toEqual([
      t("progress.announce.collected"),
      t("progress.announce.pdfReady", { language: t("language.en") }),
    ]);
  });

  it("announces a failure once", async () => {
    mock.reports = [report(running)];
    vi.useFakeTimers({ shouldAdvanceTime: true });
    await open();
    const heard = listen();
    mock.reports = [report({ status: "failed", failure_code: "INTERNAL_FAILURE" })];
    await advance(1700);
    await screen.findByRole("alert");
    await advance(20_000);
    expect(heard).toEqual([t("progress.announce.failed")]);
  });
});
