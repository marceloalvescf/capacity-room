import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { i18next } from "@/i18n/i18n";
import { installMockApi, signedIn } from "../../tests/msw";
import { renderApp } from "../../tests/render";

const mock = installMockApi(signedIn());

beforeEach(() => {
  window.localStorage.clear();
  Object.defineProperty(navigator, "languages", { value: ["pt-BR"], configurable: true });
  Object.defineProperty(navigator, "language", { value: "pt-BR", configurable: true });
});

afterEach(() => {
  Object.defineProperty(navigator, "languages", { value: ["en-US"], configurable: true });
  Object.defineProperty(navigator, "language", { value: "en-US", configurable: true });
});

describe("the language menu", () => {
  it("pins the browser's own language when it is picked", async () => {
    mock.session = signedIn({ language: "en", language_follows_browser: true });
    const user = userEvent.setup();
    await renderApp("/account", "en");
    await waitFor(() => {
      expect(i18next.language).toBe("pt-BR");
    });
    await user.click(screen.getByRole("button", { name: /Idioma/ }));
    await user.click(await screen.findByRole("menuitemradio", { name: "Português (Brasil)" }));
    await waitFor(() => {
      expect(mock.calls.filter((c) => c.method === "PATCH").map((c) => c.body)).toEqual([{ language: "pt-BR" }]);
    });
    expect(mock.session.authenticated && "principal" in mock.session && mock.session.principal.language_follows_browser).toBe(false);
  });
});
