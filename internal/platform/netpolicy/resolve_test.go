package netpolicy

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
)

func answers(texts ...string) Resolver {
	return func(context.Context, string) ([]netip.Addr, error) {
		var out []netip.Addr
		for _, text := range texts {
			out = append(out, netip.MustParseAddr(text))
		}
		return out, nil
	}
}

func TestResolveLiteralNeverCallsResolver(t *testing.T) {
	o := Origin{Scheme: "https", Host: "10.1.1.1", Port: 443, Path: "/", Literal: true}
	p := Policy{Networks: prefixes(t, "10.0.0.0/8")}
	got, err := p.Resolve(context.Background(), o, func(context.Context, string) ([]netip.Addr, error) {
		t.Fatal("the resolver was called")
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != netip.MustParseAddr("10.1.1.1") {
		t.Errorf("got %v", got)
	}
}

func TestResolveMixedAnswersFailClosed(t *testing.T) {
	p := Policy{Networks: prefixes(t, "10.0.0.0/8")}
	o := Origin{Scheme: "https", Host: "svc.internal", Port: 443, Path: "/"}
	for _, order := range [][]string{{"10.1.1.1", "127.0.0.1"}, {"127.0.0.1", "10.1.1.1"}} {
		_, err := p.Resolve(context.Background(), o, answers(order...))
		if err == nil || codeOf(t, err) != "ENDPOINT_ADDRESS_REJECTED" {
			t.Errorf("%v: %v", order, err)
		}
	}
}

func TestResolveDedupes(t *testing.T) {
	p := Policy{Networks: prefixes(t, "10.0.0.0/8")}
	o := Origin{Scheme: "https", Host: "svc.internal", Port: 443, Path: "/"}
	got, err := p.Resolve(context.Background(), o, answers("10.1.1.1", "10.1.1.1", "::ffff:10.1.1.2"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != netip.MustParseAddr("10.1.1.1") || got[1] != netip.MustParseAddr("10.1.1.2") {
		t.Errorf("got %v", got)
	}
}

func TestResolveEmptyAnswers(t *testing.T) {
	p := Policy{Hostnames: []string{"svc.internal"}}
	o := Origin{Scheme: "https", Host: "svc.internal", Port: 443, Path: "/"}
	failing := func(context.Context, string) ([]netip.Addr, error) {
		return nil, errors.New("lookup secret-resolver-text failed")
	}
	for _, r := range []Resolver{answers(), failing} {
		_, err := p.Resolve(context.Background(), o, r)
		if err == nil || codeOf(t, err) != "ENDPOINT_RESOLUTION_FAILED" {
			t.Fatalf("got %v", err)
		}
		if strings.Contains(err.Error(), "svc.internal") || strings.Contains(err.Error(), "secret-resolver-text") {
			t.Errorf("error text leaks: %q", err.Error())
		}
	}
}

func TestResolveHostnameEntryAdmitsAnyClassSafeAnswer(t *testing.T) {
	p := Policy{Hostnames: []string{"h"}}
	o := Origin{Scheme: "https", Host: "h", Port: 443, Path: "/"}
	got, err := p.Resolve(context.Background(), o, answers("203.0.113.5"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != netip.MustParseAddr("203.0.113.5") {
		t.Errorf("got %v", got)
	}
}

func TestErrorCarriesOnlyTheCode(t *testing.T) {
	var errs []error
	for _, raw := range []string{"ftp://secret.example/x", "https://127.0.0.1/", "https://secret.example/%2e%2e"} {
		_, err := Policy{}.ValidateURL(raw)
		errs = append(errs, err)
	}
	p := Policy{}
	o := Origin{Scheme: "https", Host: "secret.example", Port: 443, Path: "/"}
	_, err := p.Resolve(context.Background(), o, answers())
	errs = append(errs, err)
	_, err = p.Resolve(context.Background(), o, answers("10.1.1.1"))
	errs = append(errs, err)
	for _, err := range errs {
		var target *Error
		if !errors.As(err, &target) {
			t.Fatalf("%v is not an *Error", err)
		}
		if err.Error() != string(target.Code) {
			t.Errorf("text %q, code %q", err.Error(), target.Code)
		}
		if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "127") {
			t.Errorf("text leaks: %q", err.Error())
		}
	}
}
