package catalog

import (
	"strings"
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

// prefixed returns the keys of set that start with prefix.
func prefixed(set map[Key]bool, prefix string) []Key {
	var found []Key
	for k := range set {
		if strings.HasPrefix(string(k), prefix) {
			found = append(found, k)
		}
	}
	return found
}

func TestRiskCountChoosesTheEntry(t *testing.T) {
	tests := []struct {
		severity string
		count    float64
		want     Key
	}{
		{"critical", 1, "risk_status.critical.singular"},
		{"critical", 2, "risk_status.critical.plural"},
		{"attention", 0, "risk_status.attention.plural"},
	}
	for _, tt := range tests {
		if got := RiskCount(tt.severity, tt.count); got != tt.want {
			t.Errorf("RiskCount(%q, %v) = %q, want %q", tt.severity, tt.count, got, tt.want)
		}
	}
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

	var executive int
	for _, k := range Keys() {
		if strings.HasPrefix(string(k), "sections.executive.") {
			executive++
		}
	}
	if executive != 42 {
		t.Errorf("Keys() holds %d executive keys, want 42", executive)
	}
	for _, k := range []Key{PlaceholderEmptyCell, SectionsExecutiveHeading, SectionsExecutiveActiveReplicasSingular, SectionsExecutiveActiveReplicasPlural} {
		if !keys[k] {
			t.Errorf("Keys() does not hold %q", k)
		}
	}
	for _, group := range []struct {
		name  string
		count int
		want  int
		keys  []Key
	}{
		{"priorities", len(Priorities), 2, prefixed(keys, "priority.")},
		{"risk counts", len(RiskSeverities) * 2, 4, prefixed(keys, "risk_status.")},
		{"summaries", len(ScopeSummaries), 4, prefixed(keys, "scope_summary.")},
		{"reasons", len(ReasonFamilies) * len(ReasonResources), 20, prefixed(keys, "reason.")},
	} {
		if group.count != group.want {
			t.Errorf("%s: lists give %d keys, want %d", group.name, group.count, group.want)
		}
		if group.keys != nil && len(group.keys) != group.want {
			t.Errorf("Keys() holds %d %s, want %d", len(group.keys), group.name, group.want)
		}
	}
	for _, family := range []string{"request_peak", "request_average", "limit_peak", "limit_average", "request_missing", "limit_missing", "bursty"} {
		for _, resource := range ReasonResources {
			if !keys[Reason(family, resource)] {
				t.Errorf("Keys() does not hold %q", Reason(family, resource))
			}
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
	if FindingSummary("node_overrequested", "") != "finding.node_overrequested.summary" {
		t.Errorf("FindingSummary(node_overrequested) = %q", FindingSummary("node_overrequested", ""))
	}
}

func TestFindingKeyFunctions(t *testing.T) {
	tests := []struct {
		got, want Key
	}{
		{FindingSummary("resource_bursty", "cpu"), "finding.resource_bursty.cpu.summary"},
		{FindingAction("resource_bursty", "memory"), "finding.resource_bursty.memory.action"},
		{FindingSummary("hpa_pinned", ""), "finding.hpa_pinned.summary"},
		{FindingObservation("node_usage", "cpu"), "finding.node_usage.cpu.observation"},
		{FindingObservation("nodepool_unknown", ""), "finding.nodepool_unknown.observation"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("got %q, want %q", tt.got, tt.want)
		}
	}
}

func TestKeysHoldTheFindingKeys(t *testing.T) {
	count := 0
	for _, k := range Keys() {
		if strings.HasPrefix(string(k), "finding.") {
			count++
		}
	}
	if count != 75 {
		t.Errorf("Keys() holds %d finding keys, want 75", count)
	}
}
