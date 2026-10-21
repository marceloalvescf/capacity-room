import type { components } from "@/api/schema.gen";
import { languageValues } from "@/api/schema.gen";
import { readStored, removeStored, writeStored } from "@/lib/storage";

export type Language = components["schemas"]["Language"];

const storageKey = "capacityroom.language";

export function isLanguage(value: unknown): value is Language {
  return languageValues.some((language) => language === value);
}

export function browserLanguage(): Language {
  const first = navigator.languages[0] ?? navigator.language;
  return first.toLowerCase().startsWith("pt") ? "pt-BR" : "en";
}

// Before sign-in the stored choice wins, then the browser's language.
export function initialLanguage(): Language {
  const stored = readStored(storageKey);
  return isLanguage(stored) ? stored : browserLanguage();
}

export function storeLanguage(language: Language): void {
  writeStored(storageKey, language);
}

export function forgetLanguage(): void {
  removeStored(storageKey);
}

// The choice of an account: a pinned language, or the browser's.
export const browserChoice = "browser";
export type LanguageChoice = Language | typeof browserChoice;

export function choiceOf(account: { language: Language; language_follows_browser: boolean }): LanguageChoice {
  return account.language_follows_browser ? browserChoice : account.language;
}

export function isLanguageChoice(value: unknown): value is LanguageChoice {
  return value === browserChoice || isLanguage(value);
}

// The body that saves a choice: a language pins it, the browser's unpins.
export function choiceBody(choice: LanguageChoice): { language: Language } | { language_follows_browser: true } {
  return choice === browserChoice ? { language_follows_browser: true } : { language: choice };
}
