package profiles

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/jackc/pgx/v5"

	"github.com/marceloalvescf/capacity-room/internal/platform/netpolicy"
	"github.com/marceloalvescf/capacity-room/internal/platform/store"
)

// The actions an import reports.
const (
	OutcomeCreated   = "created"
	OutcomeRevised   = "revised"
	OutcomeUnchanged = "unchanged"
	OutcomeRecreated = "recreated"
)

// ImportFile is the file of admin import-profiles.
type ImportFile struct {
	Profiles []ImportEntry `toml:"profiles"`
}

// ImportThresholds holds the thresholds a profile of the file sets; a key
// left out keeps its default.
type ImportThresholds struct {
	WarningPercent           *float64 `toml:"warning_percent"`
	CriticalPercent          *float64 `toml:"critical_percent"`
	LowUtilizationPercent    *float64 `toml:"low_utilization_percent"`
	AutoscalerNearMaxPercent *float64 `toml:"autoscaler_near_max_percent"`
	NodepoolSpreadPoints     *float64 `toml:"nodepool_spread_points"`
	BurstyGapPercent         *float64 `toml:"bursty_gap_percent"`
	HPAReplicaDivergence     *float64 `toml:"hpa_replica_divergence"`
}

// ImportEntry is one profile of the file: its id and its document, under
// the JSON names of the document as TOML keys. A key left out keeps the
// default of Defaults. Document is filled by ReadImportFile.
type ImportEntry struct {
	ID                      string            `toml:"id"`
	DisplayName             *string           `toml:"display_name"`
	Active                  *bool             `toml:"active"`
	EndpointURL             *string           `toml:"endpoint_url"`
	EndpointType            *string           `toml:"endpoint_type"`
	TimeoutSeconds          *float64          `toml:"timeout_seconds"`
	TLSVerify               *bool             `toml:"tls_verify"`
	CredentialFile          *string           `toml:"credential_file"`
	CABundleFile            *string           `toml:"ca_bundle_file"`
	ClusterName             *string           `toml:"cluster_name"`
	ClusterLabel            *string           `toml:"cluster_label"`
	FilterMode              *string           `toml:"filter_mode"`
	NodepoolLabel           *string           `toml:"nodepool_label"`
	NodeAutoscaling         *string           `toml:"node_autoscaling"`
	AllowedNamespaces       *[]string         `toml:"allowed_namespaces"`
	ExcludedContainers      *[]string         `toml:"excluded_containers"`
	PermittedPeriods        *[]string         `toml:"permitted_periods"`
	DefaultPeriod           *string           `toml:"default_period"`
	PermittedTimezones      *[]string         `toml:"permitted_timezones"`
	DefaultTimezone         *string           `toml:"default_timezone"`
	PermittedReportProfiles *[]string         `toml:"permitted_report_profiles"`
	DefaultReportProfile    *string           `toml:"default_report_profile"`
	Thresholds              *ImportThresholds `toml:"thresholds"`

	Document Document `toml:"-"`
}

// overlay sets *dst to *src when the key was present.
func overlay[T any](dst *T, src *T) {
	if src != nil {
		*dst = *src
	}
}

// document builds the document of the entry over the defaults.
func (e ImportEntry) document() Document {
	d := Defaults()
	overlay(&d.DisplayName, e.DisplayName)
	overlay(&d.Active, e.Active)
	overlay(&d.EndpointURL, e.EndpointURL)
	overlay(&d.EndpointType, e.EndpointType)
	overlay(&d.TimeoutSeconds, e.TimeoutSeconds)
	overlay(&d.TLSVerify, e.TLSVerify)
	overlay(&d.CredentialFile, e.CredentialFile)
	overlay(&d.CABundleFile, e.CABundleFile)
	overlay(&d.ClusterName, e.ClusterName)
	overlay(&d.ClusterLabel, e.ClusterLabel)
	overlay(&d.FilterMode, e.FilterMode)
	overlay(&d.NodepoolLabel, e.NodepoolLabel)
	overlay(&d.NodeAutoscaling, e.NodeAutoscaling)
	overlay(&d.AllowedNamespaces, e.AllowedNamespaces)
	overlay(&d.ExcludedContainers, e.ExcludedContainers)
	overlay(&d.PermittedPeriods, e.PermittedPeriods)
	overlay(&d.PermittedTimezones, e.PermittedTimezones)
	overlay(&d.PermittedReportProfiles, e.PermittedReportProfiles)
	d.DefaultPeriod = e.DefaultPeriod
	d.DefaultTimezone = e.DefaultTimezone
	d.DefaultReportProfile = e.DefaultReportProfile
	if t := e.Thresholds; t != nil {
		overlay(&d.Thresholds.WarningPercent, t.WarningPercent)
		overlay(&d.Thresholds.CriticalPercent, t.CriticalPercent)
		overlay(&d.Thresholds.LowUtilizationPercent, t.LowUtilizationPercent)
		overlay(&d.Thresholds.AutoscalerNearMaxPercent, t.AutoscalerNearMaxPercent)
		overlay(&d.Thresholds.NodepoolSpreadPoints, t.NodepoolSpreadPoints)
		overlay(&d.Thresholds.BurstyGapPercent, t.BurstyGapPercent)
		overlay(&d.Thresholds.HPAReplicaDivergence, t.HPAReplicaDivergence)
	}
	return d
}

// ReadImportFile parses and validates the file whole: an unknown key, an
// inline credential key, a repeated id, an invalid document, or a refused
// endpoint is an error that names the profile and the field, never a value.
// Each returned entry carries its finished document, with the canonical
// endpoint.
func ReadImportFile(path string, p netpolicy.Policy) ([]ImportEntry, error) {
	var file ImportFile
	md, err := toml.DecodeFile(path, &file)
	if err != nil {
		return nil, decodeImportError(path, err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return nil, fmt.Errorf("%s: unknown key %s", path, undecoded[0])
	}
	seen := make(map[string]bool, len(file.Profiles))
	for i, e := range file.Profiles {
		if !ValidReferenceName(e.ID) {
			return nil, fmt.Errorf("%s: profile %d: id invalid", path, i+1)
		}
		if seen[e.ID] {
			return nil, fmt.Errorf("%s: profile %s is repeated", path, e.ID)
		}
		seen[e.ID] = true
	}
	for i := range file.Profiles {
		e := &file.Profiles[i]
		d := e.document()
		if problems := d.Validate(); len(problems) > 0 {
			return nil, fmt.Errorf("%s: profile %s: %s %s", path, e.ID, problems[0].Field, problems[0].Word)
		}
		canonical, err := CanonicalEndpoint(p, d.EndpointURL, d.EndpointType)
		if err != nil {
			return nil, fmt.Errorf("%s: profile %s: endpoint_url invalid", path, e.ID)
		}
		d.EndpointURL = canonical
		e.Document = d
	}
	return file.Profiles, nil
}

// decodeImportError words a failure of toml.DecodeFile without repeating any
// value of the file.
func decodeImportError(path string, err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return fmt.Errorf("%s: cannot be read: %s", path, pathErr.Err)
	}
	var parseErr toml.ParseError
	if errors.As(err, &parseErr) {
		return fmt.Errorf("%s: is not valid TOML: %d:%d", path, parseErr.Position.Line, parseErr.Position.Col)
	}
	return fmt.Errorf("%s: has a value of the wrong type", path)
}

// Outcome says what the import did with one profile.
type Outcome struct {
	ID     string
	Action string // created, recreated, revised, unchanged
}

// Import creates or revises the profiles in one transaction, with no actor.
// A profile whose document equals its current one is left alone, and a
// profile the entries do not name is never touched. The profiles are
// written in id order, so that two imports lock their rows in one order;
// the outcomes follow the order of entries.
func (s *Service) Import(ctx context.Context, entries []ImportEntry) ([]Outcome, error) {
	now := s.now()
	order := make([]int, len(entries))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(a, b int) int { return strings.Compare(entries[a].ID, entries[b].ID) })
	out := make([]Outcome, len(entries))
	err := store.Transact(ctx, s.Pool, func(tx pgx.Tx) error {
		for _, i := range order {
			action, err := s.importOne(ctx, tx, entries[i], now)
			if err != nil {
				return err
			}
			out[i] = Outcome{ID: entries[i].ID, Action: action}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) importOne(ctx context.Context, tx pgx.Tx, e ImportEntry, now time.Time) (string, error) {
	newTree, newJSON, err := documentTree(e.Document)
	if err != nil {
		return "", err
	}
	p, err := store.GetOrCreateProfile(ctx, tx, e.ID, now)
	if err != nil {
		return "", err
	}
	if p.CurrentRevisionID == nil {
		// A profile whose pointer was cleared keeps its revisions, and
		// the import returns it.
		_, recreated, err := createStep(ctx, tx, nil, e.ID, e.Document, newTree, now)
		if err != nil {
			return "", err
		}
		if recreated {
			return OutcomeRecreated, nil
		}
		return OutcomeCreated, nil
	}
	cur, err := store.GetProfileRevision(ctx, tx, *p.CurrentRevisionID)
	if err != nil {
		return "", notFound(err)
	}
	before, err := currentOf(cur)
	if err != nil {
		return "", err
	}
	oldTree, oldJSON, err := documentTree(before.Document)
	if err != nil {
		return "", err
	}
	if bytes.Equal(oldJSON, newJSON) {
		return OutcomeUnchanged, nil
	}
	_, err = reviseStep(ctx, tx, nil, e.ID, cur, e.Document, oldTree, newTree, now)
	return OutcomeRevised, err
}
