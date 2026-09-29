package collect

import "testing"

// TestRegistriesOfCodes transcribes the shape the brief pins: seven
// warning codes, two notice codes, and a "count" parameter for each
// notice.
func TestRegistriesOfCodes(t *testing.T) {
	wantWarnings := []string{
		WarningNamespaceUsageIncomplete,
		WarningNamespaceRequestsLimitsIncomplete,
		WarningNamespaceRequestsLimitsUnavailable,
		WarningWorkloadUsageIncomplete,
		WarningWorkloadRequestsLimitsIncomplete,
		WarningWorkloadRequestsLimitsUnavailable,
		WarningWorkloadHistoryCurrentAssociation,
	}
	if len(WarningCodes) != len(wantWarnings) {
		t.Fatalf("len(WarningCodes) = %d, want %d", len(WarningCodes), len(wantWarnings))
	}
	for i, code := range wantWarnings {
		if WarningCodes[i] != code {
			t.Errorf("WarningCodes[%d] = %q, want %q", i, WarningCodes[i], code)
		}
	}

	wantNotices := []string{
		NoticeWorkloadsOmittedNoUsage,
		NoticeJobsOmitted,
	}
	if len(NoticeCodes) != len(wantNotices) {
		t.Fatalf("len(NoticeCodes) = %d, want %d", len(NoticeCodes), len(wantNotices))
	}
	for i, code := range wantNotices {
		if NoticeCodes[i] != code {
			t.Errorf("NoticeCodes[%d] = %q, want %q", i, NoticeCodes[i], code)
		}
	}

	for _, code := range wantNotices {
		params, ok := NoticeParams[code]
		if !ok {
			t.Fatalf("NoticeParams has no entry for %q", code)
		}
		if len(params) != 1 || params[0] != "count" {
			t.Errorf("NoticeParams[%q] = %v, want [\"count\"]", code, params)
		}
	}
}
