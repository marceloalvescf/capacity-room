import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Menu, MenuContent, MenuRadioGroup, MenuRadioItem, MenuTrigger } from "@/components/ui/menu";
import { chooseTheme, currentTheme, isThemeChoice, themeChoices } from "@/theme/theme";

export function ThemeToggle() {
  const { t } = useTranslation();
  const [choice, setChoice] = useState(currentTheme);

  function choose(value: string) {
    if (isThemeChoice(value)) {
      chooseTheme(value);
      setChoice(value);
    }
  }

  return (
    <Menu>
      <MenuTrigger asChild>
        <Button>{t("header.theme", { theme: t(`theme.${choice}`) })}</Button>
      </MenuTrigger>
      <MenuContent>
        <MenuRadioGroup value={choice} onValueChange={choose}>
          {themeChoices.map((option) => (
            <MenuRadioItem key={option} value={option}>
              {t(`theme.${option}`)}
            </MenuRadioItem>
          ))}
        </MenuRadioGroup>
      </MenuContent>
    </Menu>
  );
}
