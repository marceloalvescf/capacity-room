import { Trans } from "react-i18next";

// A sentence whose `<date>` parts never break inside, so a date and its time stay on one line.
export function DatedText({ i18nKey, values }: { i18nKey: string; values: Record<string, string | number> }) {
  return <Trans i18nKey={i18nKey} values={values} components={{ date: <span className="whitespace-nowrap" /> }} />;
}
