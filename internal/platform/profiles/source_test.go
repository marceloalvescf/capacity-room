//go:build postgres

package profiles

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/netpolicy"
)

func TestSourceReadsTheReferences(t *testing.T) {
	f := newFixture(t)
	writeFile(t, f.credDir, "token", secretToken+"\n")
	bundle := certPEM(t)
	write(t, f.caDir, "ca.pem", bundle)
	var revealed []string
	f.svc.Reveal = func(secret string) { revealed = append(revealed, secret) }

	d := baseDocument()
	d.CredentialFile = "token"
	d.CABundleFile = "ca.pem"
	metrics := config.Metrics{URL: "https://metrics.test/api/v1", TimeoutSeconds: 7}
	opts, err := f.svc.Source(d, metrics)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Token != secretToken || opts.Metrics != metrics {
		t.Fatalf("options = %+v", opts)
	}
	if len(revealed) != 1 || revealed[0] != secretToken {
		t.Fatalf("revealed %q", revealed)
	}
	transport, ok := opts.Transport.(*netpolicy.Transport)
	if !ok {
		t.Fatalf("transport is %T", opts.Transport)
	}
	block, _ := pem.Decode(bundle)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	want := x509.NewCertPool()
	want.AddCert(cert)
	if transport.TLS.RootCAs == nil || !transport.TLS.RootCAs.Equal(want) || transport.TLS.InsecureSkipVerify {
		t.Fatalf("TLS = %+v", transport.TLS)
	}
	if !reflect.DeepEqual(transport.Policy, f.svc.Policy) {
		t.Fatalf("policy = %+v", transport.Policy)
	}
}

func TestSourceRefusesAMissingCredential(t *testing.T) {
	f := newFixture(t)
	called := false
	f.svc.Reveal = func(string) { called = true }
	d := baseDocument()
	d.CredentialFile = "absent"
	_, err := f.svc.Source(d, config.Metrics{})
	var se *SourceError
	if !errors.As(err, &se) || se.Code != "PROFILE_CREDENTIAL_UNAVAILABLE" {
		t.Fatalf("err = %v", err)
	}
	if called {
		t.Fatal("Reveal was called")
	}
}

func TestSourceRefusesAMissingCABundle(t *testing.T) {
	f := newFixture(t)
	d := baseDocument()
	d.CABundleFile = "absent.pem"
	_, err := f.svc.Source(d, config.Metrics{})
	var se *SourceError
	if !errors.As(err, &se) || se.Code != "PROFILE_CA_BUNDLE_UNAVAILABLE" {
		t.Fatalf("err = %v", err)
	}
}

func TestRevisionReadsAnInactiveOne(t *testing.T) {
	f := newFixture(t)
	first := f.create(t, "prod", baseDocument())
	second, err := f.svc.Revise(context.Background(), f.root, "prod", Change{
		BaseRevisionID: first.RevisionID,
		Fields:         map[string]json.RawMessage{"active": json.RawMessage("false")},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.Revision(context.Background(), first.RevisionID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != first.RevisionID || got.ProfileID != "prod" || got.Number != 1 || !got.Active || !got.Document.Active {
		t.Fatalf("first = %+v", got)
	}
	got, err = f.svc.Revision(context.Background(), second.RevisionID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Number != 2 || got.Active || got.Document.Active {
		t.Fatalf("second = %+v", got)
	}
	if _, err := f.svc.Revision(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("absent: %v", err)
	}
}

func TestTestCeiling(t *testing.T) {
	f := newFixture(t)
	d := baseDocument()
	d.TimeoutSeconds = 600
	f.create(t, "prod", d)
	var deadline time.Time
	var seen bool
	f.svc.Resolve = func(ctx context.Context, _ string) ([]netip.Addr, error) {
		deadline, seen = ctx.Deadline()
		return nil, errors.New("resolver stops here")
	}
	before := time.Now()
	got, err := f.svc.Test(context.Background(), f.root, "prod")
	after := time.Now()
	if err != nil {
		t.Fatal(err)
	}
	if got.Code != "ENDPOINT_RESOLUTION_FAILED" {
		t.Fatalf("got %+v", got)
	}
	if !seen {
		t.Fatal("the resolver saw no deadline")
	}
	// The ceiling is written out so that a change of the constant is seen.
	// The deadline is set from the clock between the two readings, so it is
	// exactly the ceiling after one of them, and never a second off.
	if deadline.Before(before.Add(2*time.Minute)) || deadline.After(after.Add(2*time.Minute)) {
		t.Fatalf("the deadline is %s, want between %s and %s", deadline, before.Add(2*time.Minute), after.Add(2*time.Minute))
	}
}
