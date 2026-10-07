import { useEffect } from "react";
import { useTranslation } from "react-i18next";

// Sets the page title from the name of the screen shown.
export function useScreenTitle(screen: string): void {
  const { t } = useTranslation();
  useEffect(() => {
    document.title = t("app.title", { screen });
  }, [t, screen]);
}
