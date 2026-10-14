//go:build helm

package chart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/platform/netpolicy"
	"github.com/marceloalvescf/capacity-room/internal/platform/profiles"
)

func TestNoJobsByDefault(t *testing.T) {
	for _, o := range render(t, "capacity-room", required...) {
		if o.kind() == "Job" {
			t.Fatalf("a Job %s rendered with no admin and no profiles", o.name())
		}
	}
}

func TestAdminJob(t *testing.T) {
	objects := render(t, "capacity-room", append(required,
		"--set", "admin.username=admin", "--set", "admin.existingSecret=admin-secret")...)
	job := find(t, objects, "Job", "capacity-room-admin")
	annotations := job["metadata"].(map[string]any)["annotations"].(map[string]any)
	if annotations["helm.sh/hook"] != "post-install" ||
		annotations["helm.sh/hook-delete-policy"] != "before-hook-creation,hook-succeeded" {
		t.Fatalf("annotations %v", annotations)
	}
	c := job.podSpec()["containers"].([]any)[0].(map[string]any)
	want := []any{"admin", "bootstrap-admin", "--username", "admin", "--language", "en",
		"--password-file", "/opt/capacity-room/admin/password", "--config", "/opt/capacity-room/platform.toml"}
	if !equalLists(c["args"].([]any), want) {
		t.Fatalf("args %v", c["args"])
	}
	if !containsEnv(c["env"].([]any), "CAPACITYROOM_BOOTSTRAP_MODE", "1") {
		t.Fatalf("env %v", c["env"])
	}
	for name, spec := range pods(objects) {
		hasAdmin := false
		for _, v := range spec["volumes"].([]any) {
			if v.(map[string]any)["name"] == "admin" {
				hasAdmin = true
			}
		}
		if hasAdmin != (name == "Job capacity-room-admin") {
			t.Errorf("%s: admin volume %v", name, hasAdmin)
		}
		if name != "Job capacity-room-admin" {
			for _, e := range spec["containers"].([]any)[0].(map[string]any)["env"].([]any) {
				if e.(map[string]any)["name"] == "CAPACITYROOM_BOOTSTRAP_MODE" {
					t.Errorf("%s sets the bootstrap mode", name)
				}
			}
		}
	}
	if out := renderError(t, append(required, "--set", "admin.username=admin")...); !strings.Contains(out, "existingSecret") {
		t.Errorf("a missing existingSecret is refused without naming it:\n%s", out)
	}
	if out := renderError(t, append(required, "--set", "admin.username=admin", "--set", "admin.existingSecret=s", "--set", "admin.passwordKey=a/b")...); !strings.Contains(out, "passwordKey") {
		t.Errorf("a passwordKey with a slash is refused without naming it:\n%s", out)
	}
	if out := renderError(t, append(required, "--set", "admin.username=admin", "--set", "admin.existingSecret=s", "--set", "admin.language=fr")...); !strings.Contains(out, "language") {
		t.Errorf("language fr is refused without naming it:\n%s", out)
	}
}

func equalLists(got, want []any) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// TestProfilesRoundTrip renders testdata/profiles.yaml, whose display name
// holds quotes, a backslash, accents, <, and &, and reads the rendered file
// with the import's own parser.
func TestProfilesRoundTrip(t *testing.T) {
	objects := render(t, "capacity-room", append(required, "-f", "testdata/profiles.yaml")...)
	job := find(t, objects, "Job", "capacity-room-profiles")
	annotations := job["metadata"].(map[string]any)["annotations"].(map[string]any)
	if annotations["helm.sh/hook"] != "post-install,post-upgrade" {
		t.Fatalf("annotations %v", annotations)
	}
	data := find(t, objects, "ConfigMap", "capacity-room")["data"].(map[string]any)
	path := filepath.Join(t.TempDir(), "profiles.toml")
	if err := os.WriteFile(path, []byte(data["profiles.toml"].(string)), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded := loadSettings(t, objects)
	entries, err := profiles.ReadImportFile(path, netpolicy.Policy{
		Hostnames: loaded.AllowedEndpointHostnames, Networks: loaded.AllowedEndpointCIDRs,
	})
	if err != nil {
		t.Fatalf("ReadImportFile: %v\n%s", err, data["profiles.toml"])
	}
	if len(entries) != 2 || entries[0].ID != "prod" || entries[1].ID != "staging" {
		t.Fatalf("entries %+v", entries)
	}
	const name = `Prod "east" \ ação <b> & co`
	if entries[0].Document.DisplayName != name {
		t.Fatalf("display name %q, want %q", entries[0].Document.DisplayName, name)
	}
	if entries[1].Document.Thresholds.WarningPercent != 70 {
		t.Fatalf("thresholds %+v", entries[1].Document.Thresholds)
	}
}

func TestKubeconformJobs(t *testing.T) {
	kubeconform(t, renderRaw(t, append(required, "-f", "testdata/profiles.yaml",
		"--set", "admin.username=admin", "--set", "admin.existingSecret=admin-secret")...))
	kubeconform(t, renderRaw(t, "-f", "../../deploy/helm/capacity-room/examples/gateway.yaml"))
}
