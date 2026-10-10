package settings

import (
	"bytes"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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
	path := write(t, secretLine+"worker_slot = 2\n")
	_, err := Load(path, env(goodDSN))
	wantError(t, err, path+": unknown key worker_slot")
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
		{`["` + strings.Join([]string{strings.Repeat("a", 63), strings.Repeat("a", 63), strings.Repeat("a", 63), strings.Repeat("a", 62)}, ".") + `"]`,
			"allowed_endpoint_hostnames entry 1 is not a host name"},
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
		{`["::ffff:10.0.0.0/104"]`, "allowed_endpoint_cidrs entry 1 is an IPv4-mapped prefix; write the IPv4 form"},
		{`["10.0.0.0/8", "::ffff:0.0.0.0/96"]`, "allowed_endpoint_cidrs entry 2 is an IPv4-mapped prefix; write the IPv4 form"},
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

var workerKeys = []struct {
	key      string
	min, max int
}{
	{"worker_slots", 1, 16},
	{"worker_drain_seconds", 1, 3600},
	{"collect_timeout_seconds", 60, 3600},
	{"render_timeout_seconds", 10, 3600},
	{"job_max_attempts", 1, 10},
	{"report_retention_days", 1, 365},
	{"expired_report_days", 1, 365},
	{"admission_limit", 1, 1000},
	{"admission_profile_limit", 1, 1000},
	{"admission_retry_seconds", 1, 300},
}

func TestLoadWorkerDefaults(t *testing.T) {
	s, err := loadText(t, secretLine, goodDSN)
	if err != nil {
		t.Fatal(err)
	}
	day := 24 * time.Hour
	if s.WorkerSlots != 2 || s.WorkerDrain != 30*time.Second || s.CollectTimeout != 900*time.Second ||
		s.RenderTimeout != 300*time.Second || s.JobMaxAttempts != 3 || s.ReportRetention != 7*day ||
		s.ExpiredReportKept != 30*day || s.AdmissionLimit != 20 || s.AdmissionProfileLimit != 5 ||
		s.AdmissionRetry != 5*time.Second {
		t.Fatalf("got %+v", s)
	}
}

func TestLoadWorkerRanges(t *testing.T) {
	for _, k := range workerKeys {
		want := fmt.Sprintf("%s must be between %d and %d", k.key, k.min, k.max)
		for _, v := range []int{k.min - 1, k.max + 1} {
			_, err := loadText(t, secretLine+fmt.Sprintf("%s = %d\n", k.key, v), goodDSN)
			wantError(t, err, want)
		}
	}
}

func TestLoadWorkerBounds(t *testing.T) {
	for _, k := range workerKeys {
		for _, v := range []int{k.min, k.max} {
			text := secretLine + fmt.Sprintf("%s = %d\n", k.key, v)
			if k.key == "admission_profile_limit" && v > 20 {
				text += "admission_limit = 1000\n"
			}
			if _, err := loadText(t, text, goodDSN); err != nil {
				t.Fatalf("%s = %d: %v", k.key, v, err)
			}
		}
	}
}

func TestLoadWorkerValues(t *testing.T) {
	text := secretLine + "worker_slots = 4\nworker_drain_seconds = 10\ncollect_timeout_seconds = 120\n" +
		"render_timeout_seconds = 20\njob_max_attempts = 5\nreport_retention_days = 2\n" +
		"expired_report_days = 3\nadmission_limit = 9\nadmission_profile_limit = 9\nadmission_retry_seconds = 7\n"
	s, err := loadText(t, text, goodDSN)
	if err != nil {
		t.Fatal(err)
	}
	day := 24 * time.Hour
	if s.WorkerSlots != 4 || s.WorkerDrain != 10*time.Second || s.CollectTimeout != 120*time.Second ||
		s.RenderTimeout != 20*time.Second || s.JobMaxAttempts != 5 || s.ReportRetention != 2*day ||
		s.ExpiredReportKept != 3*day || s.AdmissionLimit != 9 || s.AdmissionProfileLimit != 9 ||
		s.AdmissionRetry != 7*time.Second {
		t.Fatalf("got %+v", s)
	}
}

// An omitted admission_profile_limit is 5, or admission_limit when that is
// smaller, so a file that lowers only the global limit loads.
func TestLoadAdmissionProfileDefaultFitsTheLimit(t *testing.T) {
	for _, c := range []struct {
		text string
		want int
	}{
		{"admission_limit = 3\n", 3},
		{"admission_limit = 1\n", 1},
		{"admission_limit = 5\n", 5},
		{"admission_limit = 6\n", 5},
		{"admission_limit = 1000\n", 5},
	} {
		s, err := loadText(t, secretLine+c.text, goodDSN)
		if err != nil {
			t.Fatalf("%q: %v", c.text, err)
		}
		if s.AdmissionProfileLimit != c.want {
			t.Fatalf("%q: admission_profile_limit %d, want %d", c.text, s.AdmissionProfileLimit, c.want)
		}
	}
}

func TestLoadAdmissionProfileAboveGlobal(t *testing.T) {
	_, err := loadText(t, secretLine+"admission_limit = 3\nadmission_profile_limit = 4\n", goodDSN)
	wantError(t, err, "admission_profile_limit must not exceed admission_limit")
}

func TestLoadWorkerWrongType(t *testing.T) {
	path := write(t, secretLine+"worker_slots = \"two\"\n")
	_, err := Load(path, env(goodDSN))
	wantError(t, err, path+" has a value of the wrong type")
}

func TestLoadRefusesFoldersThatHoldTheFileOrTheSecret(t *testing.T) {
	root := t.TempDir()
	etc := filepath.Join(root, "kcr")
	if err := os.MkdirAll(etc, 0o700); err != nil {
		t.Fatal(err)
	}
	load := func(extra string) error {
		path := filepath.Join(etc, "platform.toml")
		text := "server_secret_path = \"" + filepath.Join(etc, "server-secret") + "\"\n" + extra
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := Load(path, env(goodDSN))
		return err
	}
	const credentialMessage = "credential_dir must not hold the deployment file or the server secret"
	const caMessage = "ca_bundle_dir must not hold the deployment file or the server secret"

	wantError(t, load("credential_dir = \""+etc+"\"\n"), credentialMessage)
	wantError(t, load("ca_bundle_dir = \""+root+"\"\n"), caMessage)
	wantError(t, load("credential_dir = \""+etc+"/\"\n"), credentialMessage)
	for _, extra := range []string{
		"credential_dir = \"" + filepath.Join(etc, "credentials") + "\"\n",
		"ca_bundle_dir = \"" + filepath.Join(root, "kcr-ca") + "\"\n",
		"credential_dir = \"" + filepath.Join(root, "kcr-credentials") + "\"\n",
	} {
		if err := load(extra); err != nil {
			t.Fatalf("%s: %v", extra, err)
		}
	}
}

func TestLoadRefusesFoldersThatHoldARelativeDeploymentFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	text := "server_secret_path = \"/run/secret\"\ncredential_dir = \"" + dir + "\"\n"
	if err := os.WriteFile("platform.toml", []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load("platform.toml", env(goodDSN))
	wantError(t, err, "credential_dir must not hold the deployment file or the server secret")
}

func TestReadServerSecretRefusesWhatIsNotARegularFile(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{dir, fifo} {
		_, err := Settings{ServerSecretPath: path}.ReadServerSecret()
		wantError(t, err, "the server secret at "+path+" is not a regular file")
	}
}

func TestReadServerSecretCapsTheSize(t *testing.T) {
	dir := t.TempDir()
	exact := filepath.Join(dir, "exact")
	over := filepath.Join(dir, "over")
	if err := os.WriteFile(exact, bytes.Repeat([]byte{1}, 64<<10), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(over, bytes.Repeat([]byte{1}, 64<<10+1), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Settings{ServerSecretPath: exact}.ReadServerSecret()
	if err != nil || len(got) != 64<<10 {
		t.Fatalf("got %d bytes, %v", len(got), err)
	}
	_, err = Settings{ServerSecretPath: over}.ReadServerSecret()
	wantError(t, err, "the server secret at "+over+" is larger than 64 KiB")
}
