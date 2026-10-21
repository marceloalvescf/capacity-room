import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { useApi } from "@/api/context";
import { errorText, toError } from "@/api/errors";
import { useAnnounce } from "@/app/live-region";
import { browserLanguage, choiceBody, type Language, type LanguageChoice } from "@/i18n/language";
import { isSignedIn, sessionKey, type Session } from "./session";

// Saves the choice on the account, a language or the browser's, then keeps the
// answer in the session so the language applies; the live region says so in
// the new language.
export function useChangeLanguage(): (choice: LanguageChoice) => Promise<void> {
  const { t } = useTranslation();
  const api = useApi();
  const queryClient = useQueryClient();
  const announce = useAnnounce();

  return async (choice) => {
    try {
      const { data } = await api.PATCH("/account", { body: choiceBody(choice) });
      if (data === undefined) {
        return;
      }
      queryClient.setQueryData<Session>(sessionKey, (session) =>
        session !== undefined && isSignedIn(session) ? { ...session, principal: data } : session,
      );
      const applied: Language = data.language_follows_browser ? browserLanguage() : data.language;
      announce(t("account.language.saved", { lng: applied }));
    } catch (thrown) {
      announce(errorText(t, toError(thrown)));
    }
  };
}
