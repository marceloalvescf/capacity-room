import { useTranslation } from "react-i18next";
import { StatusMark } from "@/components/status-mark";
import type { ProfileTestResult } from "./api";

export type TestOutcome =
  | { kind: "result"; result: ProfileTestResult }
  | { kind: "refused" }
  | { kind: "failed"; text: string };

// What a test of a profile found: the outcome as a mark and a sentence, the draft the API refused to judge, or
// the failure of the call itself.
export function TestResultLine({ outcome }: { outcome: TestOutcome | null }) {
  const { t } = useTranslation();
  if (outcome === null) {
    return null;
  }
  switch (outcome.kind) {
    case "result":
      return (
        <p className="text-sm">
          <StatusMark
            status={outcome.result.ok ? "healthy" : "critical"}
            label={t(`profileTest.${outcome.result.code}`)}
          />
        </p>
      );
    case "refused":
      return <p className="text-sm text-muted-fg">{t("admin.profile.testRefused")}</p>;
    case "failed":
      return (
        <p className="text-sm">
          <StatusMark status="critical" label={outcome.text} />
        </p>
      );
  }
}
