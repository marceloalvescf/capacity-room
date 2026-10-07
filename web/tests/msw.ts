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

export function errorBody(code: Schemas["ErrorCode"]): Schemas["Error"] {
  return { error: { code, message: "message", fields: {} } };
}

export interface Mock {
  session: Session;
  calls: Call[];
}

const api = (path: string): string => `${window.location.origin}/api/v1${path}`;

export const server = setupServer();

// Installs the handlers of the contract, held by one mutable state, and the
// hooks that start and stop the server around a test file.
export function installMockApi(initial: Session): Mock {
  const mock: Mock = { session: initial, calls: [] };

  async function record(request: Request): Promise<unknown> {
    const text = await request.clone().text();
    const body: unknown = text === "" ? null : JSON.parse(text);
    mock.calls.push({
      method: request.method,
      path: new URL(request.url).pathname.replace("/api/v1", ""),
      csrf: request.headers.get("X-CSRF-Token"),
      body,
    });
    return body;
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
  ];

  beforeAll(() => {
    server.listen({ onUnhandledFrame: "error" });
  });
  afterEach(() => {
    mock.calls.length = 0;
    mock.session = initial;
    server.resetHandlers(...handlers);
  });
  afterAll(() => {
    server.close();
  });
  server.use(...handlers);
  return mock;
}
