package profiles

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/audit"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/netpolicy"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
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

// TestResult is the answer of a profile test.
type TestResult struct {
	OK           bool
	Code         string
	Capabilities []string // the names whose state is not unavailable, in their order
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

	result, err := s.runTest(ctx, d)
	if err != nil {
		return TestResult{}, err
	}
	outcome := audit.OutcomeSucceeded
	if !result.OK {
		outcome = audit.OutcomeFailed
	}
	err = audit.Append(ctx, s.Pool, s.now(), audit.Event{
		ActorID: &actor.ID, Action: ActionProfileTest, TargetType: targetProfile, TargetID: id, Outcome: outcome,
		Metadata: audit.Metadata{"code": result.Code, "revision_id": rev.ID.String()},
	})
	if err != nil {
		return TestResult{}, err
	}
	return result, nil
}

func failure(code string) TestResult { return TestResult{Code: code} }

// runTest holds no database connection: it reads files and sends requests.
func (s *Service) runTest(ctx context.Context, d Document) (TestResult, error) {
	token := ""
	if d.CredentialFile != "" {
		var err error
		if token, err = ResolveCredential(s.Settings.CredentialDir, d.CredentialFile); err != nil {
			return failure(CodeCredentialUnavailable), nil
		}
	}
	tlsConfig := &tls.Config{InsecureSkipVerify: !d.TLSVerify} //nolint:gosec // the administrator turned verification off for this profile
	if d.CABundleFile != "" && d.TLSVerify {
		bundle, err := ResolveCABundle(s.Settings.CABundleDir, d.CABundleFile)
		if err != nil {
			return failure(CodeCABundleUnavailable), nil
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(bundle) {
			return failure(CodeCABundleUnavailable), nil
		}
		tlsConfig.RootCAs = pool
	}

	period, zone, profile, severity := d.ResolvedDefaults()
	cfg := d.RunConfig(period, zone, profile, severity, d.AllowedNamespaces)
	ctx, cancel := context.WithTimeout(ctx, time.Duration(3*d.TimeoutSeconds*float64(time.Second)))
	defer cancel()

	pf, err := discovery.RunPreflight(ctx, prom.Options{
		Metrics: cfg.Metrics,
		Token:   token,
		Transport: &netpolicy.Transport{
			Policy: s.Policy, Resolve: s.Resolve, TLS: tlsConfig,
		},
	}, cfg.Cluster, cfg.Analysis, s.now())
	if err != nil {
		return classify(ctx, err)
	}
	names := []string{}
	for _, c := range pf.Capabilities {
		if c.State != discovery.StateUnavailable {
			names = append(names, c.Name)
		}
	}
	return TestResult{OK: true, Code: CodeOK, Capabilities: names}, nil
}

// classify maps an error of the preflight to a code.
func classify(ctx context.Context, err error) (TestResult, error) {
	var policyErr *netpolicy.Error
	if errors.As(err, &policyErr) {
		return failure(string(policyErr.Code)), nil
	}
	var promErr *prom.Error
	if errors.As(err, &promErr) {
		switch promErr.Kind {
		case prom.KindTLS:
			return failure(CodeTLSFailed), nil
		case prom.KindAuthentication, prom.KindAuthorization:
			return failure(CodeAuthRejected), nil
		case prom.KindEndpointNotFound:
			return failure(CodeAPINotFound), nil
		default:
			return failure(CodeUnreachable), nil
		}
	}
	var scopeErr *discovery.ScopeError
	if errors.As(err, &scopeErr) {
		return failure(CodeScopeNotFound), nil
	}
	if errors.Is(err, discovery.ErrInsufficient) {
		return failure(CodeCapabilities), nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return failure(CodeUnreachable), nil
	}
	return TestResult{}, err
}
