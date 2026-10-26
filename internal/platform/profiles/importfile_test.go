package profiles

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/platform/netpolicy"
)

const importExample = `
[[profiles]]
id = "prod"
display_name = "Production"
endpoint_url = "https://metrics.test/api/v1"
cluster_name = "prod"
allowed_namespaces = ["payments", "checkout"]
active = true

[[profiles]]
id = "stage"
display_name = "Staging"
endpoint_url = "https://metrics.test/api/v1"
cluster_name = "stage"
allowed_namespaces = ["payments"]
timeout_seconds = 30
default_period = "7d"
permitted_periods = ["24h", "7d"]

[profiles.thresholds]
warning_percent = 60
`

func writeImport(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "profiles.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func staticPolicy() netpolicy.Policy {
	return netpolicy.Policy{Hostnames: []string{"metrics.test"}}
}

func TestReadImportFileAcceptsTheExample(t *testing.T) {
	entries, err := ReadImportFile(writeImport(t, importExample), staticPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].ID != "prod" || entries[1].ID != "stage" {
		t.Fatalf("entries = %+v", entries)
	}
	prod := entries[0].Document
	if !prod.Active || prod.DisplayName != "Production" || prod.EndpointURL != "https://metrics.test/api/v1" ||
		prod.TimeoutSeconds != 90 || !prod.TLSVerify || prod.ClusterLabel != "cluster" ||
		prod.FilterMode != "required" || prod.NodeAutoscaling != "auto" || prod.DefaultPeriod != nil ||
		!reflect.DeepEqual(prod.AllowedNamespaces, []string{"payments", "checkout"}) ||
		prod.Thresholds.WarningPercent != 70 || len(prod.PermittedPeriods) != 0 {
		t.Fatalf("prod = %+v", prod)
	}
	stage := entries[1].Document
	if stage.Active || stage.TimeoutSeconds != 30 ||
		stage.DefaultPeriod == nil || *stage.DefaultPeriod != "7d" ||
		stage.Thresholds.WarningPercent != 60 || stage.Thresholds.CriticalPercent != 85 {
		t.Fatalf("stage = %+v", stage)
	}
}

func TestReadImportFileRefusals(t *testing.T) {
	const head = "[[profiles]]\nid = \"a\"\ndisplay_name = \"A\"\nendpoint_url = \"https://metrics.test/api/v1\"\ncluster_name = \"a\"\nallowed_namespaces = [\"ns\"]\n"
	cases := []struct {
		name, content, want string
	}{
		{"unknown key", head + "colour = \"red\"\n", "unknown key profiles.colour"},
		{"endpoint_type", head + "endpoint_type = \"prometheus\"\n", "unknown key profiles.endpoint_type"},
		{"token", head + "token = \"x\"\n", "unknown key profiles.token"},
		{"token_env", head + "token_env = \"x\"\n", "unknown key profiles.token_env"},
		{"credential", head + "credential = \"x\"\n", "unknown key profiles.credential"},
		{"credential_value", head + "credential_value = \"x\"\n", "unknown key profiles.credential_value"},
		{"password", head + "password = \"x\"\n", "unknown key profiles.password"},
		{"api_key", head + "api_key = \"x\"\n", "unknown key profiles.api_key"},
		{"ca_bundle", head + "ca_bundle = \"/x\"\n", "unknown key profiles.ca_bundle"},
		{"ca_bundle_path", head + "ca_bundle_path = \"/x\"\n", "unknown key profiles.ca_bundle_path"},
		{"default_min_severity", head + "default_min_severity = \"all\"\n", "unknown key profiles.default_min_severity"},
		{"permitted_min_severities", head + "permitted_min_severities = [\"all\"]\n", "unknown key profiles.permitted_min_severities"},
		{"unknown threshold", head + "[profiles.thresholds]\nloud = 1\n", "unknown key profiles.thresholds.loud"},
		{"repeated id", head + head, "profile a is repeated"},
		{"missing id", strings.Replace(head, "id = \"a\"\n", "", 1), "profile 1: id invalid"},
		{"bad id", head + strings.Replace(head, "id = \"a\"", "id = \"A B\"", 1), "profile 2: id invalid"},
		{"empty cluster", strings.Replace(head, "cluster_name = \"a\"", "cluster_name = \"\"", 1), "profile a: cluster_name empty"},
		{"metadata endpoint", strings.Replace(head, "https://metrics.test/api/v1", "http://169.254.169.254/", 1), "profile a: endpoint_url invalid"},
		{"not toml", "[[profiles\nid = \"x\"", "is not valid TOML"},
		{"wrong type", "[[profiles]]\nid = 3\n", "has a value of the wrong type"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := writeImport(t, c.content)
			_, err := ReadImportFile(path, netpolicy.Policy{})
			if err == nil {
				t.Fatal("accepted")
			}
			msg := err.Error()
			if !strings.HasPrefix(msg, path+": ") || !strings.Contains(msg, c.want) {
				t.Fatalf("message %q, want prefix %q and %q", msg, path+": ", c.want)
			}
			for _, banned := range []string{"\"x\"", "/x", "=x", "= x"} {
				if strings.Contains(msg, banned) {
					t.Fatalf("message %q holds %q", msg, banned)
				}
			}
			if strings.Contains(msg, "169.254") || strings.Contains(msg, "metrics.test") || strings.Contains(msg, "\n") {
				t.Fatalf("message %q leaks or spans lines", msg)
			}
		})
	}
}

func TestReadImportFileMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.toml")
	_, err := ReadImportFile(path, staticPolicy())
	if err == nil || !strings.HasPrefix(err.Error(), path+": cannot be read") {
		t.Fatalf("err = %v", err)
	}
}
