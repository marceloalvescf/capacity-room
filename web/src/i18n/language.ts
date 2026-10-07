import type { components } from "@/api/schema.gen";
import { languageValues } from "@/api/schema.gen";
import { readStored, removeStored, writeStored } from "@/lib/storage";

export type Language = components["schemas"]["Language"];

const storageKey = "kcr.language";

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
