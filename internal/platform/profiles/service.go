package profiles

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/audit"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/netpolicy"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/settings"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
)

// The audit actions of the profiles.
const (
	ActionProfileCreate     = "profile.create"
	ActionProfileRevise     = "profile.revise"
	ActionProfileActivate   = "profile.activate"
	ActionProfileDeactivate = "profile.deactivate"
	ActionProfileDelete     = "profile.delete"
	ActionProfileTest       = "profile.test"
)

const targetProfile = "profile"

// The refusals of the service.
var (
	ErrNotFound      = errors.New("profile not found")
	ErrIDTaken       = errors.New("profile id taken")
	ErrRevisionStale = errors.New("profile revision stale")
	ErrForbidden     = auth.ErrForbidden
)

// DocumentError carries the problems of a refused document.
type DocumentError struct{ Problems []Problem }

// Error returns a fixed sentence.
func (e *DocumentError) Error() string { return "profile document refused" }

// Service holds the profiles of the platform.
type Service struct {
	Pool     *pgxpool.Pool
	Policy   netpolicy.Policy
	Settings settings.Settings // the two folders
	Now      func() time.Time
	Resolve  netpolicy.Resolver // nil means the system resolver
}

// Current is a profile as the administration sees it.
type Current struct {
	ID         string
	RevisionID uuid.UUID
	Revision   int
	Document   Document
	CreatedAt  time.Time
}

// Summary is a profile as every signed-in reader sees it.
type Summary struct {
	ID                      string
	DisplayName             string
	AllowedNamespaces       []string
	PermittedPeriods        []string
	DefaultPeriod           string
	PermittedTimezones      []string
	DefaultTimezone         string
	PermittedReportProfiles []string
	DefaultReportProfile    string
	PermittedMinSeverities  []string
	DefaultMinSeverity      string
}

// Change is the body of a revise: the revision it was written against and
// the fields present, by JSON name. A JSON null clears a nullable default.
type Change struct {
	BaseRevisionID uuid.UUID
	Fields         map[string]json.RawMessage
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func requireAdministrator(actor auth.Principal) error {
	if actor.Role != auth.RoleAdministrator {
		return ErrForbidden
	}
	return nil
}

func currentOf(r store.ProfileRevision) (Current, error) {
	var d Document
	if err := json.Unmarshal(r.Document, &d); err != nil {
		return Current{}, fmt.Errorf("decode profile document: %w", err)
	}
	return Current{ID: r.ProfileID, RevisionID: r.ID, Revision: r.Revision, Document: d, CreatedAt: r.CreatedAt}, nil
}

func notFound(err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

// documentTree is the canonical JSON of d as a map, for comparing fields.
func documentTree(d Document) (map[string]any, []byte, error) {
	raw, err := CanonicalJSON(d)
	if err != nil {
		return nil, nil, err
	}
	var tree map[string]any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, nil, err
	}
	return tree, raw, nil
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func revisionRow(id string, d Document, parent *uuid.UUID, actor *uuid.UUID, now time.Time) (store.ProfileRevision, error) {
	raw, err := CanonicalJSON(d)
	if err != nil {
		return store.ProfileRevision{}, err
	}
	return store.ProfileRevision{
		ID:             uuid.New(),
		ProfileID:      id,
		Fingerprint:    Fingerprint(d, parent),
		DisplayName:    d.DisplayName,
		Active:         d.Active,
		Document:       raw,
		CredentialFile: d.CredentialFile,
		CABundleFile:   d.CABundleFile,
		CreatedBy:      actor,
		CreatedAt:      now,
	}, nil
}

// checkDocument validates d and replaces its endpoint by the canonical one.
func (s *Service) checkDocument(d *Document) error {
	if problems := d.Validate(); len(problems) > 0 {
		return &DocumentError{Problems: problems}
	}
	canonical, err := CanonicalEndpoint(s.Policy, d.EndpointURL, d.EndpointType)
	if err != nil {
		return err
	}
	d.EndpointURL = canonical
	return nil
}

// Create stores d as the first revision of the profile id.
func (s *Service) Create(ctx context.Context, actor auth.Principal, id string, d Document) (Current, error) {
	if err := requireAdministrator(actor); err != nil {
		return Current{}, err
	}
	if !ValidReferenceName(id) {
		return Current{}, &DocumentError{Problems: []Problem{{Field: "id", Word: "invalid"}}}
	}
	if err := s.checkDocument(&d); err != nil {
		return Current{}, err
	}
	tree, _, err := documentTree(d)
	if err != nil {
		return Current{}, err
	}
	now := s.now()
	var out Current
	err = store.Transact(ctx, s.Pool, func(tx pgx.Tx) error {
		p, err := store.GetOrCreateProfile(ctx, tx, id, now)
		if err != nil {
			return err
		}
		if p.CurrentRevisionID != nil {
			return ErrIDTaken
		}
		out, err = createStep(ctx, tx, &actor.ID, id, d, tree, now)
		return err
	})
	if err != nil {
		return Current{}, err
	}
	return out, nil
}

// createStep writes d as the first revision of the profile id, which the
// caller has locked and found without a current revision.
func createStep(ctx context.Context, tx pgx.Tx, actor *uuid.UUID, id string, d Document, tree map[string]any, now time.Time) (Current, error) {
	var parent *uuid.UUID
	latest, err := store.LatestProfileRevision(ctx, tx, id)
	switch {
	case err == nil:
		parent = &latest.ID
	case !errors.Is(err, store.ErrNotFound):
		return Current{}, err
	}
	row, err := revisionRow(id, d, parent, actor, now)
	if err != nil {
		return Current{}, err
	}
	got, err := store.AppendProfileRevision(ctx, tx, row)
	if err != nil {
		return Current{}, err
	}
	if err := store.SetCurrentRevision(ctx, tx, id, &got.ID, now); err != nil {
		return Current{}, err
	}
	out, err := currentOf(got)
	if err != nil {
		return Current{}, err
	}
	return out, audit.Append(ctx, tx, now, audit.Event{
		ActorID: actor, Action: ActionProfileCreate, TargetType: targetProfile, TargetID: id,
		Metadata: audit.Metadata{"revision_id": got.ID.String(), "revision": got.Revision, "fields": sortedKeys(tree)},
	})
}

// reviseStep writes merged as the revision that follows cur, whose document
// tree is oldTree, and audits it.
func reviseStep(ctx context.Context, tx pgx.Tx, actor *uuid.UUID, id string, cur store.ProfileRevision, merged Document, oldTree, newTree map[string]any, now time.Time) (Current, error) {
	var changed []string
	for _, k := range sortedKeys(newTree) {
		if !reflect.DeepEqual(oldTree[k], newTree[k]) {
			changed = append(changed, k)
		}
	}
	row, err := revisionRow(id, merged, &cur.ID, actor, now)
	if err != nil {
		return Current{}, err
	}
	got, err := store.AppendProfileRevision(ctx, tx, row)
	if err != nil {
		return Current{}, err
	}
	if err := store.SetCurrentRevision(ctx, tx, id, &got.ID, now); err != nil {
		return Current{}, err
	}
	out, err := currentOf(got)
	if err != nil {
		return Current{}, err
	}
	action := ActionProfileRevise
	if slices.Contains(changed, "active") {
		action = ActionProfileDeactivate
		if merged.Active {
			action = ActionProfileActivate
		}
	}
	return out, audit.Append(ctx, tx, now, audit.Event{
		ActorID: actor, Action: action, TargetType: targetProfile, TargetID: id,
		Metadata: audit.Metadata{
			"revision_id": got.ID.String(), "revision": got.Revision,
			"base_revision_id": cur.ID.String(), "fields": changed,
		},
	})
}

var documentFields = func() map[string]int {
	t := reflect.TypeOf(Document{})
	m := make(map[string]int, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		name, _, _ := bytes.Cut([]byte(t.Field(i).Tag.Get("json")), []byte(","))
		m[string(name)] = i
	}
	return m
}()

// apply overlays the present fields on d. It returns the problems of the
// fields it cannot read.
func apply(d *Document, fields map[string]json.RawMessage) []Problem {
	var problems []Problem
	v := reflect.ValueOf(d).Elem()
	for _, key := range sortedRawKeys(fields) {
		idx, ok := documentFields[key]
		if !ok {
			problems = append(problems, Problem{Field: key, Word: "invalid"})
			continue
		}
		field := v.Field(idx)
		raw := fields[key]
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			if field.Kind() != reflect.Pointer {
				problems = append(problems, Problem{Field: key, Word: "invalid"})
				continue
			}
			field.SetZero()
			continue
		}
		if field.Kind() != reflect.Struct {
			field.SetZero()
		} else if holdsNull(raw) {
			problems = append(problems, Problem{Field: key, Word: "invalid"})
			continue
		}
		if err := json.Unmarshal(raw, field.Addr().Interface()); err != nil {
			problems = append(problems, Problem{Field: key, Word: "invalid"})
		}
	}
	return problems
}

// holdsNull reports whether raw is an object with a null value: a key of
// an overlaid object is set or left out, never cleared.
func holdsNull(raw json.RawMessage) bool {
	var members map[string]json.RawMessage
	if json.Unmarshal(raw, &members) != nil {
		return false
	}
	for _, value := range members {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return true
		}
	}
	return false
}

func sortedRawKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Revise appends a revision made of the current document and the fields
// present in c, when that changes the document.
func (s *Service) Revise(ctx context.Context, actor auth.Principal, id string, c Change) (Current, error) {
	if err := requireAdministrator(actor); err != nil {
		return Current{}, err
	}
	now := s.now()
	var out Current
	err := store.Transact(ctx, s.Pool, func(tx pgx.Tx) error {
		p, err := store.GetProfileForUpdate(ctx, tx, id)
		if err != nil {
			return notFound(err)
		}
		if p.CurrentRevisionID == nil {
			return ErrNotFound
		}
		if *p.CurrentRevisionID != c.BaseRevisionID {
			return ErrRevisionStale
		}
		cur, err := store.GetProfileRevision(ctx, tx, *p.CurrentRevisionID)
		if err != nil {
			return notFound(err)
		}
		before, err := currentOf(cur)
		if err != nil {
			return err
		}
		merged := before.Document
		if problems := apply(&merged, c.Fields); len(problems) > 0 {
			return &DocumentError{Problems: problems}
		}
		if problems := merged.Validate(); len(problems) > 0 {
			return &DocumentError{Problems: problems}
		}
		_, endpointURL := c.Fields["endpoint_url"]
		_, endpointType := c.Fields["endpoint_type"]
		if endpointURL || endpointType {
			canonical, err := CanonicalEndpoint(s.Policy, merged.EndpointURL, merged.EndpointType)
			if err != nil {
				return err
			}
			merged.EndpointURL = canonical
		}
		oldTree, oldJSON, err := documentTree(before.Document)
		if err != nil {
			return err
		}
		newTree, newJSON, err := documentTree(merged)
		if err != nil {
			return err
		}
		if bytes.Equal(oldJSON, newJSON) {
			out = before
			return nil
		}
		out, err = reviseStep(ctx, tx, &actor.ID, id, cur, merged, oldTree, newTree, now)
		return err
	})
	if err != nil {
		return Current{}, err
	}
	return out, nil
}

// Delete clears the pointer of the profile; its revisions remain.
func (s *Service) Delete(ctx context.Context, actor auth.Principal, id string) error {
	if err := requireAdministrator(actor); err != nil {
		return err
	}
	now := s.now()
	return store.Transact(ctx, s.Pool, func(tx pgx.Tx) error {
		p, err := store.GetProfileForUpdate(ctx, tx, id)
		if err != nil {
			return notFound(err)
		}
		if p.CurrentRevisionID == nil {
			return ErrNotFound
		}
		if err := store.SetCurrentRevision(ctx, tx, id, nil, now); err != nil {
			return err
		}
		return audit.Append(ctx, tx, now, audit.Event{
			ActorID: &actor.ID, Action: ActionProfileDelete, TargetType: targetProfile, TargetID: id,
			Metadata: audit.Metadata{"revision_id": p.CurrentRevisionID.String()},
		})
	})
}

// Get reads the current revision of a profile.
func (s *Service) Get(ctx context.Context, actor auth.Principal, id string) (Current, error) {
	if err := requireAdministrator(actor); err != nil {
		return Current{}, err
	}
	r, err := store.GetCurrentRevision(ctx, s.Pool, id)
	if err != nil {
		return Current{}, notFound(err)
	}
	return currentOf(r)
}

// List reads every profile that has a current revision, in id order.
func (s *Service) List(ctx context.Context, actor auth.Principal) ([]Current, error) {
	if err := requireAdministrator(actor); err != nil {
		return nil, err
	}
	rows, err := store.ListCurrentRevisions(ctx, s.Pool)
	if err != nil {
		return nil, err
	}
	out := make([]Current, 0, len(rows))
	for _, r := range rows {
		c, err := currentOf(r)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// ListActive answers the summaries of the active profiles, for any reader.
func (s *Service) ListActive(ctx context.Context) ([]Summary, error) {
	rows, err := store.ListCurrentRevisions(ctx, s.Pool)
	if err != nil {
		return nil, err
	}
	out := []Summary{}
	for _, r := range rows {
		c, err := currentOf(r)
		if err != nil {
			return nil, err
		}
		d := c.Document
		if !d.Active {
			continue
		}
		period, zone, profile, severity := d.ResolvedDefaults()
		out = append(out, Summary{
			ID: c.ID, DisplayName: d.DisplayName, AllowedNamespaces: d.AllowedNamespaces,
			PermittedPeriods: d.PermittedPeriods, DefaultPeriod: period,
			PermittedTimezones: d.PermittedTimezones, DefaultTimezone: zone,
			PermittedReportProfiles: d.PermittedReportProfiles, DefaultReportProfile: profile,
			PermittedMinSeverities: d.PermittedMinSeverities, DefaultMinSeverity: severity,
		})
	}
	return out, nil
}
