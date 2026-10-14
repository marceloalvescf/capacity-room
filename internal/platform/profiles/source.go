package profiles

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"

	"github.com/google/uuid"

	"github.com/marceloalvescf/capacity-room/internal/config"
	"github.com/marceloalvescf/capacity-room/internal/discovery"
	"github.com/marceloalvescf/capacity-room/internal/platform/netpolicy"
	"github.com/marceloalvescf/capacity-room/internal/platform/store"
	"github.com/marceloalvescf/capacity-room/internal/prom"
)

// Revision is one revision as a run reads it.
type Revision struct {
	ID        uuid.UUID
	ProfileID string
	Number    int
	Active    bool
	Document  Document
}

// Revision reads the revision with id, whatever its profile's pointer. It
// returns ErrNotFound when there is none.
func (s *Service) Revision(ctx context.Context, id uuid.UUID) (Revision, error) {
	row, err := store.GetProfileRevision(ctx, s.Pool, id)
	if err != nil {
		return Revision{}, notFound(err)
	}
	cur, err := currentOf(row)
	if err != nil {
		return Revision{}, err
	}
	return Revision{ID: row.ID, ProfileID: row.ProfileID, Number: row.Revision, Active: row.Active, Document: cur.Document}, nil
}

// SourceError is a reference that could not be read; Code is
// CodeCredentialUnavailable or CodeCABundleUnavailable.
type SourceError struct{ Code string }

// Error returns the code as text.
func (e *SourceError) Error() string { return e.Code }

// Source reads the credential and the CA bundle of d and returns the options
// of the Prometheus client through the policy transport, with the metrics of
// d. Reveal is called with the credential before the options are returned.
func (s *Service) Source(d Document, metrics config.Metrics) (prom.Options, error) {
	token := ""
	if d.CredentialFile != "" {
		var err error
		if token, err = ResolveCredential(s.Settings.CredentialDir, d.CredentialFile); err != nil {
			return prom.Options{}, &SourceError{Code: CodeCredentialUnavailable}
		}
		if s.Reveal != nil {
			s.Reveal(token)
		}
	}
	tlsConfig := &tls.Config{InsecureSkipVerify: !d.TLSVerify} //nolint:gosec // the administrator turned verification off for this profile
	if d.CABundleFile != "" && d.TLSVerify {
		bundle, err := ResolveCABundle(s.Settings.CABundleDir, d.CABundleFile)
		if err != nil {
			return prom.Options{}, &SourceError{Code: CodeCABundleUnavailable}
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(bundle) {
			return prom.Options{}, &SourceError{Code: CodeCABundleUnavailable}
		}
		tlsConfig.RootCAs = pool
	}
	return prom.Options{
		Metrics:   metrics,
		Token:     token,
		Transport: &netpolicy.Transport{Policy: s.Policy, Resolve: s.Resolve, TLS: tlsConfig},
	}, nil
}

// Classify maps an error of a run to a code of the profile test and reports
// whether it knew the error. A cancelled or expired context is
// ENDPOINT_UNREACHABLE.
func Classify(err error) (string, bool) {
	var policyErr *netpolicy.Error
	if errors.As(err, &policyErr) {
		return string(policyErr.Code), true
	}
	var sourceErr *SourceError
	if errors.As(err, &sourceErr) {
		return sourceErr.Code, true
	}
	var promErr *prom.Error
	if errors.As(err, &promErr) {
		switch promErr.Kind {
		case prom.KindTLS:
			return CodeTLSFailed, true
		case prom.KindAuthentication, prom.KindAuthorization:
			return CodeAuthRejected, true
		case prom.KindEndpointNotFound:
			return CodeAPINotFound, true
		default:
			return CodeUnreachable, true
		}
	}
	var scopeErr *discovery.ScopeError
	if errors.As(err, &scopeErr) {
		return CodeScopeNotFound, true
	}
	if errors.Is(err, discovery.ErrInsufficient) {
		return CodeCapabilities, true
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return CodeUnreachable, true
	}
	return "", false
}
