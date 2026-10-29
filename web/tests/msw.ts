import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { afterAll, afterEach, beforeAll } from "vitest";
import type { components } from "@/api/schema.gen";
import type { Profile } from "@/features/admin/profiles/api";

type Schemas = components["schemas"];
type Session = Schemas["Session"];
type Principal = Schemas["Principal"];

export interface Call {
  method: string;
  path: string;
  search: string;
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
    language_follows_browser: false,
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
type ReportView = Schemas["ReportView"];
type ErrorCode = Schemas["ErrorCode"];
type ProfileSummary = Schemas["ProfileSummary"];
type Home = Schemas["Home"];
type AdminUser = Schemas["User"];
type AdminProfile = Profile;
type ProfileTestResult = Schemas["ProfileTestResult"];
type ProfileNamespaces = Schemas["ProfileNamespaces"];

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
    worst_severity: null,
    cluster_name: null,
    profile_revision_id: "1c7d4c63-3f1f-4e1b-9e68-7f7f7d2f1b02",
    attempt: 1,
    started_at: "2026-10-07T17:02:05Z",
    collected_at: "2026-10-07T17:05:00Z",
    selection: {
      namespaces: [],
      all_namespaces: false,
      period: "24h",
      start: null,
      end: null,
      timezone: "America/Sao_Paulo",
      report_profile: "detailed",
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
    ...overrides,
  };
}

export function adminUser(overrides: Partial<AdminUser> = {}): AdminUser {
  return {
    id: "a1b2c3d4-0000-4000-8000-000000000001",
    username: "ana.souza",
    role: "operator",
    language: "pt-BR",
    language_follows_browser: false,
    active: true,
    created_at: "2026-09-01T12:00:00Z",
    updated_at: "2026-09-01T12:00:00Z",
    ...overrides,
  };
}

// A profile as the administration reads it: the API's defaults, an empty list for every list, revision 1.
export function adminProfile(overrides: Partial<AdminProfile> = {}): AdminProfile {
  return {
    id: "production",
    revision_id: "5d1f3f0e-1b7d-4a53-8f55-0c9c5d0a0001",
    revision: 1,
    created_at: "2026-09-01T12:00:00Z",
    display_name: "Production",
    active: true,
    endpoint_url: "https://prometheus.example.com",
    timeout_seconds: 90,
    tls_verify: true,
    credential_file: "",
    ca_bundle_file: "",
    cluster_name: "prod-1",
    cluster_label: "cluster",
    filter_mode: "required",
    nodepool_label: "",
    node_autoscaling: "auto",
    allowed_namespaces: [],
    excluded_containers: [],
    permitted_periods: [],
    default_period: null,
    permitted_timezones: [],
    default_timezone: null,
    permitted_report_profiles: [],
    default_report_profile: null,
    thresholds: {
      warning_percent: 70,
      critical_percent: 85,
      low_utilization_percent: 30,
      autoscaler_near_max_percent: 80,
      nodepool_spread_points: 30,
      bursty_gap_percent: 100,
      hpa_replica_divergence: 1,
    },
    ...overrides,
  };
}

// A profile as the API stores it when a document leaves every field out.
function emptyProfile(id: string): AdminProfile {
  return adminProfile({
    id,
    revision_id: crypto.randomUUID(),
    revision: 1,
    display_name: "",
    active: false,
    endpoint_url: "",
    cluster_name: "",
  });
}

// A document laid over a profile: a field left out keeps its value, and a partial thresholds object keeps the keys left out.
function overlay(base: AdminProfile, document: Schemas["ProfileDocument"]): AdminProfile {
  return { ...base, ...document, thresholds: { ...base.thresholds, ...document.thresholds } };
}

// The data the report and profile handlers serve; a test sets it, and it is
// emptied after each test.
export interface Mock {
  session: Session;
  calls: Call[];
  reports: (Omit<Report, "selection"> & { selection: Report["selection"] | null })[];
  profiles: ProfileSummary[];
  // The clusters the home route serves.
  home: Home;
  // The reports requested by key, so that a key sent again answers the same report.
  requested: Map<string, { body: string; id: string }>;
  // The views of the reports, keyed `${id}:${lang}`, and the failures that replace them.
  views: Map<string, ReportView>;
  viewFailure: Map<string, { status: number; code: ErrorCode }>;
  // The users the administration routes serve, in name order.
  adminUsers: AdminUser[];
  // The profiles the administration routes serve, in id order.
  adminProfiles: AdminProfile[];
  // The outcome of a test, keyed by profile id or "draft"; a profile without one is ok.
  profileTests: Map<string, ProfileTestResult>;
  // The answer of the namespaces route, keyed by profile id; a profile without one lists three names, unlimited.
  profileNamespaces: Map<string, ProfileNamespaces>;
  // Answers that replace the next call of a route, keyed `METHOD /path`; each is used once.
  failures: Map<string, { status: number; code: ErrorCode; fields?: Record<string, string> }>;
  // When set, the creation of a user waits for it before it answers.
  hold: Promise<void> | null;
}

const api = (path: string): string => `${window.location.origin}/api/v1${path}`;

export const server = setupServer();

// Installs the handlers of the contract, held by one mutable state, and the
// hooks that start and stop the server around a test file.
export function installMockApi(initial: Session): Mock {
  const mock: Mock = { session: initial, calls: [], reports: [], profiles: [], home: { clusters: [] }, requested: new Map(),
    views: new Map(),
    viewFailure: new Map(),
    adminUsers: [],
    adminProfiles: [],
    profileTests: new Map(),
    profileNamespaces: new Map(),
    failures: new Map(),
    hold: null,
  };

  async function record(request: Request): Promise<unknown> {
    const text = await request.clone().text();
    const body: unknown = text === "" ? null : JSON.parse(text);
    mock.calls.push({
      method: request.method,
      path: new URL(request.url).pathname.replace("/api/v1", ""),
      search: new URL(request.url).search,
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

  // The answer a test set for this call, once.
  function refusal(method: string, path: string): Response | null {
    const key = `${method} ${path}`;
    const refused = mock.failures.get(key);
    if (refused === undefined) {
      return null;
    }
    mock.failures.delete(key);
    return failure(refused.status, refused.code, refused.fields === undefined ? {} : { fields: refused.fields });
  }

  // The outcome a test set for this profile, or a successful one.
  function testOutcome(key: string): ProfileTestResult {
    return mock.profileTests.get(key) ?? { ok: true, code: "PROFILE_OK", capabilities: [], namespaces: [] };
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
      const body = (await record(request)) as Schemas["AccountChange"];
      if (body.password !== undefined) {
        mock.session = anonymous();
        return HttpResponse.json(principal());
      }
      if (mock.session.authenticated && "principal" in mock.session) {
        mock.session = {
          ...mock.session,
          principal: {
            ...mock.session.principal,
            ...(body.language === undefined ? {} : { language: body.language, language_follows_browser: false }),
            ...(body.language_follows_browser === true ? { language_follows_browser: true } : {}),
          },
        };
        return HttpResponse.json(mock.session.principal);
      }
      return HttpResponse.json(errorBody("UNAUTHENTICATED"), { status: 401 });
    }),
    http.get(api("/admin/users"), async ({ request }) => {
      await record(request);
      return refusal("GET", "/admin/users") ?? HttpResponse.json({ users: mock.adminUsers } satisfies Schemas["UserList"]);
    }),
    http.post(api("/admin/users"), async ({ request }) => {
      const body = (await record(request)) as Schemas["NewUser"];
      await mock.hold;
      const refused = refusal("POST", "/admin/users");
      if (refused !== null) {
        return refused;
      }
      if (mock.adminUsers.some((user) => user.username === body.username)) {
        return failure(409, "USERNAME_TAKEN");
      }
      const created = adminUser({
        id: crypto.randomUUID(),
        username: body.username,
        role: body.role,
        language: body.language ?? "en",
        language_follows_browser: body.language === undefined,
      });
      mock.adminUsers = [...mock.adminUsers, created].sort((a, b) => a.username.localeCompare(b.username));
      return HttpResponse.json(created, { status: 201 });
    }),
    http.patch(api("/admin/users/:id"), async ({ request, params }) => {
      const body = (await record(request)) as Schemas["UserChange"];
      const id = String(params["id"]);
      const refused = refusal("PATCH", `/admin/users/${id}`);
      if (refused !== null) {
        return refused;
      }
      const found = mock.adminUsers.find((user) => user.id === id);
      if (found === undefined) {
        return failure(404, "USER_NOT_FOUND");
      }
      const changed = {
        ...found,
        ...(body.role === undefined ? {} : { role: body.role }),
        ...(body.language === undefined ? {} : { language: body.language, language_follows_browser: false }),
        ...(body.language_follows_browser === true ? { language_follows_browser: true } : {}),
        ...(body.active === undefined ? {} : { active: body.active }),
      };
      mock.adminUsers = mock.adminUsers.map((user) => (user.id === id ? changed : user));
      return HttpResponse.json(changed);
    }),
    http.delete(api("/admin/users/:id"), async ({ request, params }) => {
      await record(request);
      const id = String(params["id"]);
      const refused = refusal("DELETE", `/admin/users/${id}`);
      if (refused !== null) {
        return refused;
      }
      if (!mock.adminUsers.some((user) => user.id === id)) {
        return failure(404, "USER_NOT_FOUND");
      }
      mock.adminUsers = mock.adminUsers.map((user) => (user.id === id ? { ...user, active: false } : user));
      return new HttpResponse(null, { status: 204 });
    }),
    http.get(api("/admin/profiles"), async ({ request }) => {
      await record(request);
      return (
        refusal("GET", "/admin/profiles") ??
        HttpResponse.json({
          profiles: mock.adminProfiles.toSorted((a, b) => a.id.localeCompare(b.id)),
        } satisfies Schemas["ProfileList"])
      );
    }),
    http.post(api("/admin/profiles/test"), async ({ request }) => {
      await record(request);
      return refusal("POST", "/admin/profiles/test") ?? HttpResponse.json(testOutcome("draft"));
    }),
    http.post(api("/admin/profiles/:id/test"), async ({ request, params }) => {
      await record(request);
      const id = String(params["id"]);
      const refused = refusal("POST", `/admin/profiles/${id}/test`);
      if (refused !== null) {
        return refused;
      }
      return mock.adminProfiles.some((profile) => profile.id === id)
        ? HttpResponse.json(testOutcome(id))
        : failure(404, "PROFILE_NOT_FOUND");
    }),
    http.post(api("/admin/profiles"), async ({ request }) => {
      const body = (await record(request)) as Schemas["ProfileCreate"];
      const refused = refusal("POST", "/admin/profiles");
      if (refused !== null) {
        return refused;
      }
      if (mock.adminProfiles.some((profile) => profile.id === body.id)) {
        return failure(409, "PROFILE_ID_TAKEN");
      }
      const created = overlay(emptyProfile(body.id), body);
      mock.adminProfiles = [...mock.adminProfiles, created].sort((a, b) => a.id.localeCompare(b.id));
      return HttpResponse.json(created, { status: 201 });
    }),
    http.get(api("/admin/profiles/:id"), async ({ request, params }) => {
      await record(request);
      const id = String(params["id"]);
      const refused = refusal("GET", `/admin/profiles/${id}`);
      if (refused !== null) {
        return refused;
      }
      const found = mock.adminProfiles.find((profile) => profile.id === id);
      return found === undefined ? failure(404, "PROFILE_NOT_FOUND") : HttpResponse.json(found);
    }),
    http.patch(api("/admin/profiles/:id"), async ({ request, params }) => {
      const { base_revision_id: base, ...fields } = (await record(request)) as Schemas["ProfileChange"];
      const id = String(params["id"]);
      const refused = refusal("PATCH", `/admin/profiles/${id}`);
      if (refused !== null) {
        return refused;
      }
      const found = mock.adminProfiles.find((profile) => profile.id === id);
      if (found === undefined) {
        return failure(404, "PROFILE_NOT_FOUND");
      }
      if (base !== found.revision_id) {
        return failure(409, "PROFILE_REVISION_STALE");
      }
      const changed = overlay(found, fields);
      if (JSON.stringify(changed) === JSON.stringify(found)) {
        return HttpResponse.json(found);
      }
      const revised = { ...changed, revision_id: crypto.randomUUID(), revision: found.revision + 1 };
      mock.adminProfiles = mock.adminProfiles.map((profile) => (profile.id === id ? revised : profile));
      return HttpResponse.json(revised);
    }),
    http.delete(api("/admin/profiles/:id"), async ({ request, params }) => {
      await record(request);
      const id = String(params["id"]);
      const refused = refusal("DELETE", `/admin/profiles/${id}`);
      if (refused !== null) {
        return refused;
      }
      if (!mock.adminProfiles.some((profile) => profile.id === id)) {
        return failure(404, "PROFILE_NOT_FOUND");
      }
      mock.adminProfiles = mock.adminProfiles.filter((profile) => profile.id !== id);
      return new HttpResponse(null, { status: 204 });
    }),
    http.get(api("/profiles"), async ({ request }) => {
      await record(request);
      return HttpResponse.json({ profiles: mock.profiles });
    }),
    http.get(api("/profiles/:id/namespaces"), async ({ request, params }) => {
      await record(request);
      const id = String(params["id"]);
      return (
        refusal("GET", `/profiles/${id}/namespaces`) ??
        HttpResponse.json(
          mock.profileNamespaces.get(id) ?? {
            code: "PROFILE_OK",
            namespaces: ["checkout", "payments", "payments-batch"],
            limited: false,
          },
        )
      );
    }),
    http.get(api("/home"), async ({ request }) => {
      await record(request);
      return HttpResponse.json(mock.home);
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
    http.get(api("/reports/:id/view"), async ({ request, params }) => {
      await record(request);
      const key = `${String(params["id"])}:${new URL(request.url).searchParams.get("lang") ?? ""}`;
      const refused = mock.viewFailure.get(key);
      if (refused !== undefined) {
        return failure(refused.status, refused.code);
      }
      const view = mock.views.get(key);
      return view === undefined ? failure(404, "REPORT_NOT_FOUND") : HttpResponse.json(view);
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
    mock.home = { clusters: [] };
    mock.requested = new Map();
    mock.views = new Map();
    mock.viewFailure = new Map();
    mock.adminUsers = [];
    mock.adminProfiles = [];
    mock.profileTests = new Map();
    mock.profileNamespaces = new Map();
    mock.failures = new Map();
    mock.hold = null;
    server.resetHandlers(...handlers);
  });
  afterAll(() => {
    server.close();
  });
  server.use(...handlers);
  return mock;
}
