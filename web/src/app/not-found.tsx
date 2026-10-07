import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { useScreenTitle } from "./title";

export function NotFound() {
  const { t } = useTranslation();
  useScreenTitle(t("notFound.title"));
  return (
    <div className="flex max-w-prose flex-col gap-3">
      <h1 className="text-xl font-semibold">{t("notFound.title")}</h1>
      <p>{t("notFound.body")}</p>
      <Link to="/" className="text-accent underline">
        {t("notFound.home")}
      </Link>
    </div>
  );
}
