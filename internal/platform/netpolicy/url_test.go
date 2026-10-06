package netpolicy

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
)

func codeOf(t *testing.T, err error) Code {
	t.Helper()
	var target *Error
	if !errors.As(err, &target) {
		t.Fatalf("error %v is not an *Error", err)
	}
	return target.Code
}

func prefixes(t *testing.T, texts ...string) []netip.Prefix {
	t.Helper()
	var out []netip.Prefix
	for _, text := range texts {
		out = append(out, netip.MustParsePrefix(text))
	}
	return out
}

func TestValidateURLRefusesUnsafe(t *testing.T) {
	cases := []struct {
		raw  string
		want Code
	}{
		{"ftp://metrics.internal/api/v1", "ENDPOINT_URL_REJECTED"},
		{"https://u:p@metrics.internal/api/v1", "ENDPOINT_URL_REJECTED"},
		{"https://metrics.internal/api/v1?q=x", "ENDPOINT_URL_REJECTED"},
		{"https://metrics.internal/api/v1#x", "ENDPOINT_URL_REJECTED"},
		{"//metrics.internal/api/v1", "ENDPOINT_URL_REJECTED"},
		{"https://[::1]/api/v1", "ENDPOINT_ADDRESS_REJECTED"},
	}
	for _, c := range cases {
		_, err := Policy{}.ValidateURL(c.raw)
		if err == nil {
			t.Fatalf("%q accepted", c.raw)
		}
		if got := codeOf(t, err); got != c.want {
			t.Errorf("%q: code %s, want %s", c.raw, got, c.want)
		}
	}
}

func TestValidateURLRefusesMalformed(t *testing.T) {
	inputs := []string{
		"https://h/?", "https://h/#", "https://@h/", "https://h:0/", "https://h:70000/",
		"https://h//x", "https://h/./x", "https://h/../x", "https://h/%2x",
		"https://h/a\tb", "https://h/a\\b",
		"https://h/" + strings.Repeat("a", 2049-len("https://h/")),
		"https://metrics_internal/", "",
	}
	for _, raw := range inputs {
		_, err := Policy{}.ValidateURL(raw)
		if err == nil {
			t.Fatalf("%q accepted", raw)
		}
		if got := codeOf(t, err); got != "ENDPOINT_URL_REJECTED" {
			t.Errorf("%q: code %s", raw, got)
		}
	}
}

func TestValidateURLRefusesEncodedDotsAndSlashes(t *testing.T) {
	for _, seg := range []string{"%2e%2e", "%2F", "%2E", "%5C"} {
		raw := "https://h/a/" + seg + "/b"
		_, err := Policy{}.ValidateURL(raw)
		if err == nil {
			t.Fatalf("%q accepted", raw)
		}
		if got := codeOf(t, err); got != "ENDPOINT_URL_REJECTED" {
			t.Errorf("%q: code %s", raw, got)
		}
	}
}

func TestValidateURLKeepsEncodedSegments(t *testing.T) {
	for raw, want := range map[string]string{"https://h/a;x=1": "/a;x=1", "https://h/a%20b": "/a%20b"} {
		o, err := Policy{}.ValidateURL(raw)
		if err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
		if o.Path != want {
			t.Errorf("%q: path %q, want %q", raw, o.Path, want)
		}
	}
}

func TestValidateURLCanonical(t *testing.T) {
	o, err := Policy{}.ValidateURL("HTTPS://Metrics.Internal./api/v1/")
	if err != nil {
		t.Fatal(err)
	}
	if o.URL() != "https://metrics.internal/api/v1" || o.Port != 443 || o.Host != "metrics.internal" ||
		o.Scheme != "https" || o.Path != "/api/v1" || o.Literal {
		t.Errorf("unexpected origin %+v url %s", o, o.URL())
	}
}

func TestValidateURLPorts(t *testing.T) {
	for raw, want := range map[string]string{
		"https://h:8443/x": "h:8443",
		"https://h:443/x":  "h",
		"http://h:80/x":    "h",
	} {
		o, err := Policy{}.ValidateURL(raw)
		if err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
		if o.Authority() != want {
			t.Errorf("%q: authority %q, want %q", raw, o.Authority(), want)
		}
	}
}

func TestValidateURLIPv6Literal(t *testing.T) {
	o, err := Policy{}.ValidateURL("https://[2001:db8::1]:8443/x")
	if err != nil {
		t.Fatal(err)
	}
	if o.Host != "2001:db8::1" || !o.Literal || o.Authority() != "[2001:db8::1]:8443" ||
		o.URL() != "https://[2001:db8::1]:8443/x" {
		t.Errorf("unexpected origin %+v", o)
	}
}

func TestValidateURLLiteralLiftedByThePolicy(t *testing.T) {
	lifted := Policy{Networks: prefixes(t, "127.0.0.1/32")}
	o, err := lifted.ValidateURL("https://127.0.0.1:9090/")
	if err != nil {
		t.Fatal(err)
	}
	if o.Host != "127.0.0.1" || o.Port != 9090 || !o.Literal {
		t.Errorf("unexpected origin %+v", o)
	}
	_, err = Policy{}.ValidateURL("https://127.0.0.1:9090/")
	if err == nil || codeOf(t, err) != "ENDPOINT_ADDRESS_REJECTED" {
		t.Errorf("empty policy: %v", err)
	}
}

func TestValidateURLIDNA(t *testing.T) {
	_, err := Policy{}.ValidateURL("https://mé.internal/")
	if err == nil || codeOf(t, err) != "ENDPOINT_URL_REJECTED" {
		t.Errorf("non-ASCII host: %v", err)
	}
	o, err := Policy{}.ValidateURL("https://xn--m-bga.internal/")
	if err != nil {
		t.Fatal(err)
	}
	if o.Host != "xn--m-bga.internal" {
		t.Errorf("host %q", o.Host)
	}
}

func TestValidateURLRootPath(t *testing.T) {
	for _, raw := range []string{"https://h", "https://h/"} {
		o, err := Policy{}.ValidateURL(raw)
		if err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
		if o.Path != "/" || o.URL() != "https://h" {
			t.Errorf("%q: path %q url %q", raw, o.Path, o.URL())
		}
	}
}

func TestValidHostname(t *testing.T) {
	label := func(n int) string { return strings.Repeat("a", n) }
	cases := []struct {
		host string
		want bool
	}{
		{strings.Join([]string{label(63), label(63), label(63), label(61)}, "."), true},
		{strings.Join([]string{label(63), label(63), label(63), label(62)}, "."), false},
		{label(64) + ".b", false},
		{label(63), true},
		{"a..b", false},
		{"A.b", false},
		{"metrics.internal", true},
		{"a-b.c1", true},
		{"-a.b", false},
		{"a_b", false},
		{"", false},
	}
	for _, c := range cases {
		if got := ValidHostname(c.host); got != c.want {
			t.Errorf("ValidHostname of %d characters = %v, want %v", len(c.host), got, c.want)
		}
	}
}

func TestValidateURLRefusesALongHostName(t *testing.T) {
	label := strings.Repeat("a", 63)
	host := strings.Join([]string{label, label, label, strings.Repeat("a", 62)}, ".")
	if _, err := (Policy{}).ValidateURL("https://" + host + "/"); err == nil || codeOf(t, err) != CodeURLRejected {
		t.Fatalf("got %v", err)
	}
}

func TestValidateURLDropsTheZone(t *testing.T) {
	o, err := Policy{}.ValidateURL("https://[2001:db8::1%25eth0]/")
	if err != nil {
		t.Fatal(err)
	}
	if o.Host != "2001:db8::1" || !o.Literal {
		t.Errorf("unexpected origin %+v", o)
	}
	_, err = Policy{}.ValidateURL("https://[fe80::1%25eth0]/")
	if err == nil || codeOf(t, err) != "ENDPOINT_ADDRESS_REJECTED" {
		t.Errorf("link-local with zone: %v", err)
	}
}
