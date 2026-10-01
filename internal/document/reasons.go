package document

import (
	"fmt"
	"strings"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// evidenceNumber returns the number a finding carries under name.
func evidenceNumber(f snapshot.Finding, name string) (float64, bool) {
	for _, e := range f.Evidence {
		if e.Name == name && e.Value.Kind == "number" {
			return e.Value.Number, true
		}
	}
	return 0, false
}

// reasonFragment writes the short reason of a node finding. A usage finding
// gives the peak, or the average when it has no peak; a request finding
// gives its percentage; the two findings without a number give their summary
// without its trailing periods.
func reasonFragment(b *builder, f snapshot.Finding) string {
	resource := "cpu"
	if strings.Contains(f.Code, "_MEMORY_") {
		resource = "memory"
	}

	switch {
	case strings.HasSuffix(f.Code, "_REQUEST_HIGH"):
		if percent, ok := evidenceNumber(f, "percent"); ok {
			return b.text(catalog.Reason("request_high", resource), catalog.Params{"Percent": b.cat.Percent(percent)})
		}
	case strings.Contains(f.Code, "_USAGE_"):
		if peak, ok := evidenceNumber(f, "peak_percent"); ok {
			return b.text(catalog.Reason("usage_peak", resource), catalog.Params{"Percent": b.cat.Percent(peak)})
		}
		if average, ok := evidenceNumber(f, "average_percent"); ok {
			return b.text(catalog.Reason("usage_average", resource), catalog.Params{"Percent": b.cat.Percent(average)})
		}
	case f.Code == "NODE_OVERREQUESTED":
		return strings.TrimRight(b.text(catalog.FindingSummary("node_overrequested", ""), nil), ".")
	case f.Code == "NODE_LOW_UTILIZATION":
		return strings.TrimRight(b.text(catalog.FindingSummary("node_low_utilization", ""), nil), ".")
	}
	if b.err == nil {
		b.err = fmt.Errorf("finding %s has no reason text", f.Code)
	}
	return ""
}
