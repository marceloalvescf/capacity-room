import { TranslateIcon } from "@phosphor-icons/react";
import { useTranslation } from "react-i18next";
import { languageValues } from "@/api/schema.gen";
import { IconButton } from "@/components/ui/icon-button";
import { Menu, MenuContent, MenuRadioGroup, MenuRadioItem, MenuTrigger } from "@/components/ui/menu";
import { chooseLanguage } from "@/i18n/i18n";
import { isLanguage } from "@/i18n/language";
import { usePrincipal } from "@/session/session";
import { useChangeLanguage } from "@/session/use-change-language";

export function LanguageMenu() {
  const { t, i18n } = useTranslation();
  const signedIn = usePrincipal() !== null;
  const change = useChangeLanguage();
  const current = isLanguage(i18n.language) ? i18n.language : "en";

  function choose(value: string) {
    if (!isLanguage(value)) {
      return;
    }
    if (signedIn) {
      void change(value);
    } else {
      chooseLanguage(value);
    }
  }

  return (
    <Menu>
      <MenuTrigger asChild>
        <IconButton label={t("header.language", { language: t(`language.${current}`) })}>
          <TranslateIcon aria-hidden="true" className="size-5" />
        </IconButton>
      </MenuTrigger>
      <MenuContent>
        <MenuRadioGroup value={current} onValueChange={choose}>
          {languageValues.map((language) => (
            <MenuRadioItem key={language} value={language}>
              {t(`language.${language}`, { lng: language })}
            </MenuRadioItem>
          ))}
        </MenuRadioGroup>
      </MenuContent>
    </Menu>
  );
}
