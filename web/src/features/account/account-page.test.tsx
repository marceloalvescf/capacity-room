import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, it } from "vitest";
import { anonymous, errorBody, server, signedIn, installMockApi } from "../../../tests/msw";
import { location, renderApp } from "../../../tests/render";
import { i18next } from "@/i18n/i18n";
import { sessionKey } from "@/session/session";

const mock = installMockApi(signedIn());
const url = (path: string) => `${window.location.origin}/api/v1${path}`;

beforeEach(() => {
  window.localStorage.clear();
  mock.session = signedIn();
});

const label = (key: string) => i18next.t(key);

async function typePasswords(user: ReturnType<typeof userEvent.setup>, current: string, next: string, repeat: string) {
  await user.type(await screen.findByLabelText(label("account.password.current")), current);
  await user.type(screen.getByLabelText(label("account.password.new")), next);
  await user.type(screen.getByLabelText(label("account.password.repeat")), repeat);
  await user.click(screen.getByRole("button", { name: label("account.password.submit") }));
}

describe("account", () => {
  it("shows who is signed in and the version", async () => {
    await renderApp("/account");
    expect(await screen.findByText("Signed in as ana, administrator.")).toBeInTheDocument();
    expect(screen.getByText(/^Version /)).toHaveClass("font-mono");
  });

  it("language: one PATCH, the screen follows, the live region says so", async () => {
    const user = userEvent.setup();
    await renderApp("/account");
    await user.click(await screen.findByRole("radio", { name: "Português (Brasil)" }));
    expect(await screen.findByRole("heading", { name: "Conta" })).toBeInTheDocument();
    const patches = mock.calls.filter((c) => c.method === "PATCH");
    expect(patches).toHaveLength(1);
    expect(patches[0]?.body).toEqual({ language: "pt-BR" });
    await waitFor(() => {
      expect(screen.getByRole("status")).toHaveTextContent("Idioma salvo.");
    });
  });

  it("password policy: short and differing entries send nothing", async () => {
    const user = userEvent.setup();
    await renderApp("/account");
    await typePasswords(user, "old-password-1", "short-pass1", "other-pass12");
    expect(await screen.findByText(label("validation.passwordShort"))).toBeInTheDocument();
    expect(screen.getByText(label("validation.passwordsDiffer"))).toBeInTheDocument();
    expect(mock.calls.some((c) => c.method === "PATCH")).toBe(false);
  });

  it("server policy: the 422 is worded", async () => {
    server.use(http.patch(url("/account"), () => HttpResponse.json(errorBody("PASSWORD_TOO_SHORT"), { status: 422 })));
    const user = userEvent.setup();
    await renderApp("/account");
    await typePasswords(user, "old-password-1", "twelve-chars-x", "twelve-chars-x");
    expect(await screen.findByRole("alert")).toHaveTextContent(label("error.PASSWORD_TOO_SHORT"));
  });

  it("a wrong current password is an error of its field", async () => {
    server.use(http.patch(url("/account"), () => HttpResponse.json(errorBody("INVALID_CREDENTIALS"), { status: 401 })));
    const user = userEvent.setup();
    const { router } = await renderApp("/account");
    await typePasswords(user, "old-password-1", "twelve-chars-x", "twelve-chars-x");
    const field = await screen.findByLabelText(label("account.password.current"));
    await waitFor(() => {
      expect(field).toHaveAccessibleDescription(label("error.INVALID_CREDENTIALS"));
    });
    expect(location(router)).toBe("/account");
  });

  it("a rate limit shows the wait", async () => {
    server.use(
      http.patch(url("/account"), () =>
        HttpResponse.json(errorBody("AUTH_RATE_LIMITED"), { status: 429, headers: { "Retry-After": "45" } }),
      ),
    );
    const user = userEvent.setup();
    await renderApp("/account");
    await typePasswords(user, "old-password-1", "twelve-chars-x", "twelve-chars-x");
    expect(await screen.findByRole("alert")).toHaveTextContent("45");
  });

  it("a password change signs out", async () => {
    const user = userEvent.setup();
    const { router, queryClient } = await renderApp("/account");
    await typePasswords(user, "old-password-1", "twelve-chars-x", "twelve-chars-x");
    await waitFor(() => {
      expect(location(router)).toBe("/login?changed=1");
    });
    expect(await screen.findByText(label("login.passwordChanged"))).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
    expect(mock.calls.find((c) => c.method === "PATCH")?.body).toEqual({
      current_password: "old-password-1",
      password: "twelve-chars-x",
    });
    await waitFor(() => {
      expect(queryClient.getQueryData(sessionKey)).toMatchObject({ authenticated: false });
    });
  });

  it("a 401 on sign-out ends the session quietly", async () => {
    server.use(
      http.post(url("/auth/logout"), () => {
        mock.session = anonymous();
        return HttpResponse.json(errorBody("UNAUTHENTICATED"), { status: 401 });
      }),
    );
    const user = userEvent.setup();
    const { router } = await renderApp("/account");
    await user.click(within(await screen.findByRole("banner")).getByRole("button", { name: label("header.signOut") }));
    await waitFor(() => {
      expect(location(router)).toBe("/login");
    });
    expect(screen.queryByRole("alert")).toBeNull();
    expect(location(router)).not.toContain("ended");
  });

  it("sign out posts the token and opens sign-in", async () => {
    const user = userEvent.setup();
    const { router } = await renderApp("/account");
    const header = await screen.findByRole("banner");
    await user.click(within(header).getByRole("button", { name: label("header.signOut") }));
    await waitFor(() => {
      expect(location(router)).toBe("/login");
    });
    const logout = mock.calls.find((c) => c.path === "/auth/logout");
    expect(logout).toMatchObject({ method: "POST", csrf: "csrf-1" });
  });
});
