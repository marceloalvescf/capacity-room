import { useTranslation } from "react-i18next";

// The load-line disc: a circle crossed at its centre by the line a loaded hull may sink to.
function LoadLine() {
  return (
    <svg viewBox="0 0 30 30" aria-hidden="true" className="size-7 flex-none">
      <circle cx="15" cy="15" r="10" fill="none" stroke="currentColor" strokeWidth="2.25" />
      <line x1="1" y1="15" x2="29" y2="15" strokeWidth="2.5" className="stroke-accent" />
    </svg>
  );
}

export function Brand() {
  const { t } = useTranslation();
  return (
    <div className="flex items-center gap-2.5 px-3 text-fg">
      <LoadLine />
      <span className="text-lg font-bold tracking-tight">{t("app.short")}</span>
    </div>
  );
}
