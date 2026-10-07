import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { adoptLanguage, chooseLanguage, i18next } from "./i18n";
import { browserLanguage, forgetLanguage, initialLanguage } from "./language";

function browser(languages: string[], language = "en-US"): void {
  Object.defineProperty(navigator, "languages", { value: languages, configurable: true });
  Object.defineProperty(navigator, "language", { value: language, configurable: true });
}

beforeEach(() => {
  window.localStorage.clear();
  forgetLanguage();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("language", () => {
  it.each([
    { languages: ["pt-PT"], language: "en-US", expected: "pt-BR" },
    { languages: ["PT"], language: "en-US", expected: "pt-BR" },
    { languages: ["en-US"], language: "pt-BR", expected: "en" },
    { languages: [], language: "pt-BR", expected: "pt-BR" },
    { languages: ["de"], language: "de", expected: "en" },
  ])("browser default: $languages and $language give $expected", ({ languages, language, expected }) => {
    browser(languages, language);
    expect(browserLanguage()).toBe(expected);
    expect(initialLanguage()).toBe(expected);
  });

  it("stored choice wins before sign-in", () => {
    browser(["pt-BR"]);
    window.localStorage.setItem("kcr.language", "en");
    expect(initialLanguage()).toBe("en");
    window.localStorage.setItem("kcr.language", "fr");
    expect(initialLanguage()).toBe("pt-BR");
  });

  it("choosing stores the choice and sets the html lang", () => {
    chooseLanguage("pt-BR");
    expect(window.localStorage.getItem("kcr.language")).toBe("pt-BR");
    expect(document.documentElement.lang).toBe("pt-BR");
    expect(i18next.t("status.healthy")).toBe("Saudável");
    chooseLanguage("en");
    expect(document.documentElement.lang).toBe("en");
  });

  it("signing in applies the principal's language and drops the stored choice", () => {
    chooseLanguage("en");
    adoptLanguage("pt-BR");
    expect(window.localStorage.getItem("kcr.language")).toBeNull();
    expect(document.documentElement.lang).toBe("pt-BR");
  });

  it("storage refused keeps the choice in memory and throws nothing", () => {
    browser(["en-US"]);
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("refused");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("refused");
    });
    expect(initialLanguage()).toBe("en");
    expect(() => {
      chooseLanguage("pt-BR");
    }).not.toThrow();
    expect(initialLanguage()).toBe("pt-BR");
  });
});
