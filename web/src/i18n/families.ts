import {
  errorCodeValues,
  languageValues,
  minSeverityValues,
  pDFFailureCodeValues,
  pDFStatusValues,
  reportFailureCodeValues,
  reportPhaseValues,
  reportProfileValues,
  reportStatusValues,
  roleValues,
} from "@/api/schema.gen";
import { statusValues } from "@/components/status-mark";
import { themeChoices } from "@/theme/theme";

// Each family of keys, with the list its members must equal.
export const families: Readonly<Record<string, readonly string[]>> = {
  error: [...errorCodeValues, "general", "network"],
  role: roleValues,
  language: languageValues,
  status: statusValues,
  theme: themeChoices,
  reportStatus: reportStatusValues,
  reportPhase: reportPhaseValues,
  pdfStatus: pDFStatusValues,
  reportProfile: reportProfileValues,
  minSeverity: minSeverityValues,
  pdfFailure: pDFFailureCodeValues,
  reportFailure: reportFailureCodeValues,
};
