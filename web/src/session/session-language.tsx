import { useEffect } from "react";
import { adoptLanguage } from "@/i18n/i18n";
import { browserLanguage } from "@/i18n/language";
import { usePrincipal } from "./session";

// Once signed in, the account's language applies, or the browser's while the
// account follows it, and the stored choice goes.
export function SessionLanguage() {
  const principal = usePrincipal();
  const language = principal?.language;
  const follows = principal?.language_follows_browser;
  useEffect(() => {
    if (language !== undefined) {
      adoptLanguage(follows === true ? browserLanguage() : language);
    }
  }, [language, follows]);
  return null;
}
