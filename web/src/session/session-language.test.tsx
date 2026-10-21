import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { i18next } from "@/i18n/i18n";
import { installMockApi, signedIn } from "../../tests/msw";
import { renderApp } from "../../tests/render";

const mock = installMockApi(signedIn());

function browser(languages: string[]): void {
  Object.defineProperty(navigator, "languages", { value: languages, configurable: true });
  Object.defineProperty(navigator, "language", { value: languages[0] ?? "en-US", configurable: true });
}

beforeEach(() => {
  window.localStorage.clear();
});

afterEach(() => {
  browser(["en-US"]);
});

describe("the language after sign-in", () => {
  it.each([
    { languages: ["pt-BR"], follows: true, language: "en", expected: "pt-BR" },
    { languages: ["fr-FR"], follows: true, language: "pt-BR", expected: "en" },
    { languages: ["pt-BR"], follows: false, language: "en", expected: "en" },
    { languages: ["en-US"], follows: false, language: "pt-BR", expected: "pt-BR" },
  ])("browser $languages, follows $follows, account $language: $expected", async ({ languages, follows, language, expected }) => {
    browser(languages);
    mock.session = signedIn({ language: language as "en" | "pt-BR", language_follows_browser: follows });
    await renderApp("/account", "en");
    await waitFor(() => {
      expect(i18next.language).toBe(expected);
    });
  });

  it("picking a language in the header menu pins it", async () => {
    browser(["pt-BR"]);
    mock.session = signedIn({ language: "en", language_follows_browser: true });
    const user = userEvent.setup();
    await renderApp("/account", "en");
    await waitFor(() => {
      expect(i18next.language).toBe("pt-BR");
    });
    await user.click(screen.getByRole("button", { name: /Idioma/ }));
    await user.click(await screen.findByRole("menuitemradio", { name: "English" }));
    await waitFor(() => {
      expect(i18next.language).toBe("en");
    });
    expect(mock.calls.filter((c) => c.method === "PATCH").map((c) => c.body)).toEqual([{ language: "en" }]);
  });
});
