import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll } from "vitest";
import type { components } from "@/api/schema.gen";

type Schemas = components["schemas"];
type Session = Schemas["Session"];
type Principal = Schemas["Principal"];

export interface Call {
  method: string;
  path: string;
  csrf: string | null;
  key: string | null;
  body: unknown;
}

export function principal(overrides: Partial<Principal> = {}): Principal {
  return {
    id: "6f1f0d7e-3a54-4d57-9c1c-1f6a7f0a1c11",
    username: "ana",
    role: "administrator",
    language: "en",
    ...overrides,
  };
}

export function signedIn(overrides: Partial<Principal> = {}, csrf = "csrf-1"): Schemas["SignedInSession"] {
  return { authenticated: true, principal: principal(overrides), csrf_token: csrf, expires_at: "2030-01-01T00:00:00Z" };
}

export function anonymous(loginCsrf = "login-1"): Schemas["AnonymousSession"] {
  return { authenticated: false, login_csrf: loginCsrf };
}

export function errorBody(code: Schemas["ErrorCode"], fields: Record<string, string> = {}): Schemas["Error"] {
  return { error: { code, message: "message", fields } };
}

// An error answer of the contract, with the fields and the Retry-After it may carry.
export function failure(
  status: number,
  code: Schemas["ErrorCode"],
  extra: { fields?: Record<string, string>; retryAfter?: number } = {},
): Response {
  return HttpResponse.json(errorBody(code, extra.fields), {
    status,
    headers: extra.retryAfter === undefined ? {} : { "Retry-After": String(extra.retryAfter) },
  });
}

type Report = Schemas["Report"];
type ProfileSummary = Schemas["ProfileSummary"];

export function report(overrides: Partial<Report> = {}): Report {
  return {
    id: "0b6c3b52-2f0e-4d0a-8d57-6f6f6c1f0a01",
    owner_id: principal().id,
    owner_username: "ana",
    profile_id: "prod",
    status: "collected",
    phase: "complete",
    failure_code: null,
    pdfs: [],
    created_at: "2026-10-07T17:02:00Z",
    completed_at: "2026-10-07T17:05:00Z",
    expires_at: "2026-11-06T17:05:00Z",
    expired: false,
    profile_revision_id: "1c7d4c63-3f1f-4e1b-9e68-7f7f7d2f1b02",
    attempt: 1,
    started_at: "2026-10-07T17:02:05Z",
    collected_at: "2026-10-07T17:05:00Z",
    selection: {
      namespaces: [],
      period: "24h",
      start: null,
      end: null,
      timezone: "America/Sao_Paulo",
      report_profile: "detailed",
      min_severity: "all",
      language: "en",
    },
    ...overrides,
  };
}

export function profileSummary(overrides: Partial<ProfileSummary> = {}): ProfileSummary {
  return {
    id: "prod",
    display_name: "Production",
    allowed_namespaces: [],
    permitted_periods: [],
    default_period: "24h",
    permitted_timezones: [],
    default_timezone: "America/Sao_Paulo",
    permitted_report_profiles: ["detailed", "executive"],
    default_report_profile: "detailed",
    permitted_min_severities: ["all", "attention", "critical"],
    default_min_severity: "all",
    ...overrides,
  };
}

// The data the report and profile handlers serve; a test sets it, and it is
// emptied after each test.
export interface Mock {
  session: Session;
  calls: Call[];
  reports: Report[];
  profiles: ProfileSummary[];
  // The reports requested by key, so that a key sent again answers the same report.
  requested: Map<string, { body: string; id: string }>;
}

const api = (path: string): string => `${window.location.origin}/api/v1${path}`;

export const server = setupServer();

// Installs the handlers of the contract, held by one mutable state, and the
// hooks that start and stop the server around a test file.
export function installMockApi(initial: Session): Mock {
  const mock: Mock = { session: initial, calls: [], reports: [], profiles: [], requested: new Map() };

  async function record(request: Request): Promise<unknown> {
    const text = await request.clone().text();
    const body: unknown = text === "" ? null : JSON.parse(text);
    mock.calls.push({
      method: request.method,
      path: new URL(request.url).pathname.replace("/api/v1", ""),
      csrf: request.headers.get("X-CSRF-Token"),
      key: request.headers.get("Idempotency-Key"),
      body,
    });
    return body;
  }

  // Deletes a report as the signed-in principal may: an operator only their own.
  function remove(id: string): Schemas["ReportDeleteOutcome"] {
    const found = mock.reports.find((r) => r.id === id);
    if (found === undefined) {
      return "not_found";
    }
    const current = mock.session.authenticated && "principal" in mock.session ? mock.session.principal : null;
    if (current === null || current.role === "reader" || (current.role === "operator" && found.owner_id !== current.id)) {
      return "forbidden";
    }
    mock.reports = mock.reports.filter((r) => r.id !== id);
    return "deleted";
  }

  const handlers = [
    http.get(api("/auth/session"), async ({ request }) => {
      await record(request);
      return HttpResponse.json(mock.session);
    }),
    http.post(api("/auth/login"), async ({ request }) => {
      await record(request);
      mock.session = signedIn();
      return HttpResponse.json(mock.session);
    }),
    http.post(api("/auth/logout"), async ({ request }) => {
      await record(request);
      mock.session = anonymous();
      return new HttpResponse(null, { status: 204 });
    }),
    http.patch(api("/account"), async ({ request }) => {
      const body = (await record(request)) as { language?: Schemas["Language"]; password?: string };
      if (body.password !== undefined) {
        mock.session = anonymous();
        return HttpResponse.json(principal());
      }
      if (mock.session.authenticated && "principal" in mock.session) {
        mock.session = {
          ...mock.session,
          principal: { ...mock.session.principal, ...(body.language === undefined ? {} : { language: body.language }) },
        };
        return HttpResponse.json(mock.session.principal);
      }
      return HttpResponse.json(errorBody("UNAUTHENTICATED"), { status: 401 });
    }),
    http.get(api("/profiles"), async ({ request }) => {
      await record(request);
      return HttpResponse.json({ profiles: mock.profiles });
    }),
    http.get(api("/reports"), async ({ request }) => {
      await record(request);
      const url = new URL(request.url);
      const limit = Number(url.searchParams.get("limit") ?? "50");
      const start = Number(url.searchParams.get("cursor") ?? "0");
      const end = start + limit;
      return HttpResponse.json({
        items: mock.reports.slice(start, end),
        next_cursor: end < mock.reports.length ? String(end) : null,
      });
    }),
    http.post(api("/reports"), async ({ request }) => {
      const body = await record(request);
      const key = request.headers.get("Idempotency-Key") ?? "";
      const text = JSON.stringify(body);
      const earlier = mock.requested.get(key);
      if (earlier !== undefined && earlier.body !== text) {
        return failure(409, "IDEMPOTENCY_KEY_REUSED");
      }
      const id = earlier?.id ?? crypto.randomUUID();
      mock.requested.set(key, { body: text, id });
      return HttpResponse.json(
        {
          id,
          status: "queued",
          phase: "waiting",
          expires_at: "2026-11-06T17:05:00Z",
          replayed: earlier !== undefined,
        } satisfies Schemas["ReportAccepted"],
        { status: 202 },
      );
    }),
    http.post(api("/reports/delete"), async ({ request }) => {
      const body = (await record(request)) as Schemas["ReportDeleteRequest"];
      const results = body.ids.map((id): Schemas["ReportDeleteResult"] => ({ id, outcome: remove(id) }));
      return HttpResponse.json({ results } satisfies Schemas["ReportDeleteResults"]);
    }),
    http.post(api("/reports/:id/pdfs"), async ({ request, params }) => {
      const body = (await record(request)) as { language: Schemas["Language"] };
      const found = mock.reports.find((r) => r.id === params["id"]);
      if (found === undefined) {
        return failure(404, "REPORT_NOT_FOUND");
      }
      const earlier = found.pdfs.find((pdf) => pdf.language === body.language);
      if (earlier?.status === "ready") {
        return failure(409, "PDF_ALREADY_READY");
      }
      const pending: Schemas["ReportPDFState"] = { language: body.language, status: "pending", failure_code: null };
      found.pdfs = [...found.pdfs.filter((pdf) => pdf.language !== body.language), pending];
      return HttpResponse.json(
        { language: body.language, status: "pending" } satisfies Schemas["PDFRequestAccepted"],
        { status: 202 },
      );
    }),
    http.get(api("/reports/:id"), async ({ request, params }) => {
      await record(request);
      const found = mock.reports.find((r) => r.id === params["id"]);
      return found === undefined
        ? HttpResponse.json(errorBody("REPORT_NOT_FOUND"), { status: 404 })
        : HttpResponse.json(found);
    }),
    http.delete(api("/reports/:id"), async ({ request, params }) => {
      await record(request);
      const outcome = remove(String(params["id"]));
      if (outcome === "forbidden") {
        return HttpResponse.json(errorBody("FORBIDDEN"), { status: 403 });
      }
      return outcome === "deleted"
        ? new HttpResponse(null, { status: 204 })
        : HttpResponse.json(errorBody("REPORT_NOT_FOUND"), { status: 404 });
    }),
  ];

  beforeAll(() => {
    server.listen({ onUnhandledFrame: "error" });
  });
  afterEach(() => {
    mock.calls.length = 0;
    mock.session = initial;
    mock.reports = [];
    mock.profiles = [];
    mock.requested = new Map();
    server.resetHandlers(...handlers);
  });
  afterAll(() => {
    server.close();
  });
  server.use(...handlers);
  return mock;
}
