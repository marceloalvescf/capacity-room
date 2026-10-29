package profiles

import (
	"context"
	"slices"
	"time"

	"github.com/marceloalvescf/capacity-room/internal/discovery"
	"github.com/marceloalvescf/capacity-room/internal/platform/audit"
	"github.com/marceloalvescf/capacity-room/internal/platform/auth"
	"github.com/marceloalvescf/capacity-room/internal/platform/netpolicy"
	"github.com/marceloalvescf/capacity-room/internal/platform/store"
	"github.com/marceloalvescf/capacity-room/internal/prom"
)

// The codes of a profile test.
const (
	CodeOK                    = "PROFILE_OK"
	CodeCredentialUnavailable = "PROFILE_CREDENTIAL_UNAVAILABLE"
	CodeCABundleUnavailable   = "PROFILE_CA_BUNDLE_UNAVAILABLE"
	CodeTLSFailed             = "ENDPOINT_TLS_FAILED"
	CodeAuthRejected          = "ENDPOINT_AUTH_REJECTED"
	CodeAPINotFound           = "ENDPOINT_API_NOT_FOUND"
	CodeUnreachable           = "ENDPOINT_UNREACHABLE"
	CodeScopeNotFound         = "CLUSTER_SCOPE_NOT_FOUND"
	CodeCapabilities          = "CAPABILITIES_INSUFFICIENT"
)

// TestCodes lists every code of a profile test, in the order of the contract.
var TestCodes = []string{
	CodeOK,
	string(netpolicy.CodeURLRejected),
	string(netpolicy.CodeAddressRejected),
	string(netpolicy.CodeResolutionFailed),
	string(netpolicy.CodeConnectionRejected),
	string(netpolicy.CodeRedirectRejected),
	CodeCredentialUnavailable,
	CodeCABundleUnavailable,
	CodeTLSFailed,
	CodeAuthRejected,
	CodeAPINotFound,
	CodeUnreachable,
	CodeScopeNotFound,
	CodeCapabilities,
}

// TestResult is the answer of a profile test.
type TestResult struct {
	OK           bool
	Code         string
	Capabilities []string // the names whose state is not unavailable, in their order
	Namespaces   []string // the namespaces with pods at the source, sorted, with no limit applied; empty when not ok
}

// Test resolves the references, runs the preflight through the policy
// transport, and answers a result; it returns an error only for the
// database, a missing profile, or a failure that is not the source's.
func (s *Service) Test(ctx context.Context, actor auth.Principal, id string) (TestResult, error) {
	if err := requireAdministrator(actor); err != nil {
		return TestResult{}, err
	}
	rev, err := store.GetCurrentRevision(ctx, s.Pool, id)
	if err != nil {
		return TestResult{}, notFound(err)
	}
	cur, err := currentOf(rev)
	if err != nil {
		return TestResult{}, err
	}
	d := cur.Document

	result, err := s.runTest(ctx, d, true)
	if err != nil {
		return TestResult{}, err
	}
	outcome := audit.OutcomeSucceeded
	if !result.OK {
		outcome = audit.OutcomeFailed
	}
	// The requests have left the host whatever the caller does now, so the
	// row is written even when the request context has ended.
	err = audit.Append(context.WithoutCancel(ctx), s.Pool, s.now(), audit.Event{
		ActorID: &actor.ID, Action: ActionProfileTest, TargetType: targetProfile, TargetID: id, Outcome: outcome,
		Metadata: audit.Metadata{"code": result.Code, "revision_id": rev.ID.String()},
	})
	if err != nil {
		return TestResult{}, err
	}
	return result, nil
}

// TestDraft runs the same preflight against a document that is not stored. A
// non-empty id names the profile the draft belongs to in the audit row. It
// reads and writes no profile row.
func (s *Service) TestDraft(ctx context.Context, actor auth.Principal, id string, d Document) (TestResult, error) {
	if err := requireAdministrator(actor); err != nil {
		return TestResult{}, err
	}
	if id != "" && !ValidReferenceName(id) {
		return TestResult{}, &DocumentError{Problems: []Problem{{Field: "id", Word: "invalid"}}}
	}
	if err := s.checkDocument(&d); err != nil {
		return TestResult{}, err
	}
	result, err := s.runTest(ctx, d, true)
	if err != nil {
		return TestResult{}, err
	}
	outcome := audit.OutcomeSucceeded
	if !result.OK {
		outcome = audit.OutcomeFailed
	}
	event := audit.Event{
		ActorID: &actor.ID, Action: ActionProfileTest, Outcome: outcome,
		Metadata: audit.Metadata{"code": result.Code, "draft": true},
	}
	if id != "" {
		event.TargetType, event.TargetID = targetProfile, id
	}
	// The requests have left the host whatever the caller does now.
	if err := audit.Append(context.WithoutCancel(ctx), s.Pool, s.now(), event); err != nil {
		return TestResult{}, err
	}
	return result, nil
}

func failure(code string) TestResult { return TestResult{Code: code} }

// testCeiling bounds a profile test whatever the profile's timeout.
const testCeiling = 120 * time.Second

// runTest holds no database connection: it reads files and sends requests.
// With detect it runs the whole preflight, as a test does; without it, it
// connects and resolves the scope alone, which is all that listing the
// namespaces needs, and answers no capability.
func (s *Service) runTest(ctx context.Context, d Document, detect bool) (TestResult, error) {
	period, zone, profile := d.ResolvedDefaults()
	cfg := d.RunConfig(period, "", "", zone, profile, d.AllowedNamespaces)
	opts, err := s.Source(d, cfg.Metrics)
	if err != nil {
		return resultOf(ctx, err)
	}
	ctx, cancel := context.WithTimeout(ctx, min(time.Duration(3*d.TimeoutSeconds*float64(time.Second)), testCeiling))
	defer cancel()

	at := s.now()
	var pf discovery.Preflight
	if detect {
		pf, err = discovery.RunPreflight(ctx, opts, cfg.Cluster, cfg.Analysis, at)
	} else {
		pf.Client, err = prom.Connect(ctx, opts)
		if err == nil {
			pf.Scope, err = discovery.ResolveScope(ctx, pf.Client, cfg.Cluster, at)
		}
	}
	if err != nil {
		return resultOf(ctx, err)
	}
	found, err := discovery.ListNamespaces(ctx, pf.Client, pf.Scope, at, 0, "")
	if err != nil {
		return resultOf(ctx, err)
	}
	names := []string{}
	for _, c := range pf.Capabilities {
		if c.State != discovery.StateUnavailable {
			names = append(names, c.Name)
		}
	}
	return TestResult{OK: true, Code: CodeOK, Capabilities: names, Namespaces: found}, nil
}

// resultOf maps an error of the run to a failed result. An error Classify
// does not know, raised once ctx is done, is ENDPOINT_UNREACHABLE; any other
// is returned.
func resultOf(ctx context.Context, err error) (TestResult, error) {
	if code, ok := Classify(err); ok {
		return failure(code), nil
	}
	if ctx.Err() != nil {
		return failure(CodeUnreachable), nil
	}
	return TestResult{}, err
}

// NamespaceList is the answer of Namespaces: the namespaces a report of the
// profile can cover, and whether the profile limits them.
type NamespaceList struct {
	Code       string
	Namespaces []string
	Limited    bool
}

// Namespaces lists the namespaces a report of the current revision of an
// active profile can cover. A revision that names allowed namespaces answers
// them, sorted, and sends no request, as a collection of all namespaces runs
// them; one that names none connects, resolves the scope, and lists the
// namespaces with pods at the source. Concurrent calls for a revision share
// one run, and a good answer is reused for 30 seconds. It writes no audit
// row. A failure of the source is a code, not an error.
func (s *Service) Namespaces(ctx context.Context, actor auth.Principal, id string) (NamespaceList, error) {
	if actor.Role != auth.RoleOperator && actor.Role != auth.RoleAdministrator {
		return NamespaceList{}, ErrForbidden
	}
	rev, err := store.GetCurrentRevision(ctx, s.Pool, id)
	if err != nil {
		return NamespaceList{}, notFound(err)
	}
	if !rev.Active {
		return NamespaceList{}, ErrInactive
	}
	cur, err := currentOf(rev)
	if err != nil {
		return NamespaceList{}, err
	}
	if allowed := cur.Document.AllowedNamespaces; len(allowed) > 0 {
		return NamespaceList{Code: CodeOK, Namespaces: slices.Sorted(slices.Values(allowed)), Limited: true}, nil
	}
	// The run is shared by every caller waiting on it, so one caller that
	// leaves does not cancel it; runTest bounds it on its own.
	return s.namespaces.load(cur.RevisionID, s.now, func() (NamespaceList, error) {
		result, err := s.runTest(context.WithoutCancel(ctx), cur.Document, false)
		if err != nil {
			return NamespaceList{}, err
		}
		return NamespaceList{Code: result.Code, Namespaces: append([]string{}, result.Namespaces...)}, nil
	})
}
