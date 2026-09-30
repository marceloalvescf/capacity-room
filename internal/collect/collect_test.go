package collect

import "testing"

// TestRegistriesOfCodes transcribes the shape of the registries: the first seven
// warning codes, the first two notice codes, and a "count" parameter for each
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
	if len(WarningCodes) < len(wantWarnings) {
		t.Fatalf("len(WarningCodes) = %d, want at least %d", len(WarningCodes), len(wantWarnings))
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
	if len(NoticeCodes) < len(wantNotices) {
		t.Fatalf("len(NoticeCodes) = %d, want at least %d", len(NoticeCodes), len(wantNotices))
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

// TestNoticeAndWarningCodesGrew holds the eleven warnings and three notices,
// and the count parameter of the newest notice.
func TestNoticeAndWarningCodesGrew(t *testing.T) {
	if len(WarningCodes) != 11 {
		t.Fatalf("len(WarningCodes) = %d, want 11", len(WarningCodes))
	}
	if WarningCodes[7] != "HPA_AMBIGUOUS_SPEC" || WarningCodes[8] != "HPA_AMBIGUOUS_TARGET" {
		t.Errorf("WarningCodes[7:9] = %v, want HPA_AMBIGUOUS_SPEC, HPA_AMBIGUOUS_TARGET", WarningCodes[7:9])
	}
	if WarningCodes[9] != "NODE_IDENTITY_UNAVAILABLE" || WarningCodes[10] != "NODE_CAPACITY_EVIDENCE_INCOMPLETE" {
		t.Errorf("WarningCodes[9:] = %v, want NODE_IDENTITY_UNAVAILABLE, NODE_CAPACITY_EVIDENCE_INCOMPLETE", WarningCodes[9:])
	}
	if len(NoticeCodes) != 3 {
		t.Fatalf("len(NoticeCodes) = %d, want 3", len(NoticeCodes))
	}
	if NoticeCodes[2] != "WORKLOADS_OMITTED_SCALE_ZERO" {
		t.Errorf("NoticeCodes[2] = %q, want WORKLOADS_OMITTED_SCALE_ZERO", NoticeCodes[2])
	}
	params := NoticeParams["WORKLOADS_OMITTED_SCALE_ZERO"]
	if len(params) != 1 || params[0] != "count" {
		t.Errorf("NoticeParams = %v, want [count]", params)
	}
}
