import { QueryClient } from "@tanstack/react-query";
import { http, HttpResponse } from "msw";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { anonymous, errorBody, server, signedIn, installMockApi } from "../../tests/msw";
import { sessionKey, type Session } from "@/session/session";
import { createApi, ApiFailure, requestTimeout } from "./client";

const mock = installMockApi(signedIn());
const url = (path: string) => `${window.location.origin}/api/v1${path}`;

function setup(session: Session = signedIn()) {
  const queryClient = new QueryClient();
  queryClient.setQueryData(sessionKey, session);
  const navigate = vi.fn<(to: string) => void>();
  return { queryClient, navigate, api: createApi(queryClient, navigate) };
}

beforeEach(() => {
  window.history.replaceState(null, "", "/account");
  mock.session = signedIn({}, "csrf-2");
});

afterEach(() => {
  vi.useRealTimers();
});

describe("client", () => {
  it("a request with no answer rejects as a timeout once the wait is over", async () => {
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
    const { api } = setup();
    server.use(http.get(url("/auth/session"), () => new Promise(() => {})));
    let outcome: unknown = null;
    void api.GET("/auth/session").catch((thrown: unknown) => {
      outcome = thrown;
    });
    await vi.advanceTimersByTimeAsync(requestTimeout - 1);
    expect(outcome).toBeNull();
    await vi.advanceTimersByTimeAsync(1);
    expect(outcome).toMatchObject({ name: "TimeoutError" });
    expect(outcome).not.toBeInstanceOf(ApiFailure);
  });

  it("a state-changing call carries the session token and a read carries none", async () => {
    const { api } = setup(signedIn({}, "csrf-7"));
    await api.GET("/auth/session");
    await api.PATCH("/account", { body: { language: "en" } });
    expect(mock.calls.map((c) => [c.method, c.csrf])).toEqual([
      ["GET", null],
      ["PATCH", "csrf-7"],
    ]);
  });

  it("the sign-in call carries the sign-in token", async () => {
    const { api } = setup(anonymous("login-9"));
    await api.POST("/auth/login", { body: { username: "ana", password: "secret-secret" } });
    expect(mock.calls[0]?.csrf).toBe("login-9");
  });

  it("a rejected token is refreshed and the call repeated once", async () => {
    const { api, queryClient } = setup(signedIn({}, "old"));
    let patches = 0;
    server.use(
      http.patch(url("/account"), ({ request }) => {
        patches += 1;
        mock.calls.push({
          method: "PATCH",
          path: "/account",
          search: "",
          csrf: request.headers.get("X-CSRF-Token"),
          key: null,
          body: null,
        });
        return patches === 1
          ? HttpResponse.json(errorBody("CSRF_REJECTED"), { status: 403 })
          : HttpResponse.json(signedIn().principal);
      }),
    );
    const { data } = await api.PATCH("/account", { body: { language: "en" } });
    expect(data?.username).toBe("ana");
    expect(mock.calls.filter((c) => c.method === "PATCH").map((c) => c.csrf)).toEqual(["old", "csrf-2"]);
    expect(mock.calls.filter((c) => c.method === "GET")).toHaveLength(1);
    expect(queryClient.getQueryData(sessionKey)).toEqual(mock.session);
  });

  it("a second rejection is returned as an error", async () => {
    const { api } = setup();
    server.use(
      http.patch(url("/account"), ({ request }) => {
        mock.calls.push({
          method: "PATCH",
          path: "/account",
          search: "",
          csrf: request.headers.get("X-CSRF-Token"),
          key: null,
          body: null,
        });
        return HttpResponse.json(errorBody("CSRF_REJECTED"), { status: 403 });
      }),
    );
    await expect(api.PATCH("/account", { body: { language: "en" } })).rejects.toMatchObject({ code: "CSRF_REJECTED" });
    expect(mock.calls.filter((c) => c.method === "PATCH")).toHaveLength(2);
    expect(mock.calls.filter((c) => c.method === "GET")).toHaveLength(1);
  });

  it("a 401 clears the cache and opens sign-in", async () => {
    const { api, queryClient, navigate } = setup();
    server.use(http.get(url("/auth/session"), () => HttpResponse.json(errorBody("UNAUTHENTICATED"), { status: 401 })));
    await expect(api.GET("/auth/session")).rejects.toBeInstanceOf(ApiFailure);
    expect(queryClient.getQueryData(sessionKey)).toBeUndefined();
    expect(navigate).toHaveBeenCalledWith("/login?next=%2Faccount&ended=1");
  });

  it("a 401 on a path that only starts like sign-in opens sign-in", async () => {
    window.history.replaceState(null, "", "/loginx");
    const { api, navigate } = setup();
    server.use(http.get(url("/auth/session"), () => HttpResponse.json(errorBody("UNAUTHENTICATED"), { status: 401 })));
    await expect(api.GET("/auth/session")).rejects.toBeInstanceOf(ApiFailure);
    expect(navigate).toHaveBeenCalledWith("/login?next=%2Floginx&ended=1");
  });

  it("a 401 on the sign-in screen stays there", async () => {
    window.history.replaceState(null, "", "/login");
    const { api, navigate } = setup();
    server.use(http.get(url("/auth/session"), () => HttpResponse.json(errorBody("UNAUTHENTICATED"), { status: 401 })));
    await expect(api.GET("/auth/session")).rejects.toBeInstanceOf(ApiFailure);
    expect(navigate).not.toHaveBeenCalled();
  });

  it("a 401 UNAUTHENTICATED on the account change opens sign-in", async () => {
    const { api, queryClient, navigate } = setup();
    server.use(http.patch(url("/account"), () => HttpResponse.json(errorBody("UNAUTHENTICATED"), { status: 401 })));
    await expect(api.PATCH("/account", { body: { language: "en" } })).rejects.toMatchObject({ code: "UNAUTHENTICATED" });
    expect(queryClient.getQueryData(sessionKey)).toBeUndefined();
    expect(navigate).toHaveBeenCalledWith("/login?next=%2Faccount&ended=1");
  });

  it("a 401 INVALID_CREDENTIALS on sign-in stays on the form", async () => {
    const { api, navigate } = setup(anonymous());
    server.use(http.post(url("/auth/login"), () => HttpResponse.json(errorBody("INVALID_CREDENTIALS"), { status: 401 })));
    await expect(api.POST("/auth/login", { body: { username: "a", password: "b" } })).rejects.toMatchObject({
      code: "INVALID_CREDENTIALS",
    });
    expect(navigate).not.toHaveBeenCalled();
  });

  it("the 401 of a wrong password stays on the form", async () => {
    const { api, queryClient, navigate } = setup();
    server.use(http.patch(url("/account"), () => HttpResponse.json(errorBody("INVALID_CREDENTIALS"), { status: 401 })));
    await expect(api.PATCH("/account", { body: { password: "x", current_password: "y" } })).rejects.toMatchObject({
      status: 401,
      code: "INVALID_CREDENTIALS",
    });
    expect(navigate).not.toHaveBeenCalled();
    expect(queryClient.getQueryData(sessionKey)).toBeDefined();
  });

  it("a Retry-After header becomes seconds", async () => {
    const { api } = setup(anonymous());
    server.use(
      http.post(url("/auth/login"), () =>
        HttpResponse.json(errorBody("AUTH_RATE_LIMITED"), { status: 429, headers: { "Retry-After": "120" } }),
      ),
    );
    await expect(api.POST("/auth/login", { body: { username: "a", password: "b" } })).rejects.toMatchObject({
      status: 429,
      code: "AUTH_RATE_LIMITED",
      retryAfter: 120,
    });
  });

  it("client keeps the fields of an error", async () => {
    const { api } = setup();
    server.use(
      http.get(url("/auth/session"), () =>
        HttpResponse.json(
          { error: { code: "REQUEST_INVALID", message: "message", fields: { period: "not_permitted" } } },
          { status: 422 },
        ),
      ),
    );
    await expect(api.GET("/auth/session")).rejects.toMatchObject({ fields: { period: "not_permitted" } });
    server.use(
      http.get(url("/auth/session"), () => HttpResponse.json({ error: { code: "REQUEST_INVALID" } }, { status: 422 })),
    );
    await expect(api.GET("/auth/session")).rejects.toMatchObject({ fields: {} });
    server.use(http.get(url("/auth/session"), () => new HttpResponse("nope", { status: 500 })));
    await expect(api.GET("/auth/session")).rejects.toMatchObject({ fields: {} });
  });
});
