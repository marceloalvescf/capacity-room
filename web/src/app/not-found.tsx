import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { useScreenTitle } from "./title";

// The screen of a path no route answers; `level` is the heading it carries, 2 when it sits under a title of its own.
export function NotFound({ level = 1 }: { level?: 1 | 2 }) {
  const { t } = useTranslation();
  const Heading = level === 1 ? "h1" : "h2";
  useScreenTitle(t("notFound.title"));
  return (
    <div className="flex max-w-prose flex-col gap-3">
      <Heading className="text-xl font-semibold">{t("notFound.title")}</Heading>
      <p>{t("notFound.body")}</p>
      <Link to="/" className="text-accent underline">
        {t("notFound.home")}
      </Link>
    </div>
  );
}
