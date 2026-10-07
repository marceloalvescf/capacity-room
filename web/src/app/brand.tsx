import { useTranslation } from "react-i18next";

export function Brand() {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col">
      <span className="font-bold">{t("app.short")}</span>
      <span className="text-sm text-sidebar-fg">{t("app.long")}</span>
    </div>
  );
}
