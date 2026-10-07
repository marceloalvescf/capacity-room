import { readStored, writeStored } from "@/lib/storage";

export const themeChoices = ["system", "light", "dark"] as const;
export type ThemeChoice = (typeof themeChoices)[number];

const storageKey = "kcr.theme";

export function isThemeChoice(value: unknown): value is ThemeChoice {
  return themeChoices.some((choice) => choice === value);
}

// The page holds the live choice in data-theme-choice: theme-boot.js sets it
// from storage before the stylesheet loads, and chooseTheme keeps it current.
export function currentTheme(): ThemeChoice {
  const live = document.documentElement.getAttribute("data-theme-choice");
  if (isThemeChoice(live)) {
    return live;
  }
  const stored = readStored(storageKey);
  return isThemeChoice(stored) ? stored : "system";
}

export function chooseTheme(choice: ThemeChoice): void {
  writeStored(storageKey, choice);
  const root = document.documentElement;
  root.setAttribute("data-theme-choice", choice);
  const dark = choice === "dark" || (choice === "system" && window.matchMedia("(prefers-color-scheme: dark)").matches);
  root.setAttribute("data-theme", dark ? "dark" : "light");
}
