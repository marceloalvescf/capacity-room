package analysis

import (
	"sort"

	"github.com/marceloalvescf/capacity-room/internal/discovery"
)

// Recommendation is one action of the report: the findings of one code
// above healthy, merged. It carries no sentence.
type Recommendation struct {
	Code       string
	Severity   string   // the highest of the merged findings
	ScopeType  string   // the scope type of the first merged finding
	Scopes     []string // the distinct scope names, sorted; never empty
	Confidence string   // the most exact of the merged findings
}

// ConfidenceRank numbers a confidence as the original does: 0 for exact,
// 1 for inferred, 2 for unknown. It returns 3 for any other text.
func ConfidenceRank(confidence string) int {
	switch confidence {
	case discovery.ConfidenceExact:
		return 0
	case discovery.ConfidenceInferred:
		return 1
	case discovery.ConfidenceUnknown:
		return 2
	default:
		return 3
	}
}

// Recommendations follows build_recommendations of the original: it
// skips the findings whose severity is healthy, groups the rest by code
// in the order the first finding of each code appears, and returns one
// Recommendation per group, sorted by severity from the highest, then
// ConfidenceRank, then the first scope name, then the code.
func Recommendations(findings []Finding) []Recommendation {
	recs := []Recommendation{}
	index := map[string]int{}
	for _, f := range findings {
		if f.Severity == SeverityHealthy {
			continue
		}
		i, ok := index[f.Code]
		if !ok {
			index[f.Code] = len(recs)
			recs = append(recs, Recommendation{
				Code:       f.Code,
				Severity:   f.Severity,
				ScopeType:  f.ScopeType,
				Scopes:     []string{f.ScopeName},
				Confidence: f.Confidence,
			})
			continue
		}
		r := &recs[i]
		if Rank(f.Severity) > Rank(r.Severity) {
			r.Severity = f.Severity
		}
		if ConfidenceRank(f.Confidence) < ConfidenceRank(r.Confidence) {
			r.Confidence = f.Confidence
		}
		r.Scopes = append(r.Scopes, f.ScopeName)
	}
	for i := range recs {
		sort.Strings(recs[i].Scopes)
		recs[i].Scopes = compact(recs[i].Scopes)
	}
	sort.SliceStable(recs, func(i, j int) bool {
		a, b := recs[i], recs[j]
		if Rank(a.Severity) != Rank(b.Severity) {
			return Rank(a.Severity) > Rank(b.Severity)
		}
		if ca, cb := ConfidenceRank(a.Confidence), ConfidenceRank(b.Confidence); ca != cb {
			return ca < cb
		}
		if a.Scopes[0] != b.Scopes[0] {
			return a.Scopes[0] < b.Scopes[0]
		}
		return a.Code < b.Code
	})
	return recs
}

// compact removes the repeated names of a sorted list.
func compact(sorted []string) []string {
	out := sorted[:1]
	for _, s := range sorted[1:] {
		if s != out[len(out)-1] {
			out = append(out, s)
		}
	}
	return out
}
