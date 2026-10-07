import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "react-i18next";
import { useApi } from "@/api/context";
import { errorText, toError } from "@/api/errors";
import { useAnnounce } from "@/app/live-region";
import type { Language } from "@/i18n/language";
import { isSignedIn, sessionKey, type Session } from "./session";

// Saves the language on the account, then keeps the answer in the session so
// the language applies; the live region says so in the new language.
export function useChangeLanguage(): (language: Language) => Promise<void> {
  const { t } = useTranslation();
  const api = useApi();
  const queryClient = useQueryClient();
  const announce = useAnnounce();

  return async (language) => {
    try {
      const { data } = await api.PATCH("/account", { body: { language } });
      if (data === undefined) {
        return;
      }
      queryClient.setQueryData<Session>(sessionKey, (session) =>
        session !== undefined && isSignedIn(session) ? { ...session, principal: data } : session,
      );
      announce(t("account.language.saved", { lng: data.language }));
    } catch (thrown) {
      announce(errorText(t, toError(thrown)));
    }
  };
}
