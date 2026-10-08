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
  viewConfidenceValues,
  viewEvidenceNameValues,
  viewHPACoverageValues,
  viewScopeTypeValues,
} from "@/api/schema.gen";
import { statusValues } from "@/components/status-mark";
import { reportTabs } from "@/features/report/tabs";
import { calendarRanges } from "@/features/reports/period";
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
  "report.tab": reportTabs,
  confidence: viewConfidenceValues,
  scope: viewScopeTypeValues,
  evidence: viewEvidenceNameValues,
  hpaCoverage: viewHPACoverageValues,
  "period.calendar": calendarRanges,
};
