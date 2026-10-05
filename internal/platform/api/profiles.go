package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/netpolicy"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/profiles"
)

var profileLimits = map[string]int{"id": 63, "display_name": 128, "endpoint_url": 2048, "cluster_name": 253}

// profileBody is a profile as the administration reads it: the document and
// the identity of its current revision.
type profileBody struct {
	ID         string    `json:"id"`
	RevisionID uuid.UUID `json:"revision_id"`
	Revision   int       `json:"revision"`
	CreatedAt  time.Time `json:"created_at"`
	profiles.Document
}

func profileOf(c profiles.Current) profileBody {
	return profileBody{ID: c.ID, RevisionID: c.RevisionID, Revision: c.Revision, CreatedAt: c.CreatedAt.UTC(), Document: c.Document}
}

// profileProblem maps an error of the profile service to the problem that
// answers it, or nil when the error is not one of the service's refusals.
func profileProblem(err error) *Problem {
	var doc *profiles.DocumentError
	var policy *netpolicy.Error
	switch {
	case errors.Is(err, profiles.ErrForbidden):
		return &Problem{Code: CodeForbidden}
	case errors.Is(err, profiles.ErrNotFound):
		return &Problem{Code: CodeProfileNotFound}
	case errors.Is(err, profiles.ErrIDTaken):
		return &Problem{Code: CodeProfileIDTaken}
	case errors.Is(err, profiles.ErrRevisionStale):
		return &Problem{Code: CodeProfileRevisionStale}
	case errors.As(err, &doc):
		fields := make(map[string]string, len(doc.Problems))
		for _, p := range doc.Problems {
			fields[p.Field] = p.Word
		}
		return &Problem{Code: CodeProfileDocumentInvalid, Fields: fields}
	case errors.As(err, &policy):
		code := CodeEndpointURLRejected
		if policy.Code == netpolicy.CodeAddressRejected {
			code = CodeEndpointAddressRejected
		}
		return &Problem{Code: code, Fields: map[string]string{"endpoint_url": "invalid"}}
	}
	return nil
}

// failProfile answers with the problem of a service refusal, and with the
// ordinary failure path for any other error.
func (s *Server) failProfile(w http.ResponseWriter, r *http.Request, err error) {
	if p := profileProblem(err); p != nil {
		WriteProblem(w, p)
		return
	}
	s.fail(w, r, err)
}

// profileID reads the id of the path and answers 404 PROFILE_NOT_FOUND when
// it cannot name a profile.
func profileID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !profiles.ValidReferenceName(id) {
		WriteProblem(w, &Problem{Code: CodeProfileNotFound})
		return "", false
	}
	return id, true
}

func (s *Server) listProfileSummaries(w http.ResponseWriter, r *http.Request, _ auth.Authenticated) {
	summaries, err := s.Profiles.ListActive(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	list := ProfileSummaryList{Profiles: make([]ProfileSummary, 0, len(summaries))}
	for _, p := range summaries {
		list.Profiles = append(list.Profiles, ProfileSummary{
			Id: p.ID, DisplayName: p.DisplayName, AllowedNamespaces: p.AllowedNamespaces,
			PermittedPeriods: p.PermittedPeriods, DefaultPeriod: p.DefaultPeriod,
			PermittedTimezones: p.PermittedTimezones, DefaultTimezone: p.DefaultTimezone,
			PermittedReportProfiles: p.PermittedReportProfiles, DefaultReportProfile: p.DefaultReportProfile,
			PermittedMinSeverities: p.PermittedMinSeverities, DefaultMinSeverity: p.DefaultMinSeverity,
		})
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) listProfiles(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
	current, err := s.Profiles.List(r.Context(), a.Principal)
	if err != nil {
		s.failProfile(w, r, err)
		return
	}
	list := struct {
		Profiles []profileBody `json:"profiles"`
	}{make([]profileBody, 0, len(current))}
	for _, c := range current {
		list.Profiles = append(list.Profiles, profileOf(c))
	}
	writeJSON(w, http.StatusOK, list)
}

// documentOf overlays the keys of a create body on the defaults. A nil
// pointer keeps the default, and thresholds are overlaid key by key.
func documentOf(b ProfileCreate) profiles.Document {
	d := profiles.Defaults()
	d.DisplayName, d.EndpointURL, d.ClusterName, d.AllowedNamespaces = b.DisplayName, b.EndpointUrl, b.ClusterName, b.AllowedNamespaces
	set := func(dst *string, src *string) {
		if src != nil {
			*dst = *src
		}
	}
	setList := func(dst *[]string, src *[]string) {
		if src != nil {
			*dst = *src
		}
	}
	if b.Active != nil {
		d.Active = *b.Active
	}
	if b.EndpointType != nil {
		d.EndpointType = string(*b.EndpointType)
	}
	if b.TimeoutSeconds != nil {
		d.TimeoutSeconds = *b.TimeoutSeconds
	}
	if b.TlsVerify != nil {
		d.TLSVerify = *b.TlsVerify
	}
	set(&d.CredentialFile, b.CredentialFile)
	set(&d.CABundleFile, b.CaBundleFile)
	set(&d.ClusterLabel, b.ClusterLabel)
	if b.FilterMode != nil {
		d.FilterMode = string(*b.FilterMode)
	}
	set(&d.NodepoolLabel, b.NodepoolLabel)
	if b.NodeAutoscaling != nil {
		d.NodeAutoscaling = string(*b.NodeAutoscaling)
	}
	setList(&d.ExcludedContainers, b.ExcludedContainers)
	setList(&d.PermittedPeriods, b.PermittedPeriods)
	setList(&d.PermittedTimezones, b.PermittedTimezones)
	if b.PermittedReportProfiles != nil {
		d.PermittedReportProfiles = make([]string, 0, len(*b.PermittedReportProfiles))
		for _, v := range *b.PermittedReportProfiles {
			d.PermittedReportProfiles = append(d.PermittedReportProfiles, string(v))
		}
	}
	if b.PermittedMinSeverities != nil {
		d.PermittedMinSeverities = make([]string, 0, len(*b.PermittedMinSeverities))
		for _, v := range *b.PermittedMinSeverities {
			d.PermittedMinSeverities = append(d.PermittedMinSeverities, string(v))
		}
	}
	d.DefaultPeriod, d.DefaultTimezone = b.DefaultPeriod, b.DefaultTimezone
	d.DefaultReportProfile, d.DefaultMinSeverity = b.DefaultReportProfile, b.DefaultMinSeverity
	if t := b.Thresholds; t != nil {
		for dst, src := range map[*float64]*float64{
			&d.Thresholds.WarningPercent: t.WarningPercent, &d.Thresholds.CriticalPercent: t.CriticalPercent,
			&d.Thresholds.LowUtilizationPercent: t.LowUtilizationPercent, &d.Thresholds.AutoscalerNearMaxPercent: t.AutoscalerNearMaxPercent,
			&d.Thresholds.NodepoolSpreadPoints: t.NodepoolSpreadPoints, &d.Thresholds.BurstyGapPercent: t.BurstyGapPercent,
			&d.Thresholds.HPAReplicaDivergence: t.HpaReplicaDivergence,
		} {
			if src != nil {
				*dst = *src
			}
		}
	}
	return d
}

func (s *Server) createProfile(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
	var body ProfileCreate
	required := []string{"id", "display_name", "endpoint_url", "cluster_name", "allowed_namespaces"}
	if p := DecodeBody(r, &body, required, profileLimits); p != nil {
		WriteProblem(w, p)
		return
	}
	created, err := s.Profiles.Create(r.Context(), a.Principal, body.Id, documentOf(body))
	if err != nil {
		s.failProfile(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, profileOf(created))
}

func (s *Server) getProfile(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
	id, ok := profileID(w, r)
	if !ok {
		return
	}
	current, err := s.Profiles.Get(r.Context(), a.Principal, id)
	if err != nil {
		s.failProfile(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, profileOf(current))
}

func (s *Server) patchProfile(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
	id, ok := profileID(w, r)
	if !ok {
		return
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, MaxBody+1))
	if err != nil {
		WriteProblem(w, &Problem{Code: CodeRequestInvalid, Fields: map[string]string{"": "invalid"}})
		return
	}
	// DecodeBody judges the shape of the body; the map below tells an absent
	// key from an explicit null.
	r.Body = io.NopCloser(bytes.NewReader(data))
	var body ProfileChange
	if p := DecodeBody(r, &body, []string{"base_revision_id"}, profileLimits); p != nil {
		WriteProblem(w, p)
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		WriteProblem(w, &Problem{Code: CodeRequestInvalid, Fields: map[string]string{"": "invalid"}})
		return
	}
	delete(fields, "base_revision_id")
	revised, err := s.Profiles.Revise(r.Context(), a.Principal, id, profiles.Change{BaseRevisionID: body.BaseRevisionId, Fields: fields})
	if err != nil {
		s.failProfile(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, profileOf(revised))
}

func (s *Server) deleteProfile(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
	id, ok := profileID(w, r)
	if !ok {
		return
	}
	if err := s.Profiles.Delete(r.Context(), a.Principal, id); err != nil {
		s.failProfile(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) testProfile(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
	id, ok := profileID(w, r)
	if !ok {
		return
	}
	result, err := s.Profiles.Test(r.Context(), a.Principal, id)
	if err != nil {
		s.failProfile(w, r, err)
		return
	}
	capabilities := result.Capabilities
	if capabilities == nil {
		capabilities = []string{}
	}
	writeJSON(w, http.StatusOK, ProfileTestResult{Ok: result.OK, Code: result.Code, Capabilities: capabilities})
}
