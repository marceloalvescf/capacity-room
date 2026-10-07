import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { http, HttpResponse } from "msw";
import { beforeEach, describe, expect, it } from "vitest";
import { anonymous, errorBody, server, signedIn, installMockApi } from "../../../tests/msw";
import { location, renderApp } from "../../../tests/render";
import { i18next } from "@/i18n/i18n";
import { sessionKey } from "@/session/session";

const mock = installMockApi(anonymous());
const url = (path: string) => `${window.location.origin}/api/v1${path}`;

beforeEach(() => {
  window.localStorage.clear();
  mock.session = anonymous();
});

async function fill(user: ReturnType<typeof userEvent.setup>) {
  await user.type(await screen.findByLabelText(i18next.t("login.username")), "ana");
  await user.type(screen.getByLabelText(i18next.t("login.password")), "correct horse");
  await user.click(screen.getByRole("button", { name: i18next.t("login.submit") }));
}

describe("login", () => {
  it("success: the session holds the principal and the screen leaves", async () => {
    const user = userEvent.setup();
    const { router, queryClient } = await renderApp("/login?next=%2Fnothing");
    await fill(user);
    await waitFor(() => {
      expect(location(router)).toBe("/nothing");
    });
    expect(queryClient.getQueryData(sessionKey)).toMatchObject({ principal: { username: "ana" } });
    expect(mock.calls.find((c) => c.method === "POST")?.csrf).toBe("login-1");
  });

  it("success without next goes to the reports", async () => {
    const user = userEvent.setup();
    const { router } = await renderApp("/login");
    await fill(user);
    await waitFor(() => {
      expect(location(router)).toBe("/reports");
    });
  });

  it("wrong credentials: the alert is worded and takes focus", async () => {
    server.use(http.post(url("/auth/login"), () => HttpResponse.json(errorBody("INVALID_CREDENTIALS"), { status: 401 })));
    const user = userEvent.setup();
    await renderApp("/login");
    await fill(user);
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(i18next.t("error.INVALID_CREDENTIALS"));
    expect(alert).toHaveFocus();
  });

  it("rate limited: the alert holds the wait", async () => {
    server.use(
      http.post(url("/auth/login"), () =>
        HttpResponse.json(errorBody("AUTH_RATE_LIMITED"), { status: 429, headers: { "Retry-After": "120" } }),
      ),
    );
    const user = userEvent.setup();
    await renderApp("/login");
    await fill(user);
    expect(await screen.findByRole("alert")).toHaveTextContent("120");
    expect(screen.getByRole("alert")).toHaveTextContent(i18next.t("error.AUTH_RATE_LIMITED", { seconds: 120 }));
  });

  it("empty fields name what is missing and send nothing", async () => {
    const user = userEvent.setup();
    await renderApp("/login");
    await user.click(await screen.findByRole("button", { name: i18next.t("login.submit") }));
    expect(await screen.findAllByText(i18next.t("validation.required"))).toHaveLength(2);
    expect(mock.calls.some((c) => c.method === "POST")).toBe(false);
  });

  it("a password of 1025 characters is refused before any request", async () => {
    const user = userEvent.setup();
    await renderApp("/login");
    await user.type(await screen.findByLabelText(i18next.t("login.username")), "ana");
    await user.click(screen.getByLabelText(i18next.t("login.password")));
    await user.paste("x".repeat(1025));
    await user.click(screen.getByRole("button", { name: i18next.t("login.submit") }));
    expect(await screen.findByText(i18next.t("validation.passwordLong"))).toBeInTheDocument();
    expect(mock.calls.some((c) => c.method === "POST")).toBe(false);
  });

  it("the notices follow the query", async () => {
    await renderApp("/login?changed=1&ended=1");
    expect(await screen.findByText(i18next.t("login.passwordChanged"))).toBeInTheDocument();
    expect(screen.getByText(i18next.t("login.sessionEnded"))).toBeInTheDocument();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("language before sign-in: the choice is stored and applied", async () => {
    const user = userEvent.setup();
    await renderApp("/login", "pt-BR");
    expect(await screen.findByRole("heading", { name: "Entrar" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /Idioma/ }));
    await user.click(await screen.findByRole("menuitemradio", { name: "English" }));
    expect(await screen.findByRole("heading", { name: "Sign in" })).toBeInTheDocument();
    expect(window.localStorage.getItem("kcr.language")).toBe("en");
    expect(document.documentElement.lang).toBe("en");
  });

  it("the account language applies after sign-in and the stored choice goes", async () => {
    window.localStorage.setItem("kcr.language", "en");
    server.use(
      http.post(url("/auth/login"), () => {
        mock.session = signedIn({ language: "pt-BR" });
        return HttpResponse.json(mock.session);
      }),
    );
    const user = userEvent.setup();
    await renderApp("/login");
    await fill(user);
    expect(await screen.findByRole("heading", { name: "Relatórios", level: 1 })).toBeInTheDocument();
    expect(window.localStorage.getItem("kcr.language")).toBeNull();
    expect(document.documentElement.lang).toBe("pt-BR");
  });

  it("a signed-in reader is sent on", async () => {
    mock.session = signedIn();
    const { router } = await renderApp("/login?next=%2Fnothing");
    await waitFor(() => {
      expect(location(router)).toBe("/nothing");
    });
  });
});
