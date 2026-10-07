import { useEffect } from "react";
import { adoptLanguage } from "@/i18n/i18n";
import { usePrincipal } from "./session";

// Once signed in, the principal's language applies and the stored choice goes.
export function SessionLanguage() {
  const language = usePrincipal()?.language;
  useEffect(() => {
    if (language !== undefined) {
      adoptLanguage(language);
    }
  }, [language]);
  return null;
}
