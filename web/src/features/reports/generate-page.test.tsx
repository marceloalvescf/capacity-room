import { act, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient } from "@tanstack/react-query";
import { createMemoryRouter } from "react-router";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createApi, requestTimeout } from "@/api/client";
import { Providers } from "@/app/providers";
import { routes } from "@/app/router";
import { i18next } from "@/i18n/i18n";
import type { Language } from "@/i18n/language";
import { failure, installMockApi, profileSummary, report, server, signedIn } from "../../../tests/msw";
import { location, renderApp } from "../../../tests/render";
import { mockColourScheme } from "../../../tests/match-media";

// The options summary is one paragraph of words and values in their own spans.
function summaryLine(text: string) {
  return (_: string, element: Element | null) => element?.tagName === "P" && element.textContent === text;
}

const mock = installMockApi(signedIn());

const t = (key: string, options: Record<string, string | number> = {}): string => i18next.t(key, options);

const prod = profileSummary({
  allowed_namespaces: ["payments", "checkout", "catalog"],
});
const staging = profileSummary({
  id: "staging",
  display_name: "Staging",
  allowed_namespaces: ["web", "checkout"],
  default_period: "7d",
  default_timezone: "UTC",
  default_report_profile: "executive",
  default_min_severity: "attention",
});

afterEach(() => {
  vi.useRealTimers();
});

beforeEach(() => {
  window.localStorage.clear();
  mockColourScheme(false);
  mock.profiles = [prod, staging];
});

const reportsUrl = `${window.location.origin}/api/v1/reports`;

// Answers the next request for a report with the given failure and returns the
// keys of the requests it answered.
function answerOnce(answer: () => Response): string[] {
  const keys: string[] = [];
  server.use(
    http.post(
      reportsUrl,
      ({ request }) => {
        keys.push(request.headers.get("Idempotency-Key") ?? "");
        return answer();
      },
      { once: true },
    ),
  );
  return keys;
}

function posts() {
  return mock.calls.filter((call) => call.method === "POST" && call.path === "/reports");
}

async function open(path = "/generate", language: Language = "en") {
  const view = await renderApp(path, language);
  await screen.findByRole("heading", { level: 1, name: t("generate.title") });
  await screen.findByRole("button", { name: t("generate.submit") });
  return { ...view, user: userEvent.setup() };
}

const submit = () => screen.getByRole("button", { name: t("generate.submit") });
const profileSelect = () => screen.getByLabelText<HTMLSelectElement>(t("generate.profile"));
const namespace = (name: string) => screen.getByRole("checkbox", { name });

describe("new report form", () => {
  it("defaults from the profile", async () => {
    mock.session = signedIn({ language: "pt-BR" });
    await open("/generate", "pt-BR");
    expect(profileSelect().value).toBe("prod");
    expect(screen.getByText(summaryLine("America/Sao_Paulo · Detalhado · Todas"))).toBeInTheDocument();
    expect(screen.getAllByRole("checkbox").filter((box) => (box as HTMLInputElement).checked)).toEqual([]);
    expect(screen.getByRole("radio", { name: "Português (Brasil)" })).toBeChecked();
    expect(screen.getByRole("radio", { name: "English" })).not.toBeChecked();
  });

  it("another profile resets", async () => {
    const { user } = await open();
    await user.click(namespace("payments"));
    expect(namespace("payments")).toBeChecked();
    await user.selectOptions(profileSelect(), "staging");
    expect(screen.queryByRole("checkbox", { name: "payments" })).not.toBeInTheDocument();
    expect(screen.getAllByRole("checkbox").filter((box) => (box as HTMLInputElement).checked)).toEqual([]);
    expect(screen.getByText(summaryLine("UTC · Executive · Attention and critical"))).toBeInTheDocument();
  });

  it("namespaces are shown in monospace", async () => {
    await open();
    expect(namespace("payments").closest("fieldset")).toHaveClass("font-mono");
  });

  it("needs a namespace", async () => {
    const { user } = await open();
    await user.click(submit());
    expect(await screen.findByText(t("validation.namespaces"))).toBeInTheDocument();
    expect(posts()).toEqual([]);
    await user.click(namespace("payments"));
    expect(screen.queryByText(t("validation.namespaces"))).not.toBeInTheDocument();
  });

  it("a submit with no namespace marks the fieldset, describes it, and focuses it", async () => {
    const { user } = await open();
    await user.click(submit());
    const fieldset = namespace("payments").closest("fieldset") as HTMLElement;
    await waitFor(() => {
      expect(fieldset).toHaveFocus();
    });
    expect(fieldset).toHaveAttribute("aria-invalid", "true");
    expect(fieldset).toHaveAccessibleDescription(t("validation.namespaces"));
    await user.click(namespace("payments"));
    expect(fieldset).not.toHaveAttribute("aria-invalid");
  });

  it("Change opens the options and moves focus to the time zone", async () => {
    const { user } = await open();
    const change = screen.getByRole("button", { name: t("generate.change") });
    expect(change).not.toHaveAttribute("aria-expanded");
    await user.click(change);
    const zone = screen.getByLabelText(t("generate.timezone"));
    await waitFor(() => {
      expect(zone).toHaveFocus();
    });
  });

  it("the period is a field of its own, reachable with the options closed", async () => {
    const { user } = await open();
    expect(screen.queryByLabelText(t("generate.timezone"))).not.toBeInTheDocument();
    const trigger = screen.getByRole("button", { name: new RegExp(t("generate.period")) });
    expect(trigger.closest("div.max-w-96")).not.toBeNull();
    expect(screen.getByRole("group", { name: t("generate.language") }).compareDocumentPosition(trigger)).toBe(
      Node.DOCUMENT_POSITION_PRECEDING,
    );
    await user.click(trigger);
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
  });

  it("the closed options summary leaves the period out", async () => {
    await open();
    const options = screen.getByText(t("generate.options")).parentElement as HTMLElement;
    expect(options).toHaveTextContent("America/Sao_Paulo");
    expect(options).toHaveTextContent(t("reportProfile.detailed"));
    expect(options).not.toHaveTextContent("Last 24 hours");
    expect(screen.getByRole("button", { name: new RegExp(t("generate.period")) })).toHaveTextContent("Last 24 hours");
  });

  it("sends the choices and goes to the job", async () => {
    const { user, router } = await open();
    await user.click(namespace("checkout"));
    await user.click(namespace("payments"));
    await user.click(screen.getByRole("radio", { name: "Português (Brasil)" }));
    await user.click(submit());
    await waitFor(() => {
      expect(location(router)).toMatch(/^\/jobs\/[0-9a-f-]{36}$/);
    });
    expect(posts()).toHaveLength(1);
    expect(posts()[0]?.body).toEqual({
      profile_id: "prod",
      language: "pt-BR",
      namespaces: ["payments", "checkout"],
      period: "24h",
      start: null,
      end: null,
      timezone: "America/Sao_Paulo",
      report_profile: "detailed",
      min_severity: "all",
    });
    expect(posts()[0]?.csrf).toBe("csrf-1");
  });

  describe("one report per attempt", () => {
    it("a double click sends one request", async () => {
      const { user, router } = await open();
      await user.click(namespace("payments"));
      await user.dblClick(submit());
      await waitFor(() => {
        expect(location(router)).toMatch(/^\/jobs\/[0-9a-f-]{36}$/);
      });
      // A second request one tick later would be seen here.
      await new Promise((resolve) => setTimeout(resolve, 50));
      expect(posts()).toHaveLength(1);
    });

    it("a resend after a network failure keeps the key", async () => {
      const { user } = await open();
      await user.click(namespace("payments"));
      const keys = answerOnce(() => HttpResponse.error());
      await user.click(submit());
      expect(await screen.findByText(t("error.network"))).toBeInTheDocument();
      await user.click(submit());
      await waitFor(() => {
        expect(posts()).toHaveLength(1);
      });
      expect(keys).toHaveLength(1);
      expect(posts()[0]?.key).toBe(keys[0]);
    });

    it("a resend after a timeout keeps the key and the body", async () => {
      const { user } = await open();
      await user.click(namespace("payments"));
      const first: { key: string; body: unknown }[] = [];
      server.use(
        http.post(
          reportsUrl,
          async ({ request }) => {
            first.push({ key: request.headers.get("Idempotency-Key") ?? "", body: await request.json() });
            return new Promise<Response>(() => {});
          },
          { once: true },
        ),
      );
      vi.useFakeTimers({ shouldAdvanceTime: true });
      await user.click(submit());
      await waitFor(() => {
        expect(first).toHaveLength(1);
      });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(requestTimeout);
      });
      expect(await screen.findByText(t("error.timeout"))).toBeInTheDocument();
      await user.click(submit());
      await waitFor(() => {
        expect(posts()).toHaveLength(1);
      });
      expect(posts()[0]?.key).toBe(first[0]?.key);
      expect(posts()[0]?.body).toEqual(first[0]?.body);
    });

    it("a resend after an answer with unchanged values keeps the key", async () => {
      const { user } = await open();
      await user.click(namespace("payments"));
      const keys = answerOnce(() => failure(429, "ADMISSION_GLOBAL_LIMIT", { retryAfter: 30 }));
      await user.click(submit());
      await screen.findByText(t("error.ADMISSION_GLOBAL_LIMIT", { seconds: 30 }));
      await user.click(submit());
      await waitFor(() => {
        expect(posts()).toHaveLength(1);
      });
      expect(posts()[0]?.key).toBe(keys[0]);
    });

    it("a changed value after an answer renews the key", async () => {
      const { user } = await open();
      await user.click(namespace("payments"));
      const keys = answerOnce(() => failure(429, "ADMISSION_GLOBAL_LIMIT", { retryAfter: 30 }));
      await user.click(submit());
      await screen.findByText(t("error.ADMISSION_GLOBAL_LIMIT", { seconds: 30 }));
      await user.click(namespace("catalog"));
      await user.click(submit());
      await waitFor(() => {
        expect(posts()).toHaveLength(1);
      });
      expect(posts()[0]?.key).not.toBe(keys[0]);
    });

    it.each([
      [404, "REPORT_NOT_FOUND"],
      [410, "REPORT_EXPIRED"],
    ] as const)("a key whose replay answered %i is renewed", async (status, code) => {
      const { user } = await open();
      await user.click(namespace("payments"));
      const keys = answerOnce(() => failure(status, code));
      await user.click(submit());
      expect(await screen.findByText(t(`error.${code}`))).toBeInTheDocument();
      await user.click(submit());
      await waitFor(() => {
        expect(posts()).toHaveLength(1);
      });
      expect(posts()[0]?.key).not.toBe(keys[0]);
    });

    it("a rejected key is renewed", async () => {
      const { user } = await open();
      await user.click(namespace("payments"));
      const keys = answerOnce(() => failure(409, "IDEMPOTENCY_KEY_REUSED"));
      await user.click(submit());
      expect(await screen.findByText(t("error.IDEMPOTENCY_KEY_REUSED"))).toBeInTheDocument();
      await user.click(submit());
      await waitFor(() => {
        expect(posts()).toHaveLength(1);
      });
      expect(posts()[0]?.key).not.toBe(keys[0]);
    });
  });

  it("restricted periods", async () => {
    mock.profiles = [
      profileSummary({
        allowed_namespaces: ["payments"],
        permitted_periods: ["24h", "7d"],
      }),
    ];
    const { user } = await open();
    await user.click(screen.getByRole("button", { name: new RegExp(t("generate.period")) }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText(t("period.restricted"))).toBeInTheDocument();
    expect(within(dialog).queryByLabelText(t("period.custom.label"))).not.toBeInTheDocument();
  });

  it("long period", async () => {
    mock.profiles = [
      profileSummary({
        allowed_namespaces: ["payments"],
        default_period: "30d",
      }),
    ];
    const { user } = await open();
    expect(screen.getByText(t("generate.longPeriod"))).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: new RegExp(t("generate.period")) }));
    await user.click(await screen.findByRole("button", { name: t("period.quick.15d") }));
    expect(screen.queryByText(t("generate.longPeriod"))).not.toBeInTheDocument();
  });

  it("time zones", async () => {
    mock.profiles = [
      profileSummary({
        allowed_namespaces: ["payments"],
        default_timezone: "UTC",
      }),
    ];
    const { user } = await open();
    await user.click(screen.getByRole("button", { name: t("generate.change") }));
    const select = screen.getByLabelText<HTMLSelectElement>(t("generate.timezone"));
    const names = [...select.options].map((option) => option.value);
    expect(select.value).toBe("UTC");
    expect(names.filter((name) => name === "UTC")).toHaveLength(1);
    expect(names).toEqual(expect.arrayContaining(Intl.supportedValuesOf("timeZone")));
  });

  it("permitted time zones, in their order", async () => {
    mock.profiles = [
      profileSummary({
        allowed_namespaces: ["payments"],
        permitted_timezones: ["America/Sao_Paulo", "UTC"],
        permitted_report_profiles: ["executive"],
        default_report_profile: "executive",
        permitted_min_severities: ["critical"],
        default_min_severity: "critical",
      }),
    ];
    const { user } = await open();
    await user.click(screen.getByRole("button", { name: t("generate.change") }));
    const zones = screen.getByLabelText<HTMLSelectElement>(t("generate.timezone"));
    expect([...zones.options].map((option) => option.value)).toEqual(["America/Sao_Paulo", "UTC"]);
    const kinds = screen.getByLabelText<HTMLSelectElement>(t("generate.reportProfile"));
    expect([...kinds.options].map((option) => option.textContent)).toEqual([t("reportProfile.executive")]);
    const severities = screen.getByLabelText<HTMLSelectElement>(t("generate.minSeverity"));
    expect([...severities.options].map((option) => option.textContent)).toEqual([t("minSeverity.critical")]);
  });

  it("server fields", async () => {
    const { user } = await open();
    await user.click(namespace("payments"));
    server.use(
      http.post(
        `${window.location.origin}/api/v1/reports`,
        () =>
          failure(422, "REPORT_SELECTION_NOT_PERMITTED", {
            fields: { period: "not_permitted" },
          }),
        { once: true },
      ),
    );
    await user.click(submit());
    expect(await screen.findByText(t("validation.notPermitted"))).toBeInTheDocument();
    expect(screen.queryByLabelText(t("generate.timezone"))).not.toBeInTheDocument();
    const trigger = screen.getByRole("button", { name: new RegExp(t("generate.period")) });
    await waitFor(() => {
      expect(trigger).toHaveFocus();
    });
    expect(trigger).toHaveAccessibleDescription(t("validation.notPermitted"));
    expect(screen.queryByText(t("error.REPORT_SELECTION_NOT_PERMITTED"))).not.toBeInTheDocument();
  });

  it("a time zone problem still opens the options and focuses the time zone", async () => {
    const { user } = await open();
    await user.click(namespace("payments"));
    server.use(
      http.post(
        `${window.location.origin}/api/v1/reports`,
        () =>
          failure(422, "REPORT_SELECTION_NOT_PERMITTED", {
            fields: { timezone: "not_permitted" },
          }),
        { once: true },
      ),
    );
    await user.click(submit());
    const zone = await screen.findByLabelText(t("generate.timezone"));
    await waitFor(() => {
      expect(zone).toHaveFocus();
    });
    expect(zone).toHaveAccessibleDescription(t("validation.notPermitted"));
  });

  it("a field the form does not show falls back to the alert of the code", async () => {
    const { user } = await open();
    await user.click(namespace("payments"));
    server.use(
      http.post(
        `${window.location.origin}/api/v1/reports`,
        () =>
          failure(400, "REQUEST_INVALID", {
            fields: { profile_id: "invalid" },
          }),
        { once: true },
      ),
    );
    await user.click(submit());
    expect(await screen.findByText(t("error.REQUEST_INVALID"))).toBeInTheDocument();
  });

  it("an invalid namespace list is marked on the namespaces", async () => {
    const { user } = await open();
    await user.click(namespace("payments"));
    server.use(
      http.post(
        `${window.location.origin}/api/v1/reports`,
        () =>
          failure(400, "REQUEST_INVALID", {
            fields: { namespaces: "invalid" },
          }),
        { once: true },
      ),
    );
    await user.click(submit());
    expect(await screen.findByText(t("validation.invalid"))).toBeInTheDocument();
  });

  it("profile changed", async () => {
    const { user } = await open();
    await user.click(namespace("payments"));
    server.use(
      http.post(`${window.location.origin}/api/v1/reports`, () => failure(409, "PROFILE_INACTIVE"), { once: true }),
    );
    mock.calls.length = 0;
    await user.click(submit());
    expect(await screen.findByText(t("generate.profileChanged"))).toBeInTheDocument();
    await waitFor(() => {
      expect(mock.calls.filter((call) => call.path === "/profiles")).toHaveLength(1);
    });
  });

  it("a profile that is gone gives way to the first one", async () => {
    const { user } = await open();
    await user.selectOptions(profileSelect(), "staging");
    await user.click(namespace("web"));
    mock.profiles = [prod];
    server.use(
      http.post(`${window.location.origin}/api/v1/reports`, () => failure(409, "PROFILE_INACTIVE"), { once: true }),
    );
    await user.click(submit());
    await screen.findByText(t("generate.profileChanged"));
    await waitFor(() => {
      expect(profileSelect().value).toBe("prod");
    });
    expect(screen.getAllByRole("checkbox").filter((box) => (box as HTMLInputElement).checked)).toEqual([]);
  });

  it("admission", async () => {
    const { user } = await open();
    await user.click(namespace("payments"));
    server.use(
      http.post(
        `${window.location.origin}/api/v1/reports`,
        () => failure(429, "ADMISSION_GLOBAL_LIMIT", { retryAfter: 30 }),
        {
          once: true,
        },
      ),
    );
    await user.click(submit());
    expect(await screen.findByText(t("error.ADMISSION_GLOBAL_LIMIT", { seconds: 30 }))).toBeInTheDocument();
    expect(submit()).toBeEnabled();
  });

  it("from a failed report", async () => {
    const failed = report({
      status: "failed",
      profile_id: "staging",
      selection: {
        namespaces: ["checkout"],
        period: "36h",
        start: null,
        end: null,
        timezone: "America/Sao_Paulo",
        report_profile: "detailed",
        min_severity: "critical",
        language: "pt-BR",
      },
    });
    mock.reports = [failed];
    await open(`/generate?from=${failed.id}`);
    expect(profileSelect().value).toBe("staging");
    expect(namespace("checkout")).toBeChecked();
    expect(namespace("web")).not.toBeChecked();
    expect(screen.getByRole("radio", { name: "Português (Brasil)" })).toBeChecked();
    expect(screen.getByText(summaryLine("America/Sao_Paulo · Detailed · Critical only"))).toBeInTheDocument();
  });

  it("a report that cannot be read leaves a blank form", async () => {
    await open("/generate?from=00000000-0000-4000-8000-000000000000");
    expect(profileSelect().value).toBe("prod");
    expect(screen.getByText(t("error.REPORT_NOT_FOUND"))).toBeInTheDocument();
  });

  it("no profile", async () => {
    mock.profiles = [];
    await renderApp("/generate");
    expect(await screen.findByText(t("generate.noProfiles"))).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: t("generate.submit") })).not.toBeInTheDocument();
  });

  it("profiles that fail to load", async () => {
    server.use(http.get(`${window.location.origin}/api/v1/profiles`, () => failure(500, "INTERNAL_ERROR")));
    await renderApp("/generate");
    expect(await screen.findByText(t("generate.profilesFailed"))).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: t("generate.submit") })).not.toBeInTheDocument();
  });

  it("a reader", async () => {
    mock.session = signedIn({ role: "reader" });
    await renderApp("/generate");
    expect(await screen.findByRole("heading", { name: t("notFound.title") })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: t("nav.generate") })).not.toBeInTheDocument();
  });

  it("the sidebar offers New report between Reports and Account to an operator", async () => {
    mock.session = signedIn({ role: "operator" });
    await renderApp("/reports");
    const nav = await screen.findByRole("navigation", { name: t("nav.label") });
    expect(
      within(nav)
        .getAllByRole("link")
        .map((link) => link.textContent),
    ).toEqual([t("nav.reports"), t("nav.generate"), t("nav.account")]);
  });

  it("the sidebar hides New report from a reader", async () => {
    mock.session = signedIn({ role: "reader" });
    await renderApp("/reports");
    const nav = await screen.findByRole("navigation", { name: t("nav.label") });
    expect(
      within(nav)
        .getAllByRole("link")
        .map((link) => link.textContent),
    ).toEqual([t("nav.reports"), t("nav.account")]);
  });
});

describe("calendar ranges", () => {
  function atTime(iso: string) {
    vi.useFakeTimers({ shouldAdvanceTime: true, toFake: ["Date"] });
    vi.setSystemTime(new Date(iso));
  }

  async function openPeriod(user: ReturnType<typeof userEvent.setup>) {
    await user.click(screen.getByRole("button", { name: new RegExp(t("generate.period")) }));
    return screen.findByRole("dialog");
  }

  async function typeRange(user: ReturnType<typeof userEvent.setup>, start: string, end: string) {
    const dialog = await openPeriod(user);
    await user.click(within(dialog).getByRole("radio", { name: t("period.mode.range") }));
    for (const [label, text] of [
      [t("period.range.from"), start],
      [t("period.range.to"), end],
    ] as const) {
      const input = within(dialog).getByLabelText(label);
      await user.clear(input);
      await user.type(input, text);
    }
    await user.click(within(dialog).getByRole("button", { name: t("period.apply") }));
  }

  async function ready(user: ReturnType<typeof userEvent.setup>) {
    await user.click(namespace("payments"));
  }

  it("sends a calendar range resolved", async () => {
    atTime("2026-10-07T21:00:42Z");
    const { user } = await open();
    await ready(user);
    const dialog = await openPeriod(user);
    await user.click(within(dialog).getByRole("button", { name: t("period.calendar.previousWeek") }));
    await user.click(submit());
    await waitFor(() => {
      expect(posts()).toHaveLength(1);
    });
    expect(posts()[0]?.body).toMatchObject({ period: null, start: "2026-09-28 00:00", end: "2026-10-05 00:00" });
  });

  it("sends a length", async () => {
    const { user } = await open();
    await ready(user);
    const dialog = await openPeriod(user);
    await user.click(within(dialog).getByRole("button", { name: t("period.quick.7d") }));
    await user.click(submit());
    await waitFor(() => {
      expect(posts()).toHaveLength(1);
    });
    expect(posts()[0]?.body).toMatchObject({ period: "7d", start: null, end: null });
  });

  it("sends a typed range trimmed", async () => {
    atTime("2026-10-07T21:00:42Z");
    const { user } = await open();
    await ready(user);
    await typeRange(user, " 2026-09-01 00:00 ", "2026-10-01 00:00 ");
    await user.click(submit());
    await waitFor(() => {
      expect(posts()).toHaveLength(1);
    });
    expect(posts()[0]?.body).toMatchObject({ period: null, start: "2026-09-01 00:00", end: "2026-10-01 00:00" });
  });

  it("an empty so-far range", async () => {
    atTime("2026-10-07T03:00:20Z");
    const { user } = await open();
    await ready(user);
    const dialog = await openPeriod(user);
    await user.click(within(dialog).getByRole("button", { name: t("period.calendar.today") }));
    await user.click(submit());
    expect(await screen.findByText(t("period.range.order"))).toBeInTheDocument();
    expect(screen.getByRole("button", { name: new RegExp(t("generate.period")) })).toHaveAccessibleDescription(
      t("period.range.order"),
    );
    expect(posts()).toEqual([]);
  });

  const serverWords = [
    { name: "server wall-clock word", status: 400, code: "REQUEST_INVALID", field: "start", reason: "invalid", word: () => t("period.range.wallClock", { zone: "America/Sao_Paulo" }) },
    { name: "server order word", status: 400, code: "REQUEST_INVALID", field: "start", reason: "order", word: () => t("period.range.order") },
    { name: "server future word", status: 400, code: "REQUEST_INVALID", field: "end", reason: "future", word: () => t("period.range.future") },
    { name: "range not permitted", status: 422, code: "REPORT_SELECTION_NOT_PERMITTED", field: "start", reason: "not_permitted", word: () => t("validation.notPermitted") },
  ] as const;
  for (const c of serverWords) {
    it(c.name, async () => {
      const { user } = await open();
      await ready(user);
      server.use(
        http.post(reportsUrl, () => failure(c.status, c.code, { fields: { [c.field]: c.reason } }), { once: true }),
      );
      await user.click(submit());
      expect(await screen.findByText(c.word())).toBeInTheDocument();
      expect(screen.queryByLabelText(t("generate.timezone"))).not.toBeInTheDocument();
      const trigger = screen.getByRole("button", { name: new RegExp(t("generate.period")) });
      expect(trigger).toHaveAccessibleDescription(c.word());
      expect(trigger).toHaveFocus();
    });
  }

  it("zone change keeps the walls", async () => {
    atTime("2026-10-07T21:00:42Z");
    const { user } = await open();
    await ready(user);
    await typeRange(user, "2026-09-01 00:00", "2026-10-01 00:00");
    await user.click(screen.getByRole("button", { name: t("generate.change") }));
    await user.selectOptions(screen.getByLabelText(t("generate.timezone")), "America/New_York");
    await user.click(submit());
    await waitFor(() => {
      expect(posts()).toHaveLength(1);
    });
    expect(posts()[0]?.body).toMatchObject({
      period: null,
      start: "2026-09-01 00:00",
      end: "2026-10-01 00:00",
      timezone: "America/New_York",
    });
  });

  it("from a range report", async () => {
    const source = report({
      status: "failed",
      selection: {
        namespaces: [],
        period: null,
        start: "2026-09-01 00:00",
        end: "2026-10-01 00:00",
        timezone: "America/Sao_Paulo",
        report_profile: "detailed",
        min_severity: "all",
        language: "en",
      },
    });
    mock.reports = [source];
    await open(`/generate?from=${source.id}`);
    const summary = screen.getByText(summaryLine("America/Sao_Paulo · Detailed · All"));
    expect(within(summary).getByText("America/Sao_Paulo")).toHaveClass("font-mono");
    expect(within(summary).getByText("Detailed")).not.toHaveClass("font-mono");
    expect(screen.getByRole("button", { name: new RegExp(t("generate.period")) })).toHaveTextContent(
      "2026-09-01 00:00 to 2026-10-01 00:00",
    );
  });

  it("a resend of a calendar name keeps the key and the body", async () => {
    atTime("2026-10-07T21:00:42Z");
    const { user } = await open();
    await ready(user);
    const dialog = await openPeriod(user);
    await user.click(within(dialog).getByRole("button", { name: t("period.calendar.today") }));
    const first: { key: string; body: unknown }[] = [];
    server.use(
      http.post(
        reportsUrl,
        async ({ request }) => {
          first.push({ key: request.headers.get("Idempotency-Key") ?? "", body: await request.json() });
          return HttpResponse.error();
        },
        { once: true },
      ),
    );
    await user.click(submit());
    expect(await screen.findByText(t("error.network"))).toBeInTheDocument();
    vi.setSystemTime(new Date("2026-10-07T21:02:10Z"));
    await user.click(submit());
    await waitFor(() => {
      expect(posts()).toHaveLength(1);
    });
    expect(posts()[0]?.key).toBe(first[0]?.key);
    expect(posts()[0]?.body).toEqual(first[0]?.body);
    expect(posts()[0]?.body).toMatchObject({ period: null, start: "2026-10-07 00:00", end: "2026-10-07 18:00" });
  });

  it.each([
    [503, "DATABASE_UNAVAILABLE"],
    [429, "ADMISSION_GLOBAL_LIMIT"],
  ] as const)("a resend of a calendar name after an answered %i keeps the key and the body", async (status, code) => {
    atTime("2026-10-07T21:00:42Z");
    const { user } = await open();
    await ready(user);
    const dialog = await openPeriod(user);
    await user.click(within(dialog).getByRole("button", { name: t("period.calendar.today") }));
    const first: { key: string; body: unknown }[] = [];
    server.use(
      http.post(
        reportsUrl,
        async ({ request }) => {
          first.push({ key: request.headers.get("Idempotency-Key") ?? "", body: await request.json() });
          return failure(status, code, { retryAfter: 30 });
        },
        { once: true },
      ),
    );
    await user.click(submit());
    await waitFor(() => {
      expect(first).toHaveLength(1);
    });
    await screen.findByRole("alert");
    vi.setSystemTime(new Date("2026-10-07T21:02:10Z"));
    await user.click(submit());
    await waitFor(() => {
      expect(posts()).toHaveLength(1);
    });
    expect(posts()[0]?.key).toBe(first[0]?.key);
    expect(posts()[0]?.body).toEqual(first[0]?.body);
  });

  it("a profile that is gone renews the key", async () => {
    const { user } = await open();
    await user.selectOptions(profileSelect(), "staging");
    await user.click(namespace("web"));
    mock.profiles = [prod];
    const keys = answerOnce(() => failure(409, "PROFILE_INACTIVE"));
    await user.click(submit());
    await screen.findByText(t("generate.profileChanged"));
    await waitFor(() => {
      expect(profileSelect().value).toBe("prod");
    });
    await user.click(namespace("payments"));
    await user.click(submit());
    await waitFor(() => {
      expect(posts()).toHaveLength(1);
    });
    expect(posts()[0]?.key).not.toBe(keys[0]);
  });

  it("a range report of a restricted profile falls back to the profile's period", async () => {
    mock.profiles = [
      profileSummary({ allowed_namespaces: ["payments"], permitted_periods: ["24h", "7d"], default_period: "7d" }),
    ];
    const source = report({
      status: "failed",
      selection: {
        namespaces: [],
        period: null,
        start: "2026-09-01 00:00",
        end: "2026-10-01 00:00",
        timezone: "America/Sao_Paulo",
        report_profile: "detailed",
        min_severity: "all",
        language: "en",
      },
    });
    mock.reports = [source];
    await open(`/generate?from=${source.id}`);
    expect(screen.getByText(summaryLine("America/Sao_Paulo · Detailed · All"))).toBeInTheDocument();
  });

  it("long range", async () => {
    atTime("2026-10-07T21:00:42Z");
    const { user } = await open();
    await ready(user);
    await typeRange(user, "2026-09-01 00:00", "2026-09-17 00:00");
    expect(screen.getByText(t("generate.longPeriod"))).toBeInTheDocument();
    await typeRange(user, "2026-09-01 00:00", "2026-09-16 00:00");
    expect(screen.queryByText(t("generate.longPeriod"))).not.toBeInTheDocument();
  });

  it("a calendar name over 15 days warns", async () => {
    atTime("2026-10-31T21:00:42Z");
    const { user } = await open();
    await ready(user);
    const dialog = await openPeriod(user);
    await user.click(within(dialog).getByRole("button", { name: t("period.calendar.thisMonth") }));
    expect(screen.getByText(t("generate.longPeriod"))).toBeInTheDocument();
  });
});

describe("what a copied report dropped", () => {
  const selection = report().selection;

  it("a profile that is gone starts the form on the first one and says so", async () => {
    const source = report({ status: "failed", profile_id: "retired", selection });
    mock.reports = [source];
    await open(`/generate?from=${source.id}`);
    expect(profileSelect().value).toBe("prod");
    const notice = screen.getByText(/The profile of that report is no longer available/);
    expect(notice).toHaveTextContent(
      "The profile of that report is no longer available, so the form starts from Production.",
    );
    expect(notice.querySelector(".font-mono")).toBeNull();
  });

  it("a namespace the profile no longer allows is named", async () => {
    const source = report({ status: "failed", selection: { ...selection, namespaces: ["payments", "retired"] } });
    mock.reports = [source];
    await open(`/generate?from=${source.id}`);
    expect(namespace("payments")).toBeChecked();
    const notice = screen.getByText(/Namespaces the profile no longer allows/);
    expect(notice).toHaveTextContent("Namespaces the profile no longer allows were left out: retired.");
    expect(notice.querySelector(".font-mono")).toBeNull();
  });

  it("a range the profile no longer permits is named", async () => {
    mock.profiles = [
      profileSummary({ allowed_namespaces: ["payments"], permitted_periods: ["24h", "7d"], default_period: "7d" }),
    ];
    const source = report({
      status: "failed",
      selection: { ...selection, period: null, start: "2026-09-01 00:00", end: "2026-10-01 00:00" },
    });
    mock.reports = [source];
    await open(`/generate?from=${source.id}`);
    expect(screen.getByText(/The profile no longer permits that report's period/)).toHaveTextContent(
      "The profile no longer permits that report's period, so the form starts from Last 7 days.",
    );
  });

  it("the notices stay as first shown when the profiles load again", async () => {
    const source = report({ status: "failed", selection: { ...selection, namespaces: ["payments", "retired"] } });
    mock.reports = [source];
    const { user } = await open(`/generate?from=${source.id}`);
    const notice = /Namespaces the profile no longer allows/;
    const text = "Namespaces the profile no longer allows were left out: retired.";
    expect(screen.getByText(notice)).toHaveTextContent(text);
    server.use(
      http.post(
        reportsUrl,
        () => failure(422, "REPORT_SELECTION_NOT_PERMITTED", { fields: { namespaces: "not_permitted" } }),
        { once: true },
      ),
    );
    mock.profiles = [profileSummary({ allowed_namespaces: ["payments", "retired", "catalog"] }), staging];
    mock.calls.length = 0;
    await user.click(submit());
    await screen.findByRole("checkbox", { name: "catalog" });
    expect(screen.getByText(notice)).toHaveTextContent(text);
  });

  it("a copy that dropped nothing says nothing", async () => {
    const source = report({ status: "failed", selection: { ...selection, namespaces: ["payments"] } });
    mock.reports = [source];
    await open(`/generate?from=${source.id}`);
    expect(namespace("payments")).toBeChecked();
    expect(screen.queryByText(/no longer/)).not.toBeInTheDocument();
  });

  it("a selection the profile no longer permits loads the profiles again", async () => {
    mock.profiles = [profileSummary({ allowed_namespaces: ["payments", "checkout"] })];
    const { user } = await open();
    await user.click(namespace("payments"));
    server.use(
      http.post(
        reportsUrl,
        () => failure(422, "REPORT_SELECTION_NOT_PERMITTED", { fields: { namespaces: "not_permitted" } }),
        { once: true },
      ),
    );
    mock.profiles = [profileSummary({ allowed_namespaces: ["checkout", "catalog"] })];
    mock.calls.length = 0;
    await user.click(submit());
    await screen.findByText(t("validation.notPermitted"));
    await waitFor(() => {
      expect(mock.calls.filter((call) => call.path === "/profiles")).toHaveLength(1);
    });
    expect(await screen.findByRole("checkbox", { name: "catalog" })).toBeInTheDocument();
    expect(screen.queryByRole("checkbox", { name: "payments" })).not.toBeInTheDocument();
  });
});

describe("the send lock", () => {
  it("shows on the button while the page is being left", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: Infinity, refetchOnWindowFocus: false } },
    });
    await i18next.changeLanguage("en");
    const router = createMemoryRouter(
      [{ path: "/jobs/:id", loader: () => new Promise<never>(() => undefined), element: null }, ...routes],
      { initialEntries: ["/generate"] },
    );
    window.history.replaceState(null, "", "/generate");
    const api = createApi(queryClient, (to) => {
      void router.navigate(to);
    });
    render(<Providers queryClient={queryClient} api={api} router={router} />);
    await screen.findByRole("button", { name: t("generate.submit") });
    const user = userEvent.setup();
    await user.click(namespace("payments"));
    await user.click(submit());
    await waitFor(() => {
      expect(posts()).toHaveLength(1);
      expect(router.state.navigation.state).toBe("loading");
    });
    const locked = screen.getByRole("button", { name: t("generate.submitting") });
    expect(locked).toBeDisabled();
    await user.click(locked);
    expect(posts()).toHaveLength(1);
  });
});
