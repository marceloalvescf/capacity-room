import { MonitorIcon, MoonIcon, SunIcon } from "@phosphor-icons/react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { IconButton } from "@/components/ui/icon-button";
import { Menu, MenuContent, MenuRadioGroup, MenuRadioItem, MenuTrigger } from "@/components/ui/menu";
import { chooseTheme, currentTheme, isThemeChoice, themeChoices, type ThemeChoice } from "@/theme/theme";

const icons: Record<ThemeChoice, typeof SunIcon> = { system: MonitorIcon, light: SunIcon, dark: MoonIcon };

export function ThemeToggle() {
  const { t } = useTranslation();
  const [choice, setChoice] = useState(currentTheme);
  const Icon = icons[choice];

  function choose(value: string) {
    if (isThemeChoice(value)) {
      chooseTheme(value);
      setChoice(value);
    }
  }

  return (
    <Menu>
      <MenuTrigger asChild>
        <IconButton label={t("header.theme", { theme: t(`theme.${choice}`) })}>
          <Icon aria-hidden="true" className="size-5" />
        </IconButton>
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
