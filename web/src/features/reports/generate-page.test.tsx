import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, it } from "vitest";
import { i18next } from "@/i18n/i18n";
import type { Language } from "@/i18n/language";
import { failure, installMockApi, profileSummary, report, server, signedIn } from "../../../tests/msw";
import { location, renderApp } from "../../../tests/render";
import { mockColourScheme } from "../../../tests/match-media";

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
    expect(screen.getByText("Últimas 24 horas · America/Sao_Paulo · Detalhado · Todas")).toBeInTheDocument();
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
    expect(screen.getByText("Last 7 days · UTC · Executive · Attention and critical")).toBeInTheDocument();
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

  it("Change opens the options and moves focus to the period", async () => {
    const { user } = await open();
    const change = screen.getByRole("button", { name: t("generate.change") });
    expect(change).not.toHaveAttribute("aria-expanded");
    await user.click(change);
    const trigger = screen.getByRole("button", { name: new RegExp(t("generate.period")) });
    await waitFor(() => {
      expect(trigger).toHaveFocus();
    });
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
    await user.click(screen.getByRole("button", { name: t("generate.change") }));
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
    await user.click(screen.getByRole("button", { name: t("generate.change") }));
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
    expect(screen.getByLabelText(t("generate.timezone"))).toBeInTheDocument();
    const trigger = screen.getByRole("button", { name: new RegExp(t("generate.period")) });
    await waitFor(() => {
      expect(trigger).toHaveFocus();
    });
    expect(trigger).toHaveAccessibleDescription(t("validation.notPermitted"));
    expect(screen.queryByText(t("error.REPORT_SELECTION_NOT_PERMITTED"))).not.toBeInTheDocument();
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
    expect(screen.getByText("Last 36 hours · America/Sao_Paulo · Detailed · Critical only")).toBeInTheDocument();
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
