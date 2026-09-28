// Package discovery decides what a Prometheus-compatible endpoint can tell
// this report: which cluster values its metrics carry, and, in a later
// task, which capabilities it can back.
package discovery

// The confidences a Scope or a capability can carry: exact, when the
// discovered evidence names the value directly; inferred, when it was
// concluded from the absence of evidence; unknown, when there was none to
// look at.
const (
	ConfidenceExact    = "exact"
	ConfidenceInferred = "inferred"
	ConfidenceUnknown  = "unknown"
)
