import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { mockColourScheme } from "../../tests/match-media";
import bootScript from "../../public/theme-boot.js?raw";
import { chooseTheme, currentTheme } from "./theme";

const root = document.documentElement;

function runBoot(): void {
  window.eval(bootScript);
}

beforeEach(() => {
  root.removeAttribute("data-theme");
  root.removeAttribute("data-theme-choice");
  window.localStorage.clear();
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("theme", () => {
  it.each([
    { stored: "dark", dark: false, expected: "dark" },
    { stored: "light", dark: true, expected: "light" },
    { stored: null, dark: true, expected: "dark" },
    { stored: null, dark: false, expected: "light" },
    { stored: "bogus", dark: true, expected: "dark" },
    { stored: "bogus", dark: false, expected: "light" },
  ])("boot script: stored $stored, system dark $dark gives $expected", ({ stored, dark, expected }) => {
    mockColourScheme(dark);
    if (stored !== null) {
      window.localStorage.setItem("capacityroom.theme", stored);
    }
    runBoot();
    expect(root.getAttribute("data-theme")).toBe(expected);
  });

  it("boot script follows the system while the choice is system", () => {
    const media = mockColourScheme(false);
    runBoot();
    expect(root.getAttribute("data-theme")).toBe("light");
    media.setDark(true);
    expect(root.getAttribute("data-theme")).toBe("dark");
  });

  it("boot script ignores the system once a choice is made", () => {
    const media = mockColourScheme(false);
    window.localStorage.setItem("capacityroom.theme", "light");
    runBoot();
    media.setDark(true);
    expect(root.getAttribute("data-theme")).toBe("light");
  });

  it("choosing stores the choice and applies it", () => {
    const media = mockColourScheme(false);
    runBoot();
    chooseTheme("dark");
    expect(root.getAttribute("data-theme")).toBe("dark");
    expect(window.localStorage.getItem("capacityroom.theme")).toBe("dark");
    media.setDark(false);
    expect(root.getAttribute("data-theme")).toBe("dark");

    media.setDark(true);
    chooseTheme("system");
    expect(root.getAttribute("data-theme")).toBe("dark");
    expect(window.localStorage.getItem("capacityroom.theme")).toBe("system");
    media.setDark(false);
    expect(root.getAttribute("data-theme")).toBe("light");
  });

  it("storage refused keeps the choice in memory and throws nothing", () => {
    mockColourScheme(false);
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("refused");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("refused");
    });
    runBoot();
    expect(root.getAttribute("data-theme")).toBe("light");
    expect(currentTheme()).toBe("system");
    expect(() => {
      chooseTheme("dark");
    }).not.toThrow();
    expect(currentTheme()).toBe("dark");
    expect(root.getAttribute("data-theme")).toBe("dark");
  });
});
