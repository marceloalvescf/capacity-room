import i18next from "i18next";
import { initReactI18next } from "react-i18next";
import en from "@/locales/en.json";
import ptBR from "@/locales/pt-BR.json";
import { forgetLanguage, initialLanguage, storeLanguage, type Language } from "./language";

const initial = initialLanguage();

void i18next.use(initReactI18next).init({
  lng: initial,
  fallbackLng: false,
  resources: { en: { translation: en }, "pt-BR": { translation: ptBR } },
  interpolation: { escapeValue: false },
  returnNull: false,
});

function followHtmlLang(language: string): void {
  document.documentElement.lang = language;
}

i18next.on("languageChanged", followHtmlLang);
followHtmlLang(initial);

// Before sign-in: the choice is stored and applied.
export function chooseLanguage(language: Language): void {
  storeLanguage(language);
  void i18next.changeLanguage(language);
}

// After sign-in: the principal's language applies and the stored choice goes.
export function adoptLanguage(language: Language): void {
  forgetLanguage();
  void i18next.changeLanguage(language);
}

export { i18next };
