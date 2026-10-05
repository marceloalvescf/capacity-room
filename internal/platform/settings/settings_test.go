package settings

import (
	"bytes"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const goodDSN = "postgres://u:p@h/db"

func env(dsn string) func(string) string {
	return func(name string) string {
		if name == "KCR_DATABASE_DSN" {
			return dsn
		}
		return ""
	}
}

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "platform.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func loadText(t *testing.T, content, dsn string) (Settings, error) {
	t.Helper()
	return Load(write(t, content), env(dsn))
}

func wantError(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("got no error, want %q", want)
	}
	if err.Error() != want {
		t.Fatalf("got %q, want %q", err.Error(), want)
	}
}

const secretLine = "server_secret_path = \"/run/secret\"\n"

func TestLoadDefaults(t *testing.T) {
	s, err := loadText(t, secretLine, goodDSN)
	if err != nil {
		t.Fatal(err)
	}
	if s.SessionTTL != 12*time.Hour || s.SessionIdle != time.Hour {
		t.Fatalf("got ttl %v idle %v", s.SessionTTL, s.SessionIdle)
	}
	if len(s.TrustedProxies) != 0 {
		t.Fatalf("got proxies %v", s.TrustedProxies)
	}
	if s.ServerSecretPath != "/run/secret" || s.DatabaseDSN != goodDSN {
		t.Fatalf("got %+v", s)
	}
}

func TestLoadEveryKey(t *testing.T) {
	text := "session_ttl_seconds = 7200\nsession_idle_seconds = 600\n" + secretLine +
		"trusted_proxies = [\"10.0.0.0/8\", \"::1/128\"]\n"
	s, err := loadText(t, text, "postgresql://x/db")
	if err != nil {
		t.Fatal(err)
	}
	if s.SessionTTL != 2*time.Hour || s.SessionIdle != 10*time.Minute {
		t.Fatalf("got ttl %v idle %v", s.SessionTTL, s.SessionIdle)
	}
	want := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("::1/128")}
	if len(s.TrustedProxies) != 2 || s.TrustedProxies[0] != want[0] || s.TrustedProxies[1] != want[1] {
		t.Fatalf("got proxies %v", s.TrustedProxies)
	}
	if s.DatabaseDSN != "postgresql://x/db" {
		t.Fatalf("got dsn %q", s.DatabaseDSN)
	}
}

func TestLoadRefusesUnknownKey(t *testing.T) {
	path := write(t, secretLine+"worker_slots = 2\n")
	_, err := Load(path, env(goodDSN))
	wantError(t, err, path+": unknown key worker_slots")
}

func TestLoadRefusesUnknownSection(t *testing.T) {
	path := write(t, secretLine+"[overrides]\nx = 1\n")
	_, err := Load(path, env(goodDSN))
	wantError(t, err, path+": unknown key overrides")
}

func TestLoadRefusesInvalidTOML(t *testing.T) {
	_, err := loadText(t, "session_ttl_seconds = \n", goodDSN)
	if err == nil || !strings.Contains(err.Error(), "is not valid TOML: 1:") {
		t.Fatalf("got %v", err)
	}
}

func TestLoadRangeOfTTL(t *testing.T) {
	for _, v := range []string{"59", "2592001"} {
		_, err := loadText(t, secretLine+"session_ttl_seconds = "+v+"\n", goodDSN)
		wantError(t, err, "session_ttl_seconds must be between 60 and 2592000")
	}
}

func TestLoadRangeOfIdle(t *testing.T) {
	for _, v := range []string{"59", "2592001"} {
		_, err := loadText(t, secretLine+"session_idle_seconds = "+v+"\n", goodDSN)
		wantError(t, err, "session_idle_seconds must be between 60 and 2592000")
	}
}

func TestLoadIdleOverTTL(t *testing.T) {
	_, err := loadText(t, secretLine+"session_ttl_seconds = 3600\nsession_idle_seconds = 7200\n", goodDSN)
	wantError(t, err, "session_idle_seconds must not exceed session_ttl_seconds")
}

func TestLoadRequiresSecretPath(t *testing.T) {
	_, err := loadText(t, "session_ttl_seconds = 3600\n", goodDSN)
	wantError(t, err, "server_secret_path is required")
}

func TestLoadRequiresAbsoluteSecretPath(t *testing.T) {
	_, err := loadText(t, "server_secret_path = \"secret.bin\"\n", goodDSN)
	wantError(t, err, "server_secret_path must be absolute")
}

func TestLoadRefusesBadProxy(t *testing.T) {
	_, err := loadText(t, secretLine+"trusted_proxies = [\"10.0.0.1\"]\n", goodDSN)
	wantError(t, err, "trusted_proxies must hold CIDR ranges")
	if strings.Contains(err.Error(), "10.0.0.1") {
		t.Fatalf("error repeats the proxy: %q", err)
	}
}

func TestLoadRequiresDSN(t *testing.T) {
	for _, dsn := range []string{"", ""} {
		_, err := loadText(t, secretLine, dsn)
		wantError(t, err, "KCR_DATABASE_DSN is not set")
	}
	_, err := Load(write(t, secretLine), func(string) string { return "" })
	wantError(t, err, "KCR_DATABASE_DSN is not set")
}

func TestLoadRefusesDSNScheme(t *testing.T) {
	_, err := loadText(t, secretLine, "mysql://x")
	wantError(t, err, "KCR_DATABASE_DSN must be a postgres:// or postgresql:// URL")
	if strings.Contains(err.Error(), "mysql") {
		t.Fatalf("error repeats the DSN: %q", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.toml")
	_, err := Load(path, env(goodDSN))
	if err == nil || !strings.HasPrefix(err.Error(), "cannot read ") || !strings.Contains(err.Error(), path) {
		t.Fatalf("got %v", err)
	}
}

func TestReadServerSecret(t *testing.T) {
	dir := t.TempDir()
	good := bytes.Repeat([]byte{0xAB}, 32)
	short := bytes.Repeat([]byte{0xCD}, 31)
	goodPath := filepath.Join(dir, "good")
	shortPath := filepath.Join(dir, "short")
	if err := os.WriteFile(goodPath, good, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shortPath, short, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Settings{ServerSecretPath: goodPath}.ReadServerSecret()
	if err != nil || !bytes.Equal(got, good) {
		t.Fatalf("got %x, %v", got, err)
	}

	_, err = Settings{ServerSecretPath: shortPath}.ReadServerSecret()
	wantError(t, err, "the server secret must hold at least 32 bytes")

	missing := filepath.Join(dir, "absent")
	_, err = Settings{ServerSecretPath: missing}.ReadServerSecret()
	if err == nil || !strings.HasPrefix(err.Error(), "cannot read the server secret at "+missing+": ") {
		t.Fatalf("got %v", err)
	}
}

func TestLoadFolderDefaults(t *testing.T) {
	s, err := loadText(t, secretLine, goodDSN)
	if err != nil {
		t.Fatal(err)
	}
	if s.CredentialDir != "/run/secrets/metrics" || s.CABundleDir != "/run/config/ca" {
		t.Fatalf("got %q and %q", s.CredentialDir, s.CABundleDir)
	}
}

func TestLoadFolderRules(t *testing.T) {
	cases := []struct{ line, want string }{
		{"credential_dir = \"secrets\"\n", "credential_dir must be absolute"},
		{"credential_dir = \"/run/../x\"\n", "credential_dir must not hold a .. segment"},
		{"ca_bundle_dir = \"ca\"\n", "ca_bundle_dir must be absolute"},
		{"ca_bundle_dir = \"/run/../x\"\n", "ca_bundle_dir must not hold a .. segment"},
	}
	for _, c := range cases {
		_, err := loadText(t, secretLine+c.line, goodDSN)
		wantError(t, err, c.want)
	}
	if _, err := loadText(t, secretLine+"credential_dir = \"/run/..x/y\"\n", goodDSN); err != nil {
		t.Fatal(err)
	}
}

func TestLoadHostnamesNormalised(t *testing.T) {
	s, err := loadText(t, secretLine+"allowed_endpoint_hostnames = [\"Prometheus.Example.Invalid.\", \" a.b \"]\n", goodDSN)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.AllowedEndpointHostnames) != 2 ||
		s.AllowedEndpointHostnames[0] != "prometheus.example.invalid" ||
		s.AllowedEndpointHostnames[1] != "a.b" {
		t.Fatalf("got %q", s.AllowedEndpointHostnames)
	}
}

func TestLoadHostnamesRefused(t *testing.T) {
	cases := []struct{ list, want string }{
		{`["*.example"]`, "allowed_endpoint_hostnames entry 1 is not a host name"},
		{`["metrics_internal"]`, "allowed_endpoint_hostnames entry 1 is not a host name"},
		{`["a", "A"]`, "allowed_endpoint_hostnames repeats entry 2"},
	}
	for _, c := range cases {
		_, err := loadText(t, secretLine+"allowed_endpoint_hostnames = "+c.list+"\n", goodDSN)
		wantError(t, err, c.want)
	}
}

func TestLoadCIDRsRefused(t *testing.T) {
	cases := []struct{ list, want string }{
		{`["10.0.0.1"]`, "allowed_endpoint_cidrs entry 1 is not a CIDR block"},
		{`["10.0.0.1/8"]`, "allowed_endpoint_cidrs entry 1 is not a CIDR block"},
		{`["169.254.0.0/16"]`, "allowed_endpoint_cidrs entry 1 covers a metadata address; list that single address instead"},
		{`["fd00:ec2::/64"]`, "allowed_endpoint_cidrs entry 1 covers a metadata address; list that single address instead"},
		{`["10.0.0.0/8", "10.0.0.0/8"]`, "allowed_endpoint_cidrs repeats entry 2"},
	}
	for _, c := range cases {
		_, err := loadText(t, secretLine+"allowed_endpoint_cidrs = "+c.list+"\n", goodDSN)
		wantError(t, err, c.want)
	}
}

func TestLoadCIDRsAccepted(t *testing.T) {
	s, err := loadText(t, secretLine+"allowed_endpoint_cidrs = [\"169.254.169.254/32\", \"10.0.0.0/8\", \"fd00:ec2::254/128\"]\n", goodDSN)
	if err != nil {
		t.Fatal(err)
	}
	want := []netip.Prefix{
		netip.MustParsePrefix("169.254.169.254/32"),
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("fd00:ec2::254/128"),
	}
	if len(s.AllowedEndpointCIDRs) != 3 {
		t.Fatalf("got %v", s.AllowedEndpointCIDRs)
	}
	for i := range want {
		if s.AllowedEndpointCIDRs[i] != want[i] {
			t.Fatalf("got %v", s.AllowedEndpointCIDRs)
		}
	}
}
