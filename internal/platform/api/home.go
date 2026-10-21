package api

import (
	"net/http"

	"github.com/marceloalvescf/capacity-room/internal/platform/auth"
	"github.com/marceloalvescf/capacity-room/internal/platform/reports"
)

// home answers, for each active profile, its newest report and its newest
// collected report that has not expired.
func (s *Server) home(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
	active, err := s.Profiles.ListActive(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	clusters, err := s.Reports.Home(r.Context(), a.Principal, active)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := Home{Clusters: make([]HomeCluster, len(clusters))}
	for i, c := range clusters {
		out.Clusters[i] = HomeCluster{
			ProfileId: c.ProfileID, DisplayName: c.DisplayName,
			Latest: summaryPointer(c.Latest), Newest: summaryPointer(c.Newest),
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func summaryPointer(s *reports.Summary) *ReportSummary {
	if s == nil {
		return nil
	}
	out := reportSummaryOf(*s)
	return &out
}
