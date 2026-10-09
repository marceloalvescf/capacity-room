import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { errorText } from "@/api/errors";
import { StatusMark } from "@/components/status-mark";
import type { ProfileTestResult } from "./api";

export type TestOutcome =
  | { kind: "result"; result: ProfileTestResult }
  | { kind: "refused" }
  | { kind: "failed"; error: Error };

// The sentence of an outcome, which the line shows and the live region announces.
export function outcomeText(t: TFunction, outcome: TestOutcome): string {
  switch (outcome.kind) {
    case "result":
      return t(`profileTest.${outcome.result.code}`);
    case "refused":
      return t("admin.profile.testRefused");
    case "failed":
      return errorText(t, outcome.error);
  }
}

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
          <StatusMark status={outcome.result.ok ? "healthy" : "critical"} label={outcomeText(t, outcome)} />
        </p>
      );
    case "refused":
      return <p className="text-sm text-muted-fg">{outcomeText(t, outcome)}</p>;
    case "failed":
      return (
        <p className="text-sm">
          <StatusMark status="critical" label={outcomeText(t, outcome)} />
        </p>
      );
  }
}
