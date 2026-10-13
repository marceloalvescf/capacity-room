package analysis

import (
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
)

// valueStats builds a Statistics of state value, with average and peak
// set to the values a test cares about.
func valueStats(average, peak float64) collect.Statistics {
	return collect.Statistics{State: collect.StateValue, Average: average, Peak: peak}
}

// findEvidence returns the value evidence names carry, and whether it
// was found.
func findEvidence(evidence []Evidence, name string) (float64, bool) {
	for _, e := range evidence {
		if e.Name == name {
			return e.Value.Number, true
		}
	}
	return 0, false
}

// codeSuffix reports whether any finding of findings has a code ending
// in suffix.
func codeSuffix(findings []Finding, suffix string) bool {
	for _, f := range findings {
		if len(f.Code) >= len(suffix) && f.Code[len(f.Code)-len(suffix):] == suffix {
			return true
		}
	}
	return false
}

// TestPeakDecidesWhenHigher covers a request ratio whose peak classifies
// higher than its average.
func TestPeakDecidesWhenHigher(t *testing.T) {
	t.Parallel()
	requestPercent := valueStats(60, 80)
	unavailable := collect.Statistics{State: collect.StateUnavailable}

	findings := resourceFindings("NAMESPACE", ScopeNamespace, "payments", "", "cpu",
		unavailable, unavailable, unavailable, requestPercent, unavailable,
		discovery.StateUnavailable, false, defaultThresholds())

	if len(findings) != 1 {
		t.Fatalf("len(findings) = %d, want 1: %+v", len(findings), findings)
	}
	got := findings[0]
	if got.Code != "NAMESPACE_CPU_REQUEST_ATTENTION" {
		t.Errorf("Code = %q, want NAMESPACE_CPU_REQUEST_ATTENTION", got.Code)
	}
	if average, ok := findEvidence(got.Evidence, "average_percent"); !ok || average != 60 {
		t.Errorf("average_percent = %v, %v, want 60, true", average, ok)
	}
	if peak, ok := findEvidence(got.Evidence, "peak_percent"); !ok || peak != 80 {
		t.Errorf("peak_percent = %v, %v, want 80, true", peak, ok)
	}
}

// TestAverageDecidesWhenHigher covers a request ratio whose average
// classifies higher than its peak.
func TestAverageDecidesWhenHigher(t *testing.T) {
	t.Parallel()
	requestPercent := valueStats(90, 86)
	unavailable := collect.Statistics{State: collect.StateUnavailable}

	findings := resourceFindings("NAMESPACE", ScopeNamespace, "payments", "", "cpu",
		unavailable, unavailable, unavailable, requestPercent, unavailable,
		discovery.StateUnavailable, false, defaultThresholds())

	if len(findings) != 1 || findings[0].Code != "NAMESPACE_CPU_REQUEST_CRITICAL" {
		t.Fatalf("findings = %+v, want one NAMESPACE_CPU_REQUEST_CRITICAL", findings)
	}
}

// TestHealthyFindingIsWritten covers a request ratio classified healthy:
// the healthy finding is written, and with it, the row is also
// overrequested.
func TestHealthyFindingIsWritten(t *testing.T) {
	t.Parallel()
	requestPercent := valueStats(10, 20)
	unavailable := collect.Statistics{State: collect.StateUnavailable}

	findings := resourceFindings("NAMESPACE", ScopeNamespace, "payments", "", "cpu",
		unavailable, unavailable, unavailable, requestPercent, unavailable,
		discovery.StateUnavailable, false, defaultThresholds())

	if !codeSuffix(findings, "_REQUEST_HEALTHY") {
		t.Errorf("findings = %+v, want a code ending _REQUEST_HEALTHY", findings)
	}
	if !codeSuffix(findings, "_OVERREQUESTED") {
		t.Errorf("findings = %+v, want a code ending _OVERREQUESTED", findings)
	}
}

// TestMissingNeedsAnAvailableState covers REQUEST_MISSING: it is written
// only when the requests-and-limits state of the row is available.
func TestMissingNeedsAnAvailableState(t *testing.T) {
	t.Parallel()
	unavailable := collect.Statistics{State: collect.StateUnavailable}

	cases := []struct {
		rowState string
		want     bool
	}{
		{discovery.StateAvailable, true},
		{discovery.StatePartial, false},
		{discovery.StateUnavailable, false},
	}
	for _, c := range cases {
		findings := resourceFindings("NAMESPACE", ScopeNamespace, "payments", "", "cpu",
			unavailable, unavailable, unavailable, unavailable, unavailable,
			c.rowState, false, defaultThresholds())
		if got := codeSuffix(findings, "_REQUEST_MISSING"); got != c.want {
			t.Errorf("rowState = %q: REQUEST_MISSING present = %v, want %v", c.rowState, got, c.want)
		}
	}
}

// TestMissingIsNotZero transcribes
// test_missing_request_is_a_gap_and_not_zero_utilization of
// tests/test_capacity_analysis.py: a request with no statistic at all is
// a gap, not a ratio of zero.
func TestMissingIsNotZero(t *testing.T) {
	t.Parallel()
	unavailable := collect.Statistics{State: collect.StateUnavailable}
	usage := valueStats(0.5, 1.0)

	findings := resourceFindings("NAMESPACE", ScopeNamespace, "payments", "", "cpu",
		usage, unavailable, unavailable, unavailable, unavailable,
		discovery.StateAvailable, false, defaultThresholds())

	if !codeSuffix(findings, "_REQUEST_MISSING") {
		t.Errorf("findings = %+v, want a code ending _REQUEST_MISSING", findings)
	}
	for _, f := range findings {
		if percent, ok := findEvidence(f.Evidence, "average_percent"); ok && percent == 0 {
			t.Errorf("finding %q carries average_percent = 0, want no such evidence", f.Code)
		}
	}
}

// TestOverrequestedBoundary covers the inclusive boundary of the low
// utilization threshold.
func TestOverrequestedBoundary(t *testing.T) {
	t.Parallel()
	unavailable := collect.Statistics{State: collect.StateUnavailable}

	written := resourceFindings("NAMESPACE", ScopeNamespace, "payments", "", "cpu",
		unavailable, unavailable, unavailable, valueStats(30, 30), unavailable,
		discovery.StateUnavailable, false, defaultThresholds())
	if !codeSuffix(written, "_OVERREQUESTED") {
		t.Errorf("average 30: findings = %+v, want a code ending _OVERREQUESTED", written)
	}

	notWritten := resourceFindings("NAMESPACE", ScopeNamespace, "payments", "", "cpu",
		unavailable, unavailable, unavailable, valueStats(30.001, 30.001), unavailable,
		discovery.StateUnavailable, false, defaultThresholds())
	if codeSuffix(notWritten, "_OVERREQUESTED") {
		t.Errorf("average 30.001: findings = %+v, want no code ending _OVERREQUESTED", notWritten)
	}
}

// TestBurstyBoundary covers the inclusive boundary of the bursty gap
// threshold, and that a usage average of zero never gives a gap.
func TestBurstyBoundary(t *testing.T) {
	t.Parallel()
	unavailable := collect.Statistics{State: collect.StateUnavailable}

	cases := []struct {
		name       string
		average    float64
		peak       float64
		wantBursty bool
		wantGap    float64
		checkGap   bool
	}{
		{"gap 100", 1, 2, true, 100, true},
		{"gap 99.9", 1, 1.999, false, 0, false},
		{"zero average", 0, 5, false, 0, false},
	}
	for _, c := range cases {
		findings := resourceFindings("NAMESPACE", ScopeNamespace, "payments", "", "cpu",
			valueStats(c.average, c.peak), unavailable, unavailable, unavailable, unavailable,
			discovery.StateUnavailable, false, defaultThresholds())
		got := codeSuffix(findings, "_BURSTY")
		if got != c.wantBursty {
			t.Errorf("%s: BURSTY present = %v, want %v", c.name, got, c.wantBursty)
		}
		if c.checkGap {
			for _, f := range findings {
				if gap, ok := findEvidence(f.Evidence, "gap_percent"); ok && gap != c.wantGap {
					t.Errorf("%s: gap_percent = %v, want %v", c.name, gap, c.wantGap)
				}
			}
		}
	}
}

// TestCPUVariationNeedsOneMillicore pins the floor of the CPU variation:
// an average below 1 mCPU is not judged, an average of exactly 1 mCPU is,
// and memory keeps no floor.
func TestCPUVariationNeedsOneMillicore(t *testing.T) {
	t.Parallel()
	unavailable := collect.Statistics{State: collect.StateUnavailable}

	cases := []struct {
		name     string
		resource string
		average  float64
		peak     float64
		want     bool
	}{
		{"cpu below one millicore", "cpu", 0.0009, 0.5, false},
		{"cpu at one millicore", "cpu", 0.001, 0.5, true},
		{"memory below one byte keeps being judged", "memory", 0.0009, 0.5, true},
	}
	for _, c := range cases {
		findings := resourceFindings("NAMESPACE", ScopeNamespace, "payments", "", c.resource,
			valueStats(c.average, c.peak), unavailable, unavailable, unavailable, unavailable,
			discovery.StateUnavailable, false, defaultThresholds())
		if got := codeSuffix(findings, "_BURSTY"); got != c.want {
			t.Errorf("%s: BURSTY present = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestApproximateGivesInferred covers an approximate row with all six
// kinds of finding: confidence is inferred on the request ratio, the
// limit ratio, and OVERREQUESTED, and exact on REQUEST_MISSING,
// LIMIT_MISSING, and BURSTY.
func TestApproximateGivesInferred(t *testing.T) {
	t.Parallel()
	unavailable := collect.Statistics{State: collect.StateUnavailable}

	findings := resourceFindings("NAMESPACE", ScopeNamespace, "payments", "", "cpu",
		valueStats(1, 3), unavailable, unavailable, valueStats(10, 20), valueStats(15, 25),
		discovery.StateAvailable, true, defaultThresholds())

	inferredSuffixes := []string{"_REQUEST_HEALTHY", "_LIMIT_HEALTHY", "_OVERREQUESTED"}
	exactSuffixes := []string{"_REQUEST_MISSING", "_LIMIT_MISSING", "_BURSTY"}

	if len(findings) != len(inferredSuffixes)+len(exactSuffixes) {
		t.Fatalf("len(findings) = %d, want %d: %+v", len(findings), len(inferredSuffixes)+len(exactSuffixes), findings)
	}

	for _, suffix := range inferredSuffixes {
		found := false
		for _, f := range findings {
			if codeSuffix([]Finding{f}, suffix) {
				found = true
				if f.Confidence != discovery.ConfidenceInferred {
					t.Errorf("%s: Confidence = %q, want %q", f.Code, f.Confidence, discovery.ConfidenceInferred)
				}
			}
		}
		if !found {
			t.Errorf("no finding ending %s among %+v", suffix, findings)
		}
	}
	for _, suffix := range exactSuffixes {
		found := false
		for _, f := range findings {
			if codeSuffix([]Finding{f}, suffix) {
				found = true
				if f.Confidence != discovery.ConfidenceExact {
					t.Errorf("%s: Confidence = %q, want %q", f.Code, f.Confidence, discovery.ConfidenceExact)
				}
			}
		}
		if !found {
			t.Errorf("no finding ending %s among %+v", suffix, findings)
		}
	}
}

// TestNoBaseGivesNoRatioFinding covers a request ratio whose state is
// no_base: it gives no finding of the request ratio, and none of
// OVERREQUESTED.
func TestNoBaseGivesNoRatioFinding(t *testing.T) {
	t.Parallel()
	unavailable := collect.Statistics{State: collect.StateUnavailable}
	noBase := collect.Statistics{State: collect.StateNoBase}

	findings := resourceFindings("NAMESPACE", ScopeNamespace, "payments", "", "cpu",
		unavailable, valueStats(0, 0), unavailable, noBase, unavailable,
		discovery.StateAvailable, false, defaultThresholds())

	if codeSuffix(findings, "_REQUEST_HEALTHY") || codeSuffix(findings, "_REQUEST_ATTENTION") || codeSuffix(findings, "_REQUEST_CRITICAL") {
		t.Errorf("findings = %+v, want no request ratio finding", findings)
	}
	if codeSuffix(findings, "_OVERREQUESTED") {
		t.Errorf("findings = %+v, want no OVERREQUESTED", findings)
	}
}
