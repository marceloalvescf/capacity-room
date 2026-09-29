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

	if len(collect.WarningCodes) != 7 {
		t.Fatalf("collect.WarningCodes has %d codes, want 7", len(collect.WarningCodes))
	}
	for _, code := range collect.WarningCodes {
		if !keys[Warning(code)] {
			t.Errorf("Keys() does not hold %q", Warning(code))
		}
	}

	if got := len(collect.NoticeCodes) * 2; got != 4 {
		t.Fatalf("collect.NoticeCodes yields %d entries, want 4", got)
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
