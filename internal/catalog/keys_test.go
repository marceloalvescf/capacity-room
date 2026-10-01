package catalog

import (
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
)

func TestNoticeChoosesTheEntry(t *testing.T) {
	tests := []struct {
		count float64
		want  Key
	}{
		{1, "notice.CODE.singular"},
		{2, "notice.CODE.plural"},
		{0, "notice.CODE.plural"},
		{1.0, "notice.CODE.singular"},
	}
	for _, tt := range tests {
		if got := Notice("CODE", tt.count); got != tt.want {
			t.Errorf("Notice(%q, %v) = %q, want %q", "CODE", tt.count, got, tt.want)
		}
	}
}

// newSectionConstants lists the 28 constants of the namespace label, the
// sections "Capacity by namespace" and "Resources by workload", the two
// placeholders, and the interpretation notes, so TestKeysHoldTheNewFamilies
// can check each is reachable through Keys() without re-deriving the list
// from wordingKeys itself.
var newSectionConstants = []Key{
	SectionsNamespaceLabel,
	SectionsNamespacesHeading, SectionsNamespacesEmpty, SectionsNamespacesHeaderResource,
	SectionsNamespacesHeaderAverage, SectionsNamespacesHeaderPeak, SectionsNamespacesHeaderRequest,
	SectionsNamespacesHeaderLimit, SectionsNamespacesHeaderPeakRequest, SectionsNamespacesHeaderPeakLimit,
	SectionsNamespacesRowCPU, SectionsNamespacesRowMemory,
	SectionsWorkloadsHeading, SectionsWorkloadsEmpty, SectionsWorkloadsHeaderWorkload,
	SectionsWorkloadsHeaderPods, SectionsWorkloadsHeaderCPUAverage, SectionsWorkloadsHeaderCPUPeak,
	SectionsWorkloadsHeaderCPURequest, SectionsWorkloadsHeaderCPULimit, SectionsWorkloadsHeaderMemoryAverage,
	SectionsWorkloadsHeaderMemoryPeak, SectionsWorkloadsHeaderMemoryRequest, SectionsWorkloadsHeaderMemoryLimit,
	PlaceholderUnavailable, PlaceholderNoBase,
	NotesHeading, NotesHeaderRemark,
}

func TestKeysHoldTheNewFamilies(t *testing.T) {
	if len(newSectionConstants) != 28 {
		t.Fatalf("newSectionConstants has %d entries, want 28", len(newSectionConstants))
	}

	keys := make(map[Key]bool)
	for _, k := range Keys() {
		keys[k] = true
	}

	for _, k := range newSectionConstants {
		if !keys[k] {
			t.Errorf("Keys() does not hold %q", k)
		}
	}

	if len(collect.WarningCodes) != 11 {
		t.Fatalf("collect.WarningCodes has %d codes, want 11", len(collect.WarningCodes))
	}
	for _, code := range collect.WarningCodes {
		if !keys[Warning(code)] {
			t.Errorf("Keys() does not hold %q", Warning(code))
		}
	}

	if got := len(collect.NoticeCodes) * 2; got != 6 {
		t.Fatalf("collect.NoticeCodes yields %d entries, want 6", got)
	}
	for _, code := range collect.NoticeCodes {
		if !keys[Notice(code, 1)] {
			t.Errorf("Keys() does not hold %q", Notice(code, 1))
		}
		if !keys[Notice(code, 2)] {
			t.Errorf("Keys() does not hold %q", Notice(code, 2))
		}
	}
}

// partConstants lists the constants of the sections "HPA history" and
// "Resource usage by node pool", the node states, and the undetermined
// placeholder.
var partConstants = []Key{
	SectionsHPAsHeading, SectionsHPAsIntro, SectionsHPAsObservedHeading, SectionsHPAsUncoveredHeading,
	SectionsHPAsHeaderHPA, SectionsHPAsHeaderSpecMin, SectionsHPAsHeaderSpecMax,
	SectionsHPAsHeaderCurrentAverage, SectionsHPAsHeaderMinReached, SectionsHPAsHeaderMaxReached,
	SectionsHPAsHeaderSituation, SectionsHPAsHeaderKind, SectionsHPAsHeaderWorkload,
	SectionsHPAsHeaderPods, SectionsHPAsSituationMaxReached, SectionsHPAsSituationPinned,
	SectionsHPAsSituationBelowMax, SectionsHPAsSituationNoHPA, SectionsHPAsSituationSpecificStrategy,
	SectionsHPAsEmptyNamespace, SectionsHPAsEmptyNamespacePartial, SectionsHPAsNoteIncompleteTitle,
	SectionsHPAsNoteIncomplete, SectionsHPAsNotePartialTitle, SectionsHPAsNotePartial,
	SectionsNodepoolsHeading, SectionsNodepoolsSummaryHeading, SectionsNodepoolsProfileHeading,
	SectionsNodepoolsDetailHeading, SectionsNodepoolsPoolLabel, SectionsNodepoolsHeaderPool,
	SectionsNodepoolsHeaderNodes, SectionsNodepoolsHeaderCPUAverage, SectionsNodepoolsHeaderCPUPeak,
	SectionsNodepoolsHeaderMemoryAverage, SectionsNodepoolsHeaderMemoryPeak,
	SectionsNodepoolsHeaderConfidence, SectionsNodepoolsHeaderCPUAllocatable,
	SectionsNodepoolsHeaderCPUUsed, SectionsNodepoolsHeaderCPURequested,
	SectionsNodepoolsHeaderMemoryAllocatable, SectionsNodepoolsHeaderMemoryUsed,
	SectionsNodepoolsHeaderMemoryRequested, SectionsNodepoolsHeaderNode,
	SectionsNodepoolsHeaderCPUAvgPeak, SectionsNodepoolsHeaderMemoryAvgPeak,
	SectionsNodepoolsHeaderReason, SectionsNodepoolsHeaderState, SectionsNodepoolsEmptySummary,
	SectionsNodepoolsEmptyProfile, SectionsNodepoolsEmptyDetail, SectionsNodepoolsNoFinding,
	NodeStateReady, NodeStateNotReady, NodeStateSchedulable, NodeStateUnschedulable,
	PlaceholderUndetermined,
}

func TestKeysHoldThePartFamilies(t *testing.T) {
	if len(partConstants) != 57 {
		t.Fatalf("partConstants has %d entries, want 57", len(partConstants))
	}

	keys := make(map[Key]bool)
	for _, k := range Keys() {
		keys[k] = true
	}
	for _, k := range partConstants {
		if !keys[k] {
			t.Errorf("Keys() does not hold %q", k)
		}
	}

	want := []Key{
		"confidence.exact", "confidence.inferred", "confidence.unknown",
		"reason.usage_peak.cpu", "reason.usage_peak.memory",
		"reason.usage_average.cpu", "reason.usage_average.memory",
		"reason.request_high.cpu", "reason.request_high.memory",
		"finding.node_overrequested.summary", "finding.node_low_utilization.summary",
	}
	for _, k := range want {
		if !keys[k] {
			t.Errorf("Keys() does not hold %q", k)
		}
	}
	if Confidence("exact") != "confidence.exact" {
		t.Errorf("Confidence(exact) = %q", Confidence("exact"))
	}
	if Reason("usage_peak", "cpu") != "reason.usage_peak.cpu" {
		t.Errorf("Reason(usage_peak, cpu) = %q", Reason("usage_peak", "cpu"))
	}
	if FindingSummary("node_overrequested") != "finding.node_overrequested.summary" {
		t.Errorf("FindingSummary(node_overrequested) = %q", FindingSummary("node_overrequested"))
	}
}
